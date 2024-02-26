package internal_handlers

import (
	"context"
	"database/sql"
	"net/url"
	"strings"

	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/macwilko/issues-sync/db/models"
	helpers "github.com/macwilko/issues-sync/internal_handlers/helpers"
	"github.com/meilisearch/meilisearch-go"
)

func Issues(c *fiber.Ctx, ctx context.Context, db *sqlx.DB, meili *meilisearch.Client) error {

	q := helpers.Truncate(c.Query("q"), 100)
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

	state := c.Query("state")

	slog.Info("💡 Starting - fetch issues",
		slog.String("owner", owner),
		slog.String("name", name))

	issues := []models.Issues{}
	var issuesJson []interface{}

	if len(q) >= 1 {

		meiliFilter := ""

		switch state {
		case "open":
			meiliFilter = "closed = false AND "
		case "closed":
			meiliFilter = "closed = true AND "
		}

		meiliFilter = meiliFilter + "repo_owner = '" + owner + "' AND repo_name = '" + name + "'"

		searchResponse, err := meili.Index("issues-"+owner+"-"+name).Search(q, &meilisearch.SearchRequest{
			Limit:                 100,
			AttributesToHighlight: []string{"*"},
			Filter:                meiliFilter,
		})

		if err != nil {
			slog.Error("💀 An internal error happened",
				slog.String("owner", owner),
				slog.String("name", name),
				slog.String("error", err.Error()),
			)

			return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{
				"message": "an internal error happened",
			})
		}

		slog.Info("💡 Search results info",
			slog.String("query", q),
			slog.String("filter", meiliFilter),
			slog.Int64("estimated_hits", searchResponse.EstimatedTotalHits),
			slog.Int64("hits", searchResponse.TotalHits))

		issuesJson = searchResponse.Hits

	} else {
		err = db.Select(&issues, "SELECT * FROM issues WHERE repo_name=$1 AND repo_owner=$2 AND closed=$3 ORDER BY created_at DESC LIMIT 25", name, owner, state == "closed")

		if err != nil && err != sql.ErrNoRows {
			slog.Error("💀 An internal error happened",
				slog.String("owner", owner),
				slog.String("name", name),
				slog.String("error", err.Error()),
			)

			return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{
				"message": "an internal error happened",
			})
		}

		for _, issue := range issues {
			json, err := issue.ToMap()

			if err != nil {
				slog.Error("💀 An internal error happened",
					slog.String("owner", owner),
					slog.String("name", name),
					slog.String("error", err.Error()),
				)

				return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{
					"message": "an internal error happened",
				})
			}

			issuesJson = append(issuesJson, *json)
		}
	}

	var closedCount int

	err = db.Get(&closedCount, "SELECT count(*) FROM issues WHERE repo_name=$1 AND repo_owner=$2 AND closed=$3", name, owner, 1)

	if err != nil {
		slog.Error("💀 An internal error happened, getting closed count",
			slog.String("owner", owner),
			slog.String("name", name),
			slog.String("error", err.Error()),
		)

		return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{
			"message": "an internal error happened",
		})
	}

	var openCount int

	err = db.Get(&openCount, "SELECT count(*) FROM issues WHERE repo_name=$1 AND repo_owner=$2 AND closed=$3", name, owner, 0)

	if err != nil {
		slog.Error("💀 An internal error happened, getting open count",
			slog.String("owner", owner),
			slog.String("name", name),
			slog.String("error", err.Error()),
		)

		return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{
			"message": "an internal error happened",
		})
	}

	responseJson := fiber.Map{
		"closed_count": closedCount,
		"open_count":   openCount,
		"issues":       issuesJson,
	}

	slog.Info("✅ Finished - fetch issues",
		slog.String("owner", owner),
		slog.String("name", name))

	return c.
		Status(fiber.StatusOK).
		JSON(&responseJson)
}
