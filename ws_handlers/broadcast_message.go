package ws_handlers

import (
	"context"
	"log/slog"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	chatserver "github.com/macwilko/issues-sync/chatserver"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
)

type BroadcastMessageInput struct {
	Message string `json:"message"`
	Topic   string `json:"topic"`
}

func BroadcastMessage(c *fiber.Ctx, ctx context.Context, db *sqlx.DB, queue *asynq.Client, server *chatserver.Server) error {
	slog.Info("⚡️ Broadcasting message")

	input := new(BroadcastMessageInput)

	if err := c.BodyParser(input); err != nil {
		slog.Warn("Invalid input 💀")

		return c.Status(fiber.StatusOK).JSON(&fiber.Map{
			"error": "Invalid input.",
		})
	}

	validate := validator.New()
	en := en.New()
	uni := ut.New(en, en)
	trans, _ := uni.GetTranslator("en")
	en_translations.RegisterDefaultTranslations(validate, trans)
	err := validate.Struct(input)

	var errors []fiber.Map

	if err != nil {
		slog.Error("💀 Unable to broadcast message, input 💀",
			slog.String("error", err.Error()))

		errs := err.(validator.ValidationErrors)

		for _, v := range errs {
			errors = append(errors, fiber.Map{
				"field":   v.Field(),
				"message": v.Translate(trans),
			})
		}
	}

	if len(errors) > 0 {
		slog.Error("💀 Unable to broadcast message, input error 💀")

		return c.Status(fiber.StatusOK).JSON(&fiber.Map{
			"errors": errors,
		})
	}

	server.Broadcast <- chatserver.Broadcast{
		Message: input.Message,
		Topic:   input.Topic,
	}

	slog.Info("Broadcasted message ✅")

	return c.Status(fiber.StatusOK).JSON(&fiber.Map{
		"ok": true,
	})
}
