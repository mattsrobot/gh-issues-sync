package ws_handlers

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
)

func AuthorizationWS(c *fiber.Ctx) error {
	slog.Info("Authorized new ws connection")
	return c.Next()
}
