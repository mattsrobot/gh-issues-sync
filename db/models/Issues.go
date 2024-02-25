package models

import (
	"database/sql"
	"maps"
	"time"

	types "github.com/jmoiron/sqlx/types"

	"github.com/gofiber/fiber/v2"
)

type Issues struct {
	ID            uint64         `db:"id"`             // INT8 PKEY
	CreatedAt     time.Time      `db:"created_at"`     // TIMESTAMPZ
	UpdatedAt     sql.NullTime   `db:"updated_at"`     // TIMESTAMPZ
	Title         string         `db:"title"`          // VARCHAR(2000)
	IssueNumber   uint64         `db:"issue_number"`   // BIGINT
	CommentsCount uint64         `db:"comments_count"` // BIGINT
	RepoName      string         `db:"repo_name"`      // VARCHAR(255) idx
	RepoOwner     string         `db:"repo_owner"`     // VARCHAR(255) idx
	Author        types.JSONText `db:"author"`         // JSONB
	Labels        types.JSONText `db:"labels"`         // JSONB
	Assignees     types.JSONText `db:"assignees"`      // JSONB
}

func (c Issues) ToMap() (*fiber.Map, error) {
	var author fiber.Map
	err := c.Author.Unmarshal(&author)

	if err != nil {
		return nil, err
	}

	var labels []fiber.Map
	err = c.Labels.Unmarshal(&labels)

	if err != nil {
		return nil, err
	}

	var assignees []fiber.Map
	err = c.Assignees.Unmarshal(&assignees)

	if err != nil {
		return nil, err
	}

	json := fiber.Map{
		"id":             c.ID,
		"created_at":     c.CreatedAt.Format(time.RFC3339),
		"title":          c.Title,
		"issue_number":   c.IssueNumber,
		"comments_count": c.CommentsCount,
		"repo_name":      c.RepoName,
		"repo_owner":     c.RepoOwner,
		"author":         author,
		"labels":         labels,
		"assignees":      assignees,
	}

	if c.UpdatedAt.Valid {
		maps.Copy(json, fiber.Map{
			"updated_at": c.UpdatedAt.Time.Format(time.RFC3339),
		})
	} else {
		maps.Copy(json, fiber.Map{
			"updated_at": c.CreatedAt.Format(time.RFC3339),
		})
	}

	return &json, nil
}
