package server

import (
	"github.com/Station-Manager/apikey"
	"github.com/Station-Manager/types"
	"github.com/gofiber/fiber/v2"
	"strings"
	"time"
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
				s.logger.ErrorWith().Err(err).Str("callsign", callsign).Msg("s.db.FetchUserByCallsign")
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
			}

			var valid bool
			valid, err = apikey.ValidateBootstrap(token, user.BootstrapHash)
			if err != nil {
				s.logger.ErrorWith().Err(err).Str("token", token).Msg("apikey.ValidateBootstrap")
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
			}
			if !valid {
				s.logger.InfoWith().Str("token", token).Msg("Invalid bootstrap token")
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
			}
		}

		//fmt.Println("User:", user)
		var typeLogbook types.Logbook
		if err = c.BodyParser(&typeLogbook); err != nil {
			s.logger.ErrorWith().Msg("Invalid request body")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
		}

		typeLogbook.UserID = user.ID // FK contraint

		// Create the logbook in the database
		typeLogbook, err = s.db.InsertLogbookContext(c.UserContext(), typeLogbook)
		if err != nil {
			s.logger.ErrorWith().Err(err).Msg("s.db.InsertLogbookContext")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database error"})
		}

		// Generate a new API key for the logbook
		fullKey, prefix, hash, err := apikey.Generate(10)
		if err != nil {
			s.logger.ErrorWith().Err(err).Msg("apikey.Generate")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "key generation failed"})
		}

		if err = s.db.InsertAPIKey(typeLogbook.Name, prefix, hash, typeLogbook.ID); err != nil {
			s.logger.ErrorWith().Err(err).Msg("s.db.InsertAPIKey")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database error"})
		}

		user.BootstrapHash = ""
		user.BootstrapUsedAt = time.Now().UTC()

		if err = s.db.UpdateUserContext(c.UserContext(), user); err != nil {
			s.logger.ErrorWith().Err(err).Interface("types.User", user).Msg("s.db.UpdateUserContext")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database error"})
		}

		// Return the full key to the client as a one-off deal
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"api_key": fullKey})
	}
}
