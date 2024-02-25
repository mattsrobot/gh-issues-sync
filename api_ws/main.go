package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"log/slog"
	"os"

	_ "github.com/lib/pq"
	chatserver "github.com/macwilko/issues-sync/chatserver"
	"github.com/macwilko/issues-sync/ws_handlers"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/idempotency"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

func runChatServer(server *chatserver.Server) {
	slog.Info("🚀 Accepting ws connections ✅")

	for {
		select {

		// Subscribe a user to a topic
		case message := <-server.Subscribe:
			if c, ok := server.Clients[message.Connection]; ok {
				go func() {
					c.Mu.Lock()
					defer c.Mu.Unlock()
					c.Topics[message.Topic] = true
					slog.Info("🚀 Subscribed to topic", slog.String("topic", message.Topic))
				}()
			}

		// Unsubscribe a user to a topic
		case message := <-server.Unsubscribe:
			if c, ok := server.Clients[message.Connection]; ok {
				go func() {
					c.Mu.Lock()
					defer c.Mu.Unlock()
					delete(c.Topics, message.Topic)
					slog.Info("🚀 Unsubscribed from topic", slog.String("topic", message.Topic))
				}()
			}

		// Register a user
		case connection := <-server.Register:
			c := chatserver.Client{
				Lp:        time.Now(),
				IsClosing: false,
				Mu:        sync.Mutex{},
				Topics:    make(map[string]bool),
			}
			server.Clients[connection] = &c
			slog.Info("😍 Client connected")

			connection.SetPingHandler(func(msg string) error {
				c.Mu.Lock()
				defer c.Mu.Unlock()
				slog.Info("🔥 Got a ping 🔥")
				c.Lp = time.Now()
				return nil
			})

		case message := <-server.Echo:
			if c, ok := server.Clients[message.Connection]; ok {
				go func() {

					c.Mu.Lock()
					defer c.Mu.Unlock()

					if c.IsClosing {
						slog.Warn("💀 Client is closing")

						return
					}

					connection := message.Connection

					if err := connection.WriteMessage(websocket.TextMessage, []byte(message.Message)); err != nil {
						c.IsClosing = true

						slog.Error("💀 Couldn't write message", slog.String("error", err.Error()))

						// Try close the connection
						connection.WriteMessage(websocket.CloseMessage, []byte{})
						connection.Close()

						// Unregister connection
						server.Unregister <- connection
					}
				}()
			}

		// Broadcast to a topic
		case broadcast := <-server.Broadcast:

			// Send the message to all clients
			for connection, c := range server.Clients {

				// send to each client in parallel so we don't block on a slow client
				go func(connection *websocket.Conn, c *chatserver.Client) {
					c.Mu.Lock()
					defer c.Mu.Unlock()

					if c.IsClosing {
						slog.Warn("💀 Client is closing")

						return
					}

					if _, ok := c.Topics[broadcast.Topic]; ok {

						marshalled, err := json.Marshal(broadcast)

						if err != nil {
							slog.Error("💀 Couldn't marshal message",
								slog.String("error", err.Error()))

							return
						}

						// Write the message to the client
						if err := connection.WriteMessage(websocket.TextMessage, marshalled); err != nil {
							c.IsClosing = true

							slog.Error("💀 Couldn't write message", slog.String("error", err.Error()))

							// Try close the connection
							connection.WriteMessage(websocket.CloseMessage, []byte{})
							connection.Close()

							// Unregister connection
							server.Unregister <- connection
						}
					}

				}(connection, c)
			}

		case connection := <-server.Unregister:
			// Remove the client from the hub
			delete(server.Clients, connection)
			slog.Info("connection unregistered")
		}
	}
}

