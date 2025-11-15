package server

import (
	"github.com/Station-Manager/apikey"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
)

func (s *Service) registerLogbookAction(c *fiber.Ctx) error {
	const op errors.Op = "server.Service.registerLogbookAction"
	if c == nil {
		return errors.New(op).Msg(errMsgNilService)
	}

	reqData, ok := c.Locals(localsRequestDataKey).(requestData)
	if !ok {
		err := errors.New(op).Msg("Unable to cast locals to requestData")
		s.logger.ErrorWith().Err(err)
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	var logbook types.Logbook
	if err := json.Unmarshal([]byte(reqData.Data), &logbook); err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err).Msg("json.Unmarshal")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// 1. Validate the logbook data
	// TODO: validate logbook

	// 2. Associated the logbook with the user.
	user, ok := c.Locals(localsUserDataKey).(types.User)
	if !ok {
		err := errors.New(op).Msg("Unable to cast locals to user")
		s.logger.ErrorWith().Err(err)
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	logbook.UserID = user.ID

	// 3. Insert the logbook into the database.
	var err error
	if logbook, err = s.db.InsertLogbookContext(c.UserContext(), logbook); err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err).Msg("s.db.InsertLogbookContext")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	if logbook.ID == 0 {
		err := errors.New(op).Msg("Logbook ID was not set")
		s.logger.ErrorWith().Err(err)
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// 4. Generate an API key for the logbook.
	fullKey, prefix, hash, err := apikey.GenerateApiKey(10)
	if err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err).Msg("apikey.GenerateApiKey")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	if err = s.db.InsertAPIKey(logbook.Callsign, prefix, hash, logbook.ID); err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err).Msg("s.db.InsertAPIKey")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": fullKey})
}
