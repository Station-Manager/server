package service

import (
	"github.com/Station-Manager/apikey"
	"github.com/Station-Manager/errors"
	"github.com/gofiber/fiber/v2"
)

// registerLogbookHandler handles registration of a new logbook, including validation, persistence, and API key generation.
func (s *Service) registerLogbookHandler(c *fiber.Ctx) error {
	const op errors.Op = "server.Service.registerLogbookAction"

	// 1. Extract the unified request context from the fiber context.
	reqCtx, err := getRequestContext(c)
	if err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("getRequestContext failed")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// 2. Check that we have a logbook payload.
	// At this point we are guaranteed that the user is authenticated.
	if reqCtx.Request.Logbook == nil {
		wrapped := errors.New(op).Msg("Logbook payload is nil")
		s.logger.ErrorWith().Err(wrapped).Msg("Logbook payload is nil")
		return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
	}

	// Work on a copy so we do not mutate the original request struct.
	logbook := *reqCtx.Request.Logbook

	// 3. Validate the logbook payload provided by the API caller
	if err = s.validate.Struct(logbook); err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("Validation failed")
		//TODO: Provide more detailed feedback to the caller
		return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
	}

	// Sanity check: the user should always be set.
	if reqCtx.User == nil {
		wrapped := errors.New(op).Msg("User is nil in request context")
		s.logger.ErrorWith().Err(wrapped).Msg("User is nil")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// Sanity check: the database service should always be set.
	if s.db == nil {
		wrapped := errors.New(op).Msg("database service is nil")
		s.logger.ErrorWith().Err(wrapped).Msg("database service is nil")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	ctx := c.UserContext()

	// 4. Begin the transaction for atomic logbook + API key creation.
	tx, txCancel, err := s.db.BeginTxContext(ctx)
	if err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("s.db.BeginTxContext")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}
	defer txCancel()

	// Assocated the user with this logbook.
	logbook.UserID = reqCtx.User.ID

	// 4a. Insert a logbook inside the transaction.
	logbook, err = s.db.InsertLogbookWithTxContext(ctx, tx, logbook)
	if err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("s.db.InsertLogbookWithTxContext failed")
		if rbErr := tx.Rollback(); rbErr != nil {
			s.logger.ErrorWith().Err(rbErr).Msg("Failed to rollback transaction after InsertLogbookWithTxContext error")
		}

		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// Sanity check: the logbook ID should always be set if the above succeeded.
	if logbook.ID == 0 {
		wrapped := errors.New(op).Msg("Logbook ID was not set")
		s.logger.ErrorWith().Err(wrapped).Msg("Logbook ID was not set")
		if rbErr := tx.Rollback(); rbErr != nil {
			s.logger.ErrorWith().Err(rbErr).Msg("Failed to rollback transaction after logbook ID check")
		}

		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// 4b. Generate an API key for the logbook.
	fullKey, prefix, hash, err := apikey.GenerateApiKey(prefixLen)
	if err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("apikey.GenerateApiKey failed")
		if rbErr := tx.Rollback(); rbErr != nil {
			s.logger.ErrorWith().Err(rbErr).Msg("Failed to rollback transaction after GenerateApiKey error")
		}

		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// 4c. Insert API key within same transaction.
	if err = s.db.InsertAPIKeyWithTxContext(ctx, tx, logbook.Callsign, prefix, hash, logbook.ID); err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("s.db.InsertAPIKeyWithTxContext")
		if rbErr := tx.Rollback(); rbErr != nil {
			s.logger.ErrorWith().Err(rbErr).Msg("Failed to rollback transaction after InsertAPIKeyWithTxContext error")
		}

		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// 5. Commit transaction. No need to rollback if the commit fails.
	if err = tx.Commit(); err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("tx.Commit")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// Return the full API key associated with the logbook back to the caller.
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": fullKey})
}
