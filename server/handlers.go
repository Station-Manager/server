package server

import (
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
	"github.com/gofiber/fiber/v2"
)

// postDispatcherHandler handles all POST requests to the server.
func (s *Service) postDispatcherHandler() fiber.Handler {
	const op errors.Op = "server.Service.postDispatcherHandler"
	if s == nil {
		return serverErrorHandler()
	}

	return func(c *fiber.Ctx) error {
		if c == nil {
			return errors.New(op).Msg(errMsgNilContext)
		}

		state, err := getRequestData(c)
		if err != nil {
			err = errors.New(op).Err(err)
			s.logger.ErrorWith().Err(err)
			return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
		}

		// Sanity check
		if !state.IsValid {
			s.logger.InfoWith().Msg("Invalid request data")
			return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
		}

		switch state.Action {
		case types.InsertQsoAction:
			return s.insertQsoAction(c)
		case types.RegisterLogbookAction:
			return s.registerLogbookAction(c)
		default:
			return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
		}
	}
}

func serverErrorHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}
}
