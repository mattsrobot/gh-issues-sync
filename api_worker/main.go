package main

import (
	"context"
	"time"

	"log/slog"
	"os"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/macwilko/issues-sync/tasks"
	"github.com/meilisearch/meilisearch-go"
	"github.com/redis/go-redis/v9"
	"github.com/valyala/fasthttp"
)

func main() {
	lg := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(lg)

	slog.Info("🥱 Preparing private networking")

	time.Sleep(2 * time.Second)

	slog.Info("🚀 Starting issue-sync WORKER ✅")

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

	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Network:  redisOpts.Network,
			Addr:     redisOpts.Addr,
			Username: redisOpts.Username,
			Password: redisOpts.Password,
			DB:       redisOpts.DB,
		},
		asynq.Config{
			Concurrency: 20,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		},
	)

	queue := asynq.NewClient(asynq.RedisClientOpt{
		Network:  redisOpts.Network,
		Addr:     redisOpts.Addr,
		Username: redisOpts.Username,
		Password: redisOpts.Password,
		DB:       redisOpts.DB,
	})

	defer queue.Close()

	mux := asynq.NewServeMux()

	mux.HandleFunc(tasks.ReindexSearchDatabase, func(ctx context.Context, t *asynq.Task) error {
		return tasks.HandleReindexSearchDatabase(ctx, t, db, meili)
	})

	mux.HandleFunc(tasks.GithubProcessIssueUpdate, func(ctx context.Context, t *asynq.Task) error {
		return tasks.HandleGithubProcessIssueUpdate(ctx, t, db, meili, queue)
	})

	mux.HandleFunc(tasks.ReindexIssue, func(ctx context.Context, t *asynq.Task) error {
		return tasks.HandleReindexIssue(ctx, t, db, meili)
	})

	if err := srv.Run(mux); err != nil {
		slog.Error("Scheduler crashed",
			slog.String("error", err.Error()))
	}
}