func main() {
	server := &chatserver.Server{
		Clients:     make(map[*websocket.Conn]*chatserver.Client), // Map of connections to clients
		Subscribe:   make(chan chatserver.Message),                // Subscribe to a topic
		Unsubscribe: make(chan chatserver.Message),                // Unsubscribe to a topic
		Echo:        make(chan chatserver.Echo),                   // Echo a message to a client
		Broadcast:   make(chan chatserver.Broadcast),              // Broadcast a message to a topic
		Register:    make(chan *websocket.Conn),                   // Register a new connection
		Unregister:  make(chan *websocket.Conn),                   // Unregister a connection
	}

	lg := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(lg)

	slog.Info("🥱 Preparing private networking")

	time.Sleep(2 * time.Second)

	slog.Info("🚀 Starting Distangled ws api ✅")

	ctx := context.Background()

	godotenv.Load("../.env")

	slog.Info("🚀 Connecting to Postgres ✅")

	db, err := sqlx.Connect("postgres", os.Getenv("DATABASE_PRIVATE_URL"))

	if err != nil {
		slog.Error("Unable to connect to db",
			slog.String("error", err.Error()))

		panic(err)
	}

	defer db.Close()

	slog.Info("🚀 Connecting to Redis ✅")

	redisOpts, err := redis.ParseURL(os.Getenv("REDIS_PRIVATE_URL"))

	if err != nil {
		slog.Error("Unable to read redis database",
			slog.String("error", err.Error()))

		panic(err)
	}

	slog.Info("🚀 Booting to async queue ✅")

	queue := asynq.NewClient(asynq.RedisClientOpt{
		Network:  redisOpts.Network,
		Addr:     redisOpts.Addr,
		Username: redisOpts.Username,
		Password: redisOpts.Password,
		DB:       redisOpts.DB,
	})

	defer queue.Close()

	slog.Info("🚀 Starting ws web server ✅")

	// Start the Chat Server

	app := fiber.New(fiber.Config{
		Network: "tcp",
	})

	app.Use(recover.New(recover.Config{EnableStackTrace: true}))
	app.Use(logger.New())
	app.Use(idempotency.New())
	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		DisableColors: false,
		Format:        "${pid} ${locals:requestid} ${status} - ${method} ${path}\u200b",
	}))
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))
	app.Use(cors.New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(fmt.Sprintf("So exotic! %s", os.Getenv("RAILWAY_REPLICA_ID")))
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("I'm healthy!")
	})

	app.Get("/metrics", monitor.New(monitor.Config{Title: "Metrics"}))

	app.Use("/ws", ws_handlers.AuthorizationWS)

	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}

		return c.SendStatus(fiber.StatusUpgradeRequired)
	})

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {

		defer func() {
			server.Unregister <- c
			c.Close()
		}()

		server.Register <- c // Register the client

		for {
			messageType, message, err := c.ReadMessage()

			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					slog.Error("Unexpected read error on connection",
						"error", err.Error())
				}

				return // Calls the deferred unregister function
			}

			if messageType == websocket.TextMessage {

				m := string(message)

				if m == "ping" {
					// Echo the message back
					server.Echo <- chatserver.Echo{
						Message:    m,
						Connection: c,
					}
				} else {

					var data map[string]string

					err := json.Unmarshal(message, &data)

					if err != nil {
						slog.Error("Not valid json, unregister client")

						return // Calls the deferred unregister function
					}

					messageType, ok := data["type"]

					if !ok {
						slog.Error("Not valid message type, unregister client")

						return // Calls the deferred unregister function
					}

					switch messageType {
					case "subscribe":
						topic, ok := data["topic"]

						if !ok {
							slog.Error("Not valid topic, unregister client")

							return // Calls the deferred unregister function
						}

						server.Subscribe <- chatserver.Message{
							Topic:      topic,
							Connection: c,
						}
					case "unsubscribe":
						topic, ok := data["topic"]

						if !ok {
							slog.Error("Not valid topic, unregister client")

							return // Calls the deferred unregister function
						}

						server.Unsubscribe <- chatserver.Message{
							Topic:      topic,
							Connection: c,
						}
					default:
						return // Calls the deferred unregister function
					}
				}
			}
		}
	}, websocket.Config{
		RecoverHandler: func(conn *websocket.Conn) {
			slog.Error("💀 ws had an unrecoverable error 💀")

			conn.WriteJSON(fiber.Map{"error": "an error occurred"})
		},
	},
	))

	v1 := fiber.New()

	app.Mount("/v1", v1)

	v1.Use(func(c *fiber.Ctx) error {
		c.Accepts("application/json")
		return c.Next()
	})

	internal := fiber.New()

	v1.Mount("/internal", internal)

	internal.Post("/broadcast-message", func(c *fiber.Ctx) error {
		return ws_handlers.BroadcastMessage(c, ctx, db, queue, server)
	})

	port := ":5001"

	if envPort := os.Getenv("PORT"); envPort != "" {
		port = ":" + envPort
	}

	go runChatServer(server)

	app.Listen(port)

	slog.Info("❌ Graceful shutdown ❌")
}
