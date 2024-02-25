package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/imroc/req/v3"
	"github.com/jmoiron/sqlx"
	"github.com/macwilko/issues-sync/db/models"
	"github.com/macwilko/issues-sync/ws_handlers"
	"github.com/meilisearch/meilisearch-go"
)

const (
	GithubProcessIssueUpdate = "github:issue-update"
)

type GithubProcessIssueUpdatePayload struct {
	WebHookPayload []byte
}

type GitHubWebhookPayload struct {
	Action string              `json:"action"`
	Issue  *GitHubWebhookIssue `json:"issue"`
	Repo   *GitHubWebhookRepo  `json:"repository"`
}

type GitHubWebhookIssue struct {
	ID        uint64                 `json:"id"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt *string                `json:"updated_at"`
	Title     string                 `json:"title"`
	Number    uint64                 `json:"number"`
	Comments  uint64                 `json:"comments"`
	State     string                 `json:"state"`
	User      map[string]interface{} `json:"user"`
	Labels    []interface{}          `json:"labels"`
	Assignees []interface{}          `json:"assignees"`
}

type GitHubWebhookRepo struct {
	Name  string                 `json:"name"`
	Owner GitHubWebhookRepoOwner `json:"owner"`
}

type GitHubWebhookRepoOwner struct {
	Login string `json:"login"`
}

func NewGithubProcessIssueUpdate(WebHookPayload []byte) (*asynq.Task, error) {
	payload, err := json.Marshal(GithubProcessIssueUpdatePayload{
		WebHookPayload: WebHookPayload,
	})

	if err != nil {
		slog.Error("Unable to schedule issue update on queue",
			slog.String("error", err.Error()))

		return nil, err
	}

	return asynq.NewTask(GithubProcessIssueUpdate, payload, asynq.MaxRetry(5)), nil
}

func HandleGithubProcessIssueUpdate(ctx context.Context, t *asynq.Task, db *sqlx.DB, meili *meilisearch.Client, queue *asynq.Client) error {
	slog.Info("🏃 Starting processing github issue update")

	var payload GithubProcessIssueUpdatePayload

	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		slog.Error("❌ Could not process github payload",
			slog.String("error", err.Error()))

		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	var webhook GitHubWebhookPayload

	if err := json.Unmarshal(payload.WebHookPayload, &webhook); err != nil {
		slog.Error("❌ Could not process github webhook",
			slog.String("error", err.Error()))

		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	if webhook.Issue == nil || webhook.Repo == nil {
		slog.Info("❌ Aborting, not a valid webhook event",
			slog.Any("info", webhook))

		return fmt.Errorf("not a valid webhook event: %w", asynq.SkipRetry)
	}

	slog.Info("💡 Starting processing of webhook info",
		slog.String("name", webhook.Repo.Name),
		slog.String("owner", webhook.Repo.Owner.Login))

	tx, err := db.BeginTxx(ctx, &sql.TxOptions{
		ReadOnly: false,
	})

	if err != nil {
		slog.Error("❌ Couldn't get tx, db error, will retry 💀",
			slog.String("name", webhook.Repo.Name),
			slog.String("owner", webhook.Repo.Owner.Login),
			slog.String("error", err.Error()))

		return err
	}

	createdAt, err := time.Parse(time.RFC3339, webhook.Issue.CreatedAt)

	if err != nil {
		tx.Rollback()

		slog.Error("❌ Couldn't parse created_at, will retry 💀",
			slog.String("name", webhook.Repo.Name),
			slog.String("owner", webhook.Repo.Owner.Login),
			slog.String("error", err.Error()))

		return err
	}

	var updatedAt time.Time

	if webhook.Issue.UpdatedAt != nil {
		updatedAt, err = time.Parse(time.RFC3339, *webhook.Issue.UpdatedAt)

		if err != nil {
			tx.Rollback()

			slog.Error("❌ Couldn't parse updated_at, will retry 💀",
				slog.String("name", webhook.Repo.Name),
				slog.String("owner", webhook.Repo.Owner.Login),
				slog.String("error", err.Error()))

			return err
		}
	} else {
		updatedAt = createdAt
	}

	author, err := json.Marshal(webhook.Issue.User)

	if err != nil {
		tx.Rollback()

		slog.Error("❌ Couldn't marshal author, will retry 💀",
			slog.String("name", webhook.Repo.Name),
			slog.String("owner", webhook.Repo.Owner.Login),
			slog.String("error", err.Error()))

		return err
	}

	labels, err := json.Marshal(webhook.Issue.Labels)

	if err != nil {
		tx.Rollback()

		slog.Error("❌ Couldn't marshal labels, will retry 💀",
			slog.String("name", webhook.Repo.Name),
			slog.String("owner", webhook.Repo.Owner.Login),
			slog.String("error", err.Error()))

		return err
	}

	assignees, err := json.Marshal(webhook.Issue.Assignees)

	if err != nil {
		tx.Rollback()

		slog.Error("❌ Couldn't marshal assignees, will retry 💀",
			slog.String("name", webhook.Repo.Name),
			slog.String("owner", webhook.Repo.Owner.Login),
			slog.String("error", err.Error()))

		return err
	}

	issue := models.Issues{}

	selectIssue := `
	SELECT * FROM issues
	WHERE github_id=$1
	LIMIT 1
	`

	err = tx.Get(&issue, selectIssue, webhook.Issue.ID)

	if err == sql.ErrNoRows {
		insertIntoIssues := `
		INSERT INTO issues
			(id, created_at, updated_at, title, issue_number, comments_count, repo_name, repo_owner, author, labels, assignees, closed, github_id)
		VALUES
			(nextval('issues_id_seq'::regclass), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`

		_, err = tx.Exec(
			insertIntoIssues,
			createdAt,
			updatedAt,
			webhook.Issue.Title,
			webhook.Issue.Number,
			webhook.Issue.Comments,
			webhook.Repo.Name,
			webhook.Repo.Owner.Login,
			author,
			labels,
			assignees,
			webhook.Issue.State == "closed",
			webhook.Issue.ID,
		)

		if err != nil {
			tx.Rollback()

			slog.Error("❌ Couldn't insert issue, will retry 💀",
				slog.String("name", webhook.Repo.Name),
				slog.String("owner", webhook.Repo.Owner.Login),
				slog.String("error", err.Error()))

			return err
		}
	} else if err != nil {
		tx.Rollback()

		slog.Error("❌ Database issue fetching issues, will retry 💀",
			slog.String("name", webhook.Repo.Name),
			slog.String("owner", webhook.Repo.Owner.Login),
			slog.String("error", err.Error()))

		return err
	} else {
		updateIssue := `
		UPDATE issues
		SET updated_at=$1, title=$2, issue_number=$3, comments_count=$4, repo_name=$5, repo_owner=$6, author=$7, labels=$8, assignees=$9, closed=$10
		WHERE id=$11
		`
		_, err = tx.
			Exec(
				updateIssue,
				updatedAt,
				webhook.Issue.Title,
				webhook.Issue.Number,
				webhook.Issue.Comments,
				webhook.Repo.Name,
				webhook.Repo.Owner.Login,
				author,
				labels,
				assignees,
				webhook.Issue.State == "closed",
				issue.ID,
			)

		if err != nil {
			tx.Rollback()

			slog.Error("❌ Couldn't update issue with new info, will retry 💀",
				slog.String("name", webhook.Repo.Name),
				slog.String("owner", webhook.Repo.Owner.Login),
				slog.String("error", err.Error()))

			return err
		}
	}

	err = tx.Commit()

	if err != nil {
		slog.Error("❌ Couldn't add message, commit db error, will retry 💀",
			slog.String("name", webhook.Repo.Name),
			slog.String("owner", webhook.Repo.Owner.Login),
			slog.String("error", err.Error()))

		return err
	}

	marshalled, err := json.Marshal(fiber.Map{
		"updated_at": time.Now().Format(time.RFC3339),
	})

	if err != nil {
		slog.Error("💀 Couldn't marshal message",
			slog.String("error", err.Error()))

		return nil
	}

	client := req.C()

	client.R().
		SetContentType("application/json").
		SetBody(&ws_handlers.BroadcastMessageInput{
			Topic:   fmt.Sprintf("repo-%s-%s", webhook.Repo.Name, webhook.Repo.Owner.Login),
			Message: string(marshalled),
		}).
		Post(os.Getenv("WS_API_PRIVATE_URL") + "/broadcast-message")

	slog.Info("✅ Completed processing github issue",
		slog.String("name", webhook.Repo.Name),
		slog.String("owner", webhook.Repo.Owner.Login))

	return nil
}
