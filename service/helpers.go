package service

import (
	stderr "errors"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
)

type requestContext struct {
	Request types.PostRequest
	User    *types.User
	Logbook *types.Logbook
	IsValid bool
}

// getRequestContext retrieves the `requestContext` from the Fiber context's local storage.
// Returns an error if the local data cannot be cast to `*requestContext` or if it is nil.
func getRequestContext(c *fiber.Ctx) (*requestContext, error) {
	const op errors.Op = "server.Service.getRequestContext"
	ctx, ok := c.Locals(localsRequestDataKey).(*requestContext)
	if !ok || ctx == nil {
		return nil, errors.New(op).Msg("Unable to cast locals to *requestContext")
	}
	return ctx, nil
}

func postgresError(err error) (string, bool) {

	var pgErr *pq.Error
	if stderr.As(err, &pgErr) {
		// pgErr.Code is the SQLSTATE, e.g. "23505" for unique_violation
		if pgErr != nil {
			switch pgErr.Code {
			case "23505":
				return "Duplicate", true
			default:
				// handle other PG errors
			}
		}
	}
	return "", false
}
