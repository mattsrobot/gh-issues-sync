package main

import (
	"context"
	"fmt"
	"time"

	"log/slog"
	"os"

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
	_ "github.com/lib/pq"
	"github.com/macwilko/issues-sync/admin_handlers"
	"github.com/macwilko/issues-sync/internal_handlers"
	"github.com/meilisearch/meilisearch-go"
	"github.com/redis/go-redis/v9"
	"github.com/valyala/fasthttp"

	"github.com/jmoiron/sqlx"
)

func main() {
	lg := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(lg)

	slog.Info("🥱 Preparing private networking")

	time.Sleep(2 * time.Second)

	slog.Info("🚀 Starting issues-sync REST api ✅")

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

	slog.Info("🚀 Connecting to Meilisearch ✅")

	fasthttpClient := &fasthttp.Client{
		Name:             "meilisearch-client",
		DialDualStack:    true,
		ConnPoolStrategy: fasthttp.LIFO,
	}

	meili := meilisearch.NewFastHTTPCustomClient(meilisearch.ClientConfig{
		Host:   os.Getenv("MEILI_PRIVATE_URL"),
		APIKey: os.Getenv("MEILI_API_KEY"),
	}, fasthttpClient)

	healthy := meili.IsHealthy()

	if !healthy {
		slog.Error("Unable to connect to meili")

		os.Exit(1)
	}

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

	slog.Info("🚀 Starting web server ✅")

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

	app.Get("/metrics", monitor.New(monitor.Config{
		Title: "Metrics",
	}))

	v1 := fiber.New()

	app.Mount("/v1", v1)

	v1.Use(func(c *fiber.Ctx) error {
		c.Accepts("application/json")
		return c.Next()
	})

	webhooks := fiber.New()

	v1.Mount("/webhooks", webhooks)

	webhooks.Get("/github/:owner/:name", func(c *fiber.Ctx) error {
		return internal_handlers.Issues(c, ctx, db, meili)
	})

	internal := fiber.New()

	v1.Mount("/internal", internal)

	internal.Get("/repo/:owner/:name/issues", func(c *fiber.Ctx) error {
		return internal_handlers.Issues(c, ctx, db, meili)
	})

	admin := fiber.New()

	v1.Mount("/admin", admin)

	admin.Post("/repo/:owner/:name/reindex", func(c *fiber.Ctx) error {
		return admin_handlers.TriggerReindex(c, queue, db)
	})

	port := ":5000"

	if envPort := os.Getenv("PORT"); envPort != "" {
		port = ":" + envPort
	}

	app.Listen(port)

	slog.Info("❌ Graceful shutdown ❌")
}
