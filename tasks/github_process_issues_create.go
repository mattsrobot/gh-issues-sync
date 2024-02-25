package tasks

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/meilisearch/meilisearch-go"
)

const (
	GithubProcessIssueCreate = "github:issue-create"
)

type GithubProcessIssuesCreatePayload struct {
}

func HandleGithubProcessIssueCreate(ctx context.Context, t *asynq.Task, db *sqlx.DB, meili *meilisearch.Client, queue *asynq.Client) error {
	slog.Info("🏃 Starting processing github issue create")

	slog.Info("✅ Broadcasted message event")

	return nil
}
