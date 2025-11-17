package server

import (
	"github.com/Station-Manager/apikey"
	"github.com/Station-Manager/errors"
	"github.com/gofiber/fiber/v2"
)

// registerLogbookAction processes the creation of a user logbook, validates input, and generates an API key within a transaction.
// It extracts data from the request, validates it, assigns user ownership, and interacts with the database for persistence.
func (s *Service) registerLogbookAction(c *fiber.Ctx) error {
	const op errors.Op = "server.Service.registerLogbookAction"
	if c == nil {
		return errors.New(op).Msg(errMsgNilContext)
	}

	rc, err := getRequestContext(c)
	if err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err).Msg("getRequestContext failed")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	if rc.Request.Logbook == nil {
		err = errors.New(op).Msg("Logbook payload is nil")
		s.logger.ErrorWith().Err(err).Msg("Logbook payload is nil")
		return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
	}

	// Work on a copy so we do not mutate the original request struct.
	logbook := *rc.Request.Logbook

	// 2. Validate the logbook data provided by the API caller
	if err := s.validate.Struct(logbook); err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err).Msg("Validation failed")
		return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
	}

	// 3. Associate the logbook with the user. This is the only time the user data is available.
	if rc.User == nil {
		err := errors.New(op).Msg("User is nil in request context")
		s.logger.ErrorWith().Err(err).Msg("User is nil")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}
	logbook.UserID = rc.User.ID

	ctx := c.UserContext()
	if ctx == nil {
		return errors.New(op).Msg(errMsgNilContext)
	}
	if s.db == nil {
		err := errors.New(op).Msg("database service is nil")
		s.logger.ErrorWith().Err(err).Msg("database service is nil")
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
		if rbErr := tx.Rollback(); rbErr != nil {
			s.logger.ErrorWith().Err(rbErr).Msg("Failed to rollback transaction after InsertLogbookWithTxContext error")
		}
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}
	if logbook.ID == 0 {
		wrapped := errors.New(op).Msg("Logbook ID was not set")
		s.logger.ErrorWith().Err(wrapped).Msg("Logbook ID was not set")
		if rbErr := tx.Rollback(); rbErr != nil {
			s.logger.ErrorWith().Err(rbErr).Msg("Failed to rollback transaction after logbook ID check")
		}
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// Generate API key.
	fullKey, prefix, hash, err := apikey.GenerateApiKey(prefixLen)
	if err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("apikey.GenerateApiKey")
		if rbErr := tx.Rollback(); rbErr != nil {
			s.logger.ErrorWith().Err(rbErr).Msg("Failed to rollback transaction after GenerateApiKey error")
		}
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// Insert API key within same transaction.
	if err = s.db.InsertAPIKeyWithTxContext(ctx, tx, logbook.Callsign, prefix, hash, logbook.ID); err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("s.db.InsertAPIKeyWithTxContext")
		if rbErr := tx.Rollback(); rbErr != nil {
			s.logger.ErrorWith().Err(rbErr).Msg("Failed to rollback transaction after InsertAPIKeyWithTxContext error")
		}
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// Commit transaction.
	if err = tx.Commit(); err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("tx.Commit")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// Return the full API key associated with the logbook to the caller.
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": fullKey})
}
