package server

import (
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
	"github.com/gofiber/fiber/v2"
)

type requestContext struct {
	Request types.PostRequest
	User    *types.User
	Logbook *types.Logbook
	IsValid bool
}

func getRequestContext(c *fiber.Ctx) (*requestContext, error) {
	const op errors.Op = "server.Service.getRequestContext"
	if c == nil {
		return nil, errors.New(op).Msg(errMsgNilContext)
	}
	ctx, ok := c.Locals(localsRequestDataKey).(*requestContext)
	if !ok || ctx == nil {
		return nil, errors.New(op).Msg("Unable to cast locals to *requestContext")
	}
	return ctx, nil
}
