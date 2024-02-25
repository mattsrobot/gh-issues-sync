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
	ReindexIssue = "search:index-issue"
)

type ReindexIssuePayload struct {
	IssueID uint64
}

func NewReindexIssue(IssueID uint64) (*asynq.Task, error) {
	payload, err := json.Marshal(ReindexIssuePayload{
		IssueID: IssueID,
	})

	slog.Info("Scheduling reindex issue")

	if err != nil {
		slog.Error("Unable to schedule reindex issue",
			slog.String("error", err.Error()))

		return nil, err
	}

	return asynq.NewTask(ReindexIssue, payload), nil
}

func HandleReindexIssue(ctx context.Context, t *asynq.Task, db *sqlx.DB, meili *meilisearch.Client) error {
	slog.Info("Starting reindexing issue ✅")

	slog.Info("Completed reindexing issue 🚀")

	return nil
}
