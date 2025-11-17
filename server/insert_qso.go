package server

import (
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
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

	// Sanity check
	if state.Logbook.ID == 0 {
		err = errors.New(op).Msg("Logbook ID was not set")
		s.logger.ErrorWith().Err(err)
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	postReq, ok := c.Locals("postRequest").(types.PostRequest)
	if !ok {
		err := errors.New(op).Msg("Unable to cast locals to PostRequest")
		s.logger.ErrorWith().Err(err)
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}
	if postReq.Qso == nil {
		err := errors.New(op).Msg("QSO payload is nil")
		s.logger.ErrorWith().Err(err)
		return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
	}

	// Work on a copy so we do not mutate the original request struct.
	qso := *postReq.Qso

	// The `station_callsign` must be set and must match the logbook's callsign.
	if qso.StationCallsign != state.Logbook.Callsign {
		err = errors.New(op).Msg("QSO callsign does not match the Logbook's callsign")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "QSO callsign does not match the Logbook's callsign"})
	}
	qso.LogbookID = state.Logbook.ID

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
