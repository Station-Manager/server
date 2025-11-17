package server

import (
	"context"
	"github.com/Station-Manager/adapters"
	"github.com/Station-Manager/adapters/converters/common"
	"github.com/Station-Manager/apikey"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
	"github.com/gofiber/fiber/v2"
)

// basicChecks performs basic/common checks on the request context, returning early if any of them fail.
func (s *Service) basicChecks() fiber.Handler {
	if s == nil {
		return serverErrorHandler()
	}

	return func(c *fiber.Ctx) error {
		const op errors.Op = "server.Service.basicChecks"
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

		// 2. Validate the request body that no fields the required field exist.
		if err := validatePostRequest(op, request); err != nil {
			s.logger.ErrorWith().Err(err).Msg("validatePostRequest")
			return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
		}

		// 3. Check for a valid action
		isValidAction, err := s.isValidAction(request.Action)
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

		// Prepare unified request context
		rc := &requestContext{
			Request: request,
			IsValid: false, // will be set true after a successful authn
		}

		// 4. Check if the action requires the user's password or API key
		// Registering a logbook requires the user's password, not the API key
		// as the API key is a per-logbook key
		if request.Action == types.RegisterLogbookAction {
			user, err := s.fetchUser(c.UserContext(), request.Callsign)
			if err != nil {
				err = errors.New(op).Err(err)
				s.logger.ErrorWith().Err(err).Msg("s.fetchUser")
				return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
			}

			valid, err := s.isValidPassword(user.PassHash, request.Key)
			if err != nil {
				err = errors.New(op).Err(err)
				s.logger.ErrorWith().Err(err).Msg("s.isValidPassword")
				return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
			}

			if !valid {
				s.logger.InfoWith().Str("callsign", request.Callsign).Msg("Invalid password")
				return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
			}

			rc.IsValid = true
			rc.User = &user
		} else {
			// Validate an API key and get the associated logbook ID.
			validApiKey, logbookId, err := s.isValidApiKey(c.UserContext(), request.Key)
			if err != nil {
				err = errors.New(op).Err(err)
				s.logger.ErrorWith().Err(err).Msg("s.isValidApiKey")
				return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
			}

			if !validApiKey {
				return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
			}

			logbook, err := s.fetchLogbookWithCache(c.UserContext(), logbookId)
			if err != nil {
				err = errors.New(op).Err(err)
				s.logger.ErrorWith().Err(err).Msg("s.fetchLogbookWithCache")
				return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
			}

			rc.IsValid = true
			rc.Logbook = &logbook
		}

		// 5. Store the unified request context in locals for downstream handlers.
		c.Locals(localsRequestDataKey, rc)

		return c.Next()
	}
}

// fetchUser fetches a user from the database by their callsign.
func (s *Service) fetchUser(ctx context.Context, callsign string) (types.User, error) {
	const op errors.Op = "server.Service.fetchUser"
	emptyRetVal := types.User{}
	if s == nil {
		return emptyRetVal, errors.New(op).Msg(errMsgNilService)
	}
	if callsign == emptyString {
		return emptyRetVal, errors.New(op).Msg("Callsign is empty")
	}

	model, err := s.db.FetchUserByCallsignContext(ctx, callsign)
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

	if user.EmailConfirmed == false {
		err = errors.New(op).Msg("User's email has not been verified")
		s.logger.ErrorWith().Err(err).Msg("User email not verified")
		return emptyRetVal, err
	}

	return user, nil
}

// isValidAction checks the Action field and returns the corresponding action enum value.
// If the action is missing or invalid, an error is returned.
func (s *Service) isValidAction(action types.RequestAction) (bool, error) {
	const op errors.Op = "server.Service.checkActionHeader"

	switch action {
	case types.RegisterLogbookAction:
		return true, nil
	case types.InsertQsoAction:
		return true, nil
	default:
		return false, errors.New(op).Errorf("Unknown action: %s", action)
	}
}

// isValidApiKey validates an API key by checking its prefix and hashed value against the stored database records.
// Returns the logbook ID if the key is valid.
func (s *Service) isValidApiKey(ctx context.Context, fullKey string) (bool, int64, error) {
	const op errors.Op = "server.Service.isValidApiKey"
	if ctx == nil {
		return false, 0, errors.New(op).Msg(errMsgNilContext)
	}

	if fullKey == emptyString {
		return false, 0, errors.New(op).Msg("API key is empty")
	}

	prefix, _, err := apikey.ParseApiKey(fullKey)
	if err != nil {
		return false, 0, errors.New(op).Err(err)
	}

	model, err := s.db.FetchAPIKeyByPrefixContext(ctx, prefix)
	if err != nil {
		return false, 0, errors.New(op).Err(err)
	}

	valid, err := apikey.ValidateApiKey(fullKey, model.KeyHash)
	if err != nil {
		return false, 0, errors.New(op).Err(err)
	}

	// Sanity check
	if model.LogbookID == 0 {
		return false, 0, errors.New(op).Msg("Logbook ID is zero")
	}

	if !valid {
		return false, 0, nil
	}
	return valid, model.LogbookID, nil
}

// isValidPassword checks if a password matches the hashed value stored in the database.
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

// validatePostRequest validates the request body for a POST request.
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
	// For action-specific payloads, enforce the presence of the correct typed field.
	switch req.Action {
	case types.RegisterLogbookAction:
		if req.Logbook == nil {
			return errors.New(op).Msg("Logbook payload is missing")
		}
	case types.InsertQsoAction:
		if req.Qso == nil {
			return errors.New(op).Msg("QSO payload is missing")
		}
	}
	return nil
}
