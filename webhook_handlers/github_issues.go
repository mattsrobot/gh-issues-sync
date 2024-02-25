package webhook_handlers

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/macwilko/issues-sync/tasks"
)

func GithubIssues(c *fiber.Ctx, queue *asynq.Client) error {

	slog.Info("🏃 Starting a github webhook issues request")

	c.Accepts("application/json")

	task, err := tasks.NewGithubProcessIssueUpdate(
		c.Body(),
	)

	if err != nil {
		slog.Error("💀 Could not enqueue github issue",
			slog.String("error", err.Error()))

		return c.
			Status(fiber.StatusOK).
			JSON(&fiber.Map{"message": "unexpected error"})
	}

	info, err := queue.Enqueue(task, asynq.Unique(time.Hour), asynq.Queue("critical"))

	if err != nil {
		switch {
		case errors.Is(err, asynq.ErrDuplicateTask):
			slog.Info("💀 Duplicate task process github issue",
				slog.String("error", err.Error()))
		default:
			slog.Error("💀 Could not enqueue process github issue",
				slog.String("error", err.Error()))
		}

		return c.
			Status(fiber.StatusOK).
			JSON(&fiber.Map{"message": "unexpected error"})
	}

	slog.Info("✅ Issue is scheduled for processing",
		slog.String("task-id", info.ID),
		slog.String("queue", info.Queue))

	return c.
		Status(fiber.StatusOK).
		JSON(&fiber.Map{"message": "ok"})
}
