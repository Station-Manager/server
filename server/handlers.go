package server

import (
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
	"github.com/gofiber/fiber/v2"
)

func (s *Service) postDispatcherHandler() fiber.Handler {
	const op errors.Op = "server.Service.postDispatcherHandler"
	if s == nil {
		return nil
	}

	return func(c *fiber.Ctx) error {
		if c == nil {
			return errors.New(op).Msg(errMsgNilContext)
		}

		state, ok := c.Locals(localsRequestDataKey).(requestData)
		if !ok {
			err := errors.New(op).Msg("Unable to cast locals to requestData")
			s.logger.ErrorWith().Err(err)
			return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
		}

		// Sanity check
		if !state.IsValid {
			s.logger.InfoWith().Msg("Invalid request data")
			return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
		}

		switch state.Action {
		case types.RegisterLogbookAction:
			return s.registerLogbookAction(c)
		default:
			return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
		}
	}
}
