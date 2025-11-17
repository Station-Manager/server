package service

import (
	"github.com/gofiber/fiber/v2"
)

func serverErrorHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}
}
