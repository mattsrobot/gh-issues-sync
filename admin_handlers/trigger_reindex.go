package admin_handlers

import (
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/macwilko/issues-sync/internal_handlers/helpers"
	"github.com/macwilko/issues-sync/tasks"
)

func TriggerReindex(c *fiber.Ctx, queue *asynq.Client, db *sqlx.DB) error {

	slog.Info("💡 Starting - trigger reindex")

	escapedOwner := helpers.Truncate(strings.ToLower(c.Params("owner")), 255)
	escapedName := helpers.Truncate(strings.ToLower(c.Params("name")), 255)

	owner, err := url.QueryUnescape(escapedOwner)

	if err != nil {

		slog.Warn("❌ Unable to unescape query parameter",
			slog.String("escaped_owner", escapedOwner),
			slog.String("error", err.Error()),
		)

		return c.Status(fiber.StatusNotFound).JSON(&fiber.Map{
			"message": "not found",
		})
	}

	name, err := url.QueryUnescape(escapedName)

	if err != nil {

		slog.Warn("❌ Unable to unescape query parameter",
			slog.String("escaped_name", escapedName),
			slog.String("error", err.Error()),
		)

		return c.Status(fiber.StatusNotFound).JSON(&fiber.Map{
			"message": "not found",
		})
	}

	task, err := tasks.NewReindexSearchDatabase(owner, name)

	if err != nil {
		slog.Info("💀 Could not enqueue search reindex",
			slog.String("error", err.Error()))

		return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{
			"message": "an internal error happened",
		})
	}

	info, err := queue.Enqueue(task, asynq.Unique(time.Hour))

	if err != nil {
		switch {
		case errors.Is(err, asynq.ErrDuplicateTask):
			slog.Info("💀 Duplicate task search reindex",
				slog.String("error", err.Error()))
		default:
			slog.Info("💀 Could not enqueue search reindex",
				slog.String("error", err.Error()))
		}

		return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{
			"message": "an internal error happened",
		})
	}

	slog.Info("✅ Finished - trigger reindex",
		slog.String("task ID", info.ID),
		slog.String("queue", info.Queue))

	return c.
		Status(fiber.StatusOK).
		JSON(&fiber.Map{"message": "queued"})
}
