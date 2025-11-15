package server

import "github.com/gofiber/fiber/v2"

var (
	jsonUnauthorized  = fiber.Map{"message": "Unauthorized"}
	jsonInvalidAction = fiber.Map{"message": "Invalid action"}
	jsonInternalError = fiber.Map{"message": "Internal error"}
	jsonBadRequest    = fiber.Map{"message": "Bad request"}
)
