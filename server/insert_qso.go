package server

import (
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
)

func (s *Service) insertQsoAction(c *fiber.Ctx) error {
	const op errors.Op = "server.Service.insertQSOAction"
	if c == nil {
		return errors.New(op).Msg(errMsgNilContext)
	}

	state, err := getRequestData(c)
	if err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err)
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	var qso types.Qso
	if err = json.Unmarshal([]byte(state.Data), &qso); err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err).Msg("json.Unmarshal")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	qso.LogbookID = state.LogbookID

	// TODO: structured error codes for fields?
	if err = s.validate.Struct(qso); err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err).Msg("Validation failed")
		return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
	}

	if qso, err = s.db.InsertQsoContext(c.UserContext(), qso); err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err).Msg("InsertQso failed")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "QSO Created"})
}
