package server

import (
	"fmt"
	"github.com/Station-Manager/apikey"
	"github.com/Station-Manager/types"
	"github.com/gofiber/fiber/v2"
)

func (s *Service) addLogbookHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Get(fiber.HeaderAuthorization)

		fmt.Println("Token:", token)

		var typeLogbook types.Logbook
		if err := c.BodyParser(&typeLogbook); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
		}

		fmt.Println(typeLogbook)

		fullKey, prefix, hash, err := apikey.Generate(10)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "key generation failed"})
		}

		fmt.Println("Store the hash: ", hash, " in the database")
		fmt.Println("Store the prefix: ", prefix)

		c.Status(fiber.StatusCreated)

		// Return the full key to the client as a one-off deal
		return c.JSON(fiber.Map{"api_key": fullKey})
	}
}
