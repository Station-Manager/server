package server

import (
	"github.com/Station-Manager/apikey"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
	"github.com/gofiber/fiber/v2"
)

// registerLogbookAction processes the creation of a user logbook, validates input, and generates an API key within a transaction.
// It extracts data from the request, validates it, assigns user ownership, and interacts with the database for persistence.
func (s *Service) registerLogbookAction(c *fiber.Ctx) error {
	const op errors.Op = "server.Service.registerLogbookAction"
	if c == nil {
		return errors.New(op).Msg(errMsgNilContext)
	}

	// 1. Fetch the typed request from the context.
	postReq, ok := c.Locals("postRequest").(types.PostRequest)
	if !ok {
		err := errors.New(op).Msg("Unable to cast locals to PostRequest")
		s.logger.ErrorWith().Err(err)
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	if postReq.Logbook == nil {
		err := errors.New(op).Msg("Logbook payload is nil")
		s.logger.ErrorWith().Err(err)
		return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
	}

	// Work on a copy so we do not mutate the original request struct.
	logbook := *postReq.Logbook

	// 2. Validate the logbook data provided by the API caller
	if err := s.validate.Struct(logbook); err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err).Msg("Validation failed")
		return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
	}

	// 3. Associate the logbook with the user. This is the only time the user data is available.
	user, ok := c.Locals(localsUserDataKey).(types.User)
	if !ok {
		err := errors.New(op).Msg("Unable to cast locals to user")
		s.logger.ErrorWith().Err(err)
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}
	logbook.UserID = user.ID

	ctx := c.UserContext()
	if ctx == nil {
		return errors.New(op).Msg(errMsgNilContext)
	}
	if s.db == nil {
		err := errors.New(op).Msg("database service is nil")
		s.logger.ErrorWith().Err(err)
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// 4. Begin the transaction for atomic logbook + API key creation.
	tx, txCancel, err := s.db.BeginTxContext(ctx)
	if err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("s.db.BeginTxContext")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}
	defer txCancel()

	// Insert logbook inside transaction.
	logbook, err = s.db.InsertLogbookWithTxContext(ctx, tx, logbook)
	if err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("s.db.InsertLogbookWithTxContext")
		_ = tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}
	if logbook.ID == 0 {
		wrapped := errors.New(op).Msg("Logbook ID was not set")
		s.logger.ErrorWith().Err(wrapped)
		_ = tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// Generate API key.
	fullKey, prefix, hash, err := apikey.GenerateApiKey(prefixLen)
	if err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("apikey.GenerateApiKey")
		_ = tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// Insert API key within same transaction.
	if err = s.db.InsertAPIKeyWithTxContext(ctx, tx, logbook.Callsign, prefix, hash, logbook.ID); err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("s.db.InsertAPIKeyWithTxContext")
		_ = tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// Commit transaction.
	if err = tx.Commit(); err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("tx.Commit")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": fullKey})
}
