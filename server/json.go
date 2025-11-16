package server

import "github.com/gofiber/fiber/v2"

var (
	jsonUnauthorized  = fiber.Map{"message": "Unauthorized"}
	jsonInternalError = fiber.Map{"message": "Internal error"}
	jsonBadRequest    = fiber.Map{"message": "Bad request"}
)
