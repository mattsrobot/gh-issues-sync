package tasks

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/meilisearch/meilisearch-go"
)

const (
	GithubProcessIssueUpdate = "discord:thread-update"
)

type GithubProcessIssueUpdatePayload struct {
}

func HandleGithubProcessIssueUpdate(ctx context.Context, t *asynq.Task, db *sqlx.DB, meili *meilisearch.Client, queue *asynq.Client) error {
	slog.Info("🏃 Starting processing github issue update")

	slog.Info("✅ Broadcasted message event")

	return nil
}
