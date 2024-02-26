package tasks

import (
	"context"
	"encoding/json"
	"fmt"
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

	var p ReindexSearchDatabasePayload

	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		slog.Error("Could not reindex search database",
			slog.String("error", err.Error()))

		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	deleteIndex, err := meili.DeleteIndex("issues-" + p.RepoOwner + "-" + p.RepoName)

	if err != nil {
		slog.Error("💀 Couldn't trigger meilisearch index, error deleting index",
			slog.String("repo_owner", p.RepoOwner),
			slog.String("repo_name", p.RepoName),
			slog.String("error", err.Error()),
		)

		return err
	}

	_, err = meili.WaitForTask(deleteIndex.TaskUID)

	if err != nil {
		slog.Error("💀 Couldn't trigger meilisearch index, error deleting index",
			slog.String("repo_owner", p.RepoOwner),
			slog.String("repo_name", p.RepoName),
			slog.String("error", err.Error()),
		)

		return err
	}

	createIndex, err := meili.CreateIndex(&meilisearch.IndexConfig{
		Uid:        "issues-" + p.RepoOwner + "-" + p.RepoName,
		PrimaryKey: "id",
	})

	if err != nil {
		slog.Error("💀 Couldn't trigger meilisearch index, error creating index",
			slog.String("repo_owner", p.RepoOwner),
			slog.String("repo_name", p.RepoName),
			slog.String("error", err.Error()),
		)

		return err
	}

	_, err = meili.WaitForTask(createIndex.TaskUID)

	if err != nil {
		slog.Error("💀 Couldn't trigger meilisearch index, error creating index",
			slog.String("repo_owner", p.RepoOwner),
			slog.String("repo_name", p.RepoName),
			slog.String("error", err.Error()),
		)

		return err
	}

	index := meili.Index("issues-" + p.RepoOwner + "-" + p.RepoName)

	_, err = index.UpdateFilterableAttributes(&[]string{"repo_owner", "repo_name", "closed"})

	if err != nil {
		slog.Error("💀 Couldnt update filterable attributed",
			slog.String("repo_owner", p.RepoOwner),
			slog.String("repo_name", p.RepoName),
			slog.String("error", err.Error()),
		)

		return err
	}

	slog.Info("Completed reindexing search database 🚀")

	return nil
}
