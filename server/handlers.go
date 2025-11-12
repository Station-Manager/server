package server

import "github.com/gofiber/fiber/v2"

func (s *Service) addLogbookHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Hello, World!"})
	}
}
