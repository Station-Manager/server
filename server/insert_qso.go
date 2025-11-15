package server

import (
	"fmt"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
)

func (s *Service) insertQsoAction(c *fiber.Ctx) error {
	const op errors.Op = "server.Service.insertQSOAction"
	if c == nil {
		return errors.New(op).Msg(errMsgNilService)
	}

	reqData, ok := c.Locals(localsRequestDataKey).(requestData)
	if !ok {
		err := errors.New(op).Msg("Unable to cast locals to requestData")
		s.logger.ErrorWith().Err(err)
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	var qso types.Qso
	if err := json.Unmarshal([]byte(reqData.Data), &qso); err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err).Msg("json.Unmarshal")
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}

	qso.LogbookID = reqData.LogbookID

	if err := s.validate.Struct(qso); err != nil {
		err = errors.New(op).Err(err)
		s.logger.ErrorWith().Err(err).Msg("Validation failed")
		return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
	}

	fmt.Println(qso)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "Successful"})
}
