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
		return errors.New(op).Msg(errMsgNilContext)
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
	if err := s.validate.Struct(logbook); err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err).Msg("Validation failed")
		return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
	}

	// 2. Associated the logbook with the user.
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

	// Begin the transaction for atomic logbook + API key creation.
	tx, txCancel, err := s.db.BeginTxContext(ctx)
	if err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("s.db.BeginTxContext")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}
	defer txCancel()

	txCtx := ctx

	// Insert logbook inside transaction.
	logbook, err = s.db.InsertLogbookWithTxContext(txCtx, tx, logbook)
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
	fullKey, prefix, hash, err := apikey.GenerateApiKey(10)
	if err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("apikey.GenerateApiKey")
		_ = tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// Insert API key within same transaction.
	if err = s.db.InsertAPIKeyWithTxContext(txCtx, tx, logbook.Callsign, prefix, hash, logbook.ID); err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("s.db.InsertAPIKeyWithTxContext")
		_ = tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	// Commit transaction.
	if err = tx.Commit(); err != nil {
		wrapped := errors.New(op).Err(err)
		s.logger.ErrorWith().Err(wrapped).Msg("tx.Commit")
		_ = tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": fullKey})
}
