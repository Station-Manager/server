package server

import (
	"fmt"
	"github.com/Station-Manager/apikey"
	"github.com/Station-Manager/types"
	"github.com/gofiber/fiber/v2"
	"strings"
)

func (s *Service) addLogbookHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		s.logger.DebugWith().Msg("Registering a new logbook")
		token := c.Get(fiber.HeaderAuthorization)

		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}

		callsign := c.Get("X-Callsign")

		var isBootstrap bool
		if strings.HasPrefix(token, "Bootstrap ") {
			// This is a bootstrap token only ever sent when bootstrapping a new user
			if callsign == "" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
			}

			isBootstrap = true
			token = token[10:]
		} else if !strings.HasPrefix(token, "ApiKey ") {
			// no-op
			token = token[7:]
		} else {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}

		var err error
		var user types.User
		if isBootstrap {
			if user, err = s.db.FetchUserByCallsign(callsign); err != nil {
				// TODO: log error
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
			}

			var valid bool
			valid, err = apikey.ValidateBootstrap(token, user.BootstrapHash)
			if err != nil {
				// TODO: log error
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
			}
			if !valid {
				//TODO: log error
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
			}
		}

		//fmt.Println("User:", user)
		var typeLogbook types.Logbook
		if err = c.BodyParser(&typeLogbook); err != nil {
			s.logger.ErrorWith().Msg("Invalid request body")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
		}

		// Create the logbook in the database
		typeLogbook, err = s.db.InsertLogbookContext(c.UserContext(), typeLogbook)
		if err != nil {
			s.logger.ErrorWith().Err(err).Msg("Database error")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database error"})
		}

		// Generate a new API key for the logbook
		fullKey, prefix, hash, err := apikey.Generate(10)
		if err != nil {
			s.logger.ErrorWith().Msg("Key generation failed")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "key generation failed"})
		}

		if err = s.db.InsertAPIKey(typeLogbook.Name, prefix, hash, typeLogbook.ID); err != nil {
			s.logger.ErrorWith().Err(err).Msg("Database error")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database error"})
		}

		fmt.Println("Generated full key: ", fullKey)

		// Return the full key to the client as a one-off deal
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"api_key": fullKey})
	}
}
