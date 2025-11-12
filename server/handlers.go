package server

import (
	"fmt"
	"github.com/Station-Manager/apikey"
	"github.com/Station-Manager/types"
	"github.com/gofiber/fiber/v2"
)

func (s *Service) addLogbookHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		s.logger.DebugWith().Msg("Registering a new logbook")

		token := c.Get(fiber.HeaderAuthorization)

		fmt.Println("Token:", token)

		var typeLogbook types.Logbook
		if err := c.BodyParser(&typeLogbook); err != nil {
			s.logger.ErrorWith().Msg("Invalid request body")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
		}

		fmt.Println(typeLogbook)

		fullKey, prefix, hash, err := apikey.Generate(10)
		if err != nil {
			s.logger.ErrorWith().Msg("Key generation failed")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "key generation failed"})
		}

		typeLogbook, err = s.db.InsertLogbookContext(c.UserContext(), typeLogbook)
		if err != nil {
			s.logger.ErrorWith().Err(err).Msg("Database error")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database error"})
		}

		fmt.Println("Logbook registered:", typeLogbook.ID)

		fmt.Println("Store the hash: ", hash, " in the database")
		fmt.Println("Store the prefix: ", prefix, " in the database")

		c.Status(fiber.StatusCreated)

		// Return the full key to the client as a one-off deal
		return c.JSON(fiber.Map{"api_key": fullKey})
	}
}
