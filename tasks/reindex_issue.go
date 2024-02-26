package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/macwilko/issues-sync/db/models"
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
	slog.Info("🏃 Starting reindexing issue ")

	var p ReindexIssuePayload

	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		slog.Error("Could not reindex issue",
			slog.String("error", err.Error()))

		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	issue := models.Issues{}

	err := db.Get(&issue, "SELECT * FROM issues WHERE id=$1", p.IssueID)

	if err != nil && err != sql.ErrNoRows {
		slog.Error("💀 An internal error happened",
			slog.Uint64("issue_id", p.IssueID),
			slog.String("error", err.Error()),
		)

		return err
	}

	json, err := issue.ToMap()

	if err != nil && err != sql.ErrNoRows {
		slog.Error("💀 An internal error happened",
			slog.Uint64("issue_id", p.IssueID),
			slog.String("error", err.Error()),
		)

		return err
	}

	index := meili.Index("issues-" + issue.RepoOwner + "-" + issue.RepoName)

	_, err = index.UpdateFilterableAttributes(&[]string{"repo_owner", "repo_name", "closed"})

	if err != nil {
		slog.Error("💀 Couldnt update filterable attributed",
			slog.Uint64("issue_id", issue.ID),
			slog.String("error", err.Error()),
		)

		return err
	}

	documents := []map[string]interface{}{}

	documents = append(documents, *json)

	taskInfo, err := index.UpdateDocuments(documents, "id")

	if err != nil {
		slog.Error("💀 Couldn't trigger meilisearch index",
			slog.Uint64("issue_id", issue.ID),
			slog.String("error", err.Error()),
		)

		return err
	}

	task, err := meili.WaitForTask(taskInfo.TaskUID)

	if err != nil {
		slog.Error("💀 Couldn't trigger meilisearch index",
			slog.Uint64("issue_id", issue.ID),
			slog.String("error", err.Error()),
		)

		return err
	}

	slog.Info("Completed",
		slog.String("duration", task.Duration))

	slog.Info("Completed reindexing issue ✅")

	return nil
}
