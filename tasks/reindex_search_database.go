package tasks

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/meilisearch/meilisearch-go"
)

const (
	ReindexSearchDatabase = "search:reindex"
)

type ReindexSearchDatabasePayload struct {
	RepoName  string
	RepoOwner string
}

func NewReindexSearchDatabase(RepoOwner string, RepoName string) (*asynq.Task, error) {
	payload, err := json.Marshal(ReindexSearchDatabasePayload{
		RepoName:  RepoName,
		RepoOwner: RepoOwner,
	})

	slog.Info("Scheduling reindex of repo")

	if err != nil {
		slog.Error("Unable to schedule reindex of repo",
			slog.String("error", err.Error()))

		return nil, err
	}

	return asynq.NewTask(ReindexSearchDatabase, payload), nil
}

func HandleReindexSearchDatabase(ctx context.Context, t *asynq.Task, db *sqlx.DB, meili *meilisearch.Client) error {
	slog.Info("Starting reindexing search database ✅")

	slog.Info("Completed reindexing search database 🚀")

	return nil
}
