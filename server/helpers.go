package server

import (
	"github.com/Station-Manager/errors"
	"github.com/gofiber/fiber/v2"
)

func getRequestData(c *fiber.Ctx) (requestData, error) {
	const op errors.Op = "server.Service.getRequestData"
	if c == nil {
		return requestData{}, errors.New(op).Msg(errMsgNilContext)
	}
	state, ok := c.Locals(localsRequestDataKey).(requestData)
	if !ok {
		return requestData{}, errors.New(op).Msg("Unable to cast locals to requestData")
	}
	return state, nil
}
