package service

import (
	"github.com/gofiber/fiber/v2"
)

func serverErrorHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
	}
}

// healthHandler returns a simple health status. It can be extended to include more checks.
func (s *Service) healthHandler(c *fiber.Ctx) error {
	// Default to unhealthy; update to healthy on success
	status := "ok"
	dbStatus := "unknown"

	if s.db != nil {
		if err := s.db.Ping(); err != nil {
			status = "degraded"
			dbStatus = "unreachable"
		} else {
			dbStatus = "up"
		}
	} else {
		status = "degraded"
		dbStatus = "not_configured"
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": status,
		"db":     dbStatus,
	})
}
