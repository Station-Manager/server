package server

import (
	"github.com/Station-Manager/adapters"
	"github.com/Station-Manager/adapters/converters/common"
	"github.com/Station-Manager/apikey"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
	"github.com/gofiber/fiber/v2"
)

// basicChecks performs basic/common checks on the request context, returning early if any of them fail.
func (s *Service) basicChecks() fiber.Handler {
	return func(c *fiber.Ctx) error {
		const op errors.Op = "server.Service.apiKeyCheck"
		if c == nil {
			return errors.New(op).Msg(errMsgNilContext)
		}

		// 1. Parse request body. All valid requests have the same structure.
		var request types.PostRequest
		if err := c.BodyParser(&request); err != nil {
			err = errors.New(op).Err(err)
			s.logger.ErrorWith().Err(err).Msg("c.BodyParser")
			return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
		}

		// 2. Validate the request body that no fields are empty.
		if err := validatePostRequest(op, request); err != nil {
			s.logger.ErrorWith().Err(err).Msg("validatePostRequest")
			return err
		}

		// 3. Find the user
		user, err := s.fetchUser(request.Callsign)
		if err != nil {
			err = errors.New(op).Err(err)
			s.logger.ErrorWith().Err(err).Msg("s.fetchUser")
			return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
		}

		// 4. Check for a valid action
		isValidAction, err := s.isValidateAction(request.Action)
		if err != nil {
			err = errors.New(op).Err(err)
			s.logger.ErrorWith().Err(err).Msg("s.isValidateAction")
			return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
		}
		if !isValidAction {
			err = errors.New(op).Msg("Invalid action")
			s.logger.ErrorWith().Err(err).Msg("s.isValidateAction")
			return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
		}

		var valid bool
		// Registering a logbook requires the user's password, not the api key
		// as the api key is a per-logbook key
		if request.Action == types.RegisterLogbookAction {
			if valid, err = s.isValidPassword(user.PassHash, request.Key); err != nil {
				err = errors.New(op).Err(err)
				s.logger.ErrorWith().Err(err).Msg("s.isValidPassword")
				return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
			}
		} else {
			if valid, err = s.isValidApiKey(request.Key); err != nil {
				err = errors.New(op).Err(err)
				s.logger.ErrorWith().Err(err).Msg("s.isValidApiKey")
				return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
			}
		}

		// 5. Store the request data in the context
		c.Locals(localsRequestDataKey, requestData{
			IsValid: valid,
			Action:  request.Action,
			Data:    request.Data,
		})

		// 6. Store the user in the context
		c.Locals(localsUserDataKey, user)

		return c.Next()
	}
}

func (s *Service) fetchUser(callsign string) (types.User, error) {
	const op errors.Op = "server.Service.fetchUser"
	emptyRetVal := types.User{}
	if s == nil {
		return emptyRetVal, errors.New(op).Msg(errMsgNilService)
	}
	if callsign == emptyString {
		return emptyRetVal, errors.New(op).Msg("Callsign is empty")
	}

	model, err := s.db.FetchUserByCallsign(callsign)
	if err != nil {
		return emptyRetVal, errors.New(op).Err(err)
	}

	adapter := adapters.New()
	adapter.RegisterConverter("PassHash", common.ModelToTypeStringConverter)
	adapter.RegisterConverter("Issuer", common.ModelToTypeStringConverter)
	adapter.RegisterConverter("Subject", common.ModelToTypeStringConverter)
	adapter.RegisterConverter("Email", common.ModelToTypeStringConverter)
	adapter.RegisterConverter("EmailConfirmed", common.ModelToTypeBoolConverter)

	var user types.User
	if err = adapter.Into(&user, &model); err != nil {
		return emptyRetVal, errors.New(op).Err(err).Msg("Failed to convert model to user")
	}

	// TODO: check if the user's account has been verified.

	return user, nil
}

// checkActionHeader checks the X-Action header and returns the corresponding action enum value.
// If the action is missing or invalid, an error is returned.
func (s *Service) isValidateAction(action types.RequestAction) (bool, error) {
	const op errors.Op = "server.Service.checkActionHeader"

	switch action {
	case types.RegisterLogbookAction:
		return true, nil
	default:
		return false, errors.New(op).Errorf("Unknown action: %s", action)
	}
}

func (s *Service) isValidApiKey(apiKey string) (bool, error) {
	const op errors.Op = "server.Service.isValidApiKey"
	if apiKey == emptyString {
		return false, errors.New(op).Msg("API key is empty")
	}

	return false, nil
}

func (s *Service) isValidPassword(hash, pass string) (bool, error) {
	const op errors.Op = "server.Service.isValidPassword"
	if pass == emptyString || hash == emptyString {
		return false, errors.New(op).Msg("Password or hash is empty")
	}
	valid, err := apikey.VerifyPassword(hash, pass)
	if err != nil {
		return false, errors.New(op).Err(err)
	}
	return valid, nil
}

func validatePostRequest(op errors.Op, req types.PostRequest) error {
	if req.Action == emptyString {
		return errors.New(op).Msg("Action is empty")
	}
	if req.Key == emptyString {
		return errors.New(op).Msg("API key is empty")
	}
	if req.Callsign == emptyString {
		return errors.New(op).Msg("Callsign is empty")
	}
	if req.Data == emptyString {
		return errors.New(op).Msg("Data is empty")
	}
	return nil
}
