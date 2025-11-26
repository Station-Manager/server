package service

import (
	"context"
	"github.com/Station-Manager/adapters"
	"github.com/Station-Manager/adapters/converters/common"
	"github.com/Station-Manager/apikey"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
	"github.com/gofiber/fiber/v2"
)

// fetchUser fetches a user from the database by their callsign.
func (s *Service) fetchUser(ctx context.Context, callsign string) (types.User, error) {
	const op errors.Op = "server.Service.fetchUser"
	emptyRetVal := types.User{}

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

	if fullKey == emptyString {
		return false, 0, errors.New(op).Msg("API key is empty")
	}

	prefix, _, err := apikey.ParseApiKey(fullKey)
	if err != nil {
		return false, 0, errors.New(op).Err(err)
	}

	// Database call to the api_keys table
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

// requestContextMiddleware sets up middleware to preprocess request context and store it in locals for downstream handlers.
// This middleware is applied to all /api routes only.
func (s *Service) requestContextMiddleware() fiber.Handler {
	const op errors.Op = "server.Service.requestContextMiddleware"
	if s == nil {
		return serverErrorHandler()
	}

	return func(c *fiber.Ctx) error {
		// 1. Parse request body. All valid requests have the same structure.
		var request types.PostRequest
		if err := c.BodyParser(&request); err != nil {
			err = errors.New(op).Err(err)
			s.logger.ErrorWith().Err(err).Msg("c.BodyParser")
			return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
		}

		if request.Callsign == "" {
			s.logger.InfoWith().Str("callsign", request.Callsign).Msg("Callsign is empty")
			return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
		}

		if request.Key == "" {
			s.logger.InfoWith().Str("callsign", request.Callsign).Msg("API key is empty")
			return c.Status(fiber.StatusBadRequest).JSON(jsonBadRequest)
		}

		// 2. Prepare unified request context
		reqCtx := &requestContext{
			Request: request,
			IsValid: false, // will be set true after a successful authn
		}

		// 3. Store the unified request context in locals for downstream handlers.
		c.Locals(localsRequestDataKey, reqCtx)

		return c.Next()
	}
}

// apikeyAuthNMiddleware verifies API keys for incoming requests, fetches related logbook data, and updates the request context.
// It handles errors related to authentication, validation, and logbook retrieval, responding with appropriate HTTP status codes.
func (s *Service) apikeyAuthNMiddleware() fiber.Handler {
	const op errors.Op = "server.Service.apikeyMiddleware"
	if s == nil {
		return serverErrorHandler()
	}

	return func(c *fiber.Ctx) error {
		// 1. Fetch unified request context from locals.
		reqCtx, err := getRequestContext(c)
		if err != nil {
			err = errors.New(op).Err(err)
			s.logger.ErrorWith().Err(err).Msg("getRequestContext failed")
			return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
		}

		// Validate an API key and get the associated logbook ID.
		validApiKey, logbookId, err := s.isValidApiKey(c.UserContext(), reqCtx.Request.Key)
		if err != nil {
			err = errors.New(op).Err(err)
			s.logger.ErrorWith().Err(err).Msg("s.isValidApiKey failed")
			return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
		}

		if !validApiKey {
			s.logger.InfoWith().Str("callsign", reqCtx.Request.Callsign).Msg("Invalid API key")
			return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
		}

		reqCtx.IsValid = validApiKey

		logbook, err := s.fetchLogbookWithCache(c.UserContext(), logbookId)
		if err != nil {
			err = errors.New(op).Err(err)
			s.logger.ErrorWith().Err(err).Msg("s.fetchLogbookWithCache failed")
			return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
		}

		reqCtx.Logbook = &logbook

		// The API key is no longer needed after a successful authn.
		// This prevents accidental leakage further down-stream.
		reqCtx.Request.Key = ""

		return c.Next()
	}
}

// passwordAuthNMiddleware authenticates a user by checking their password against the database.
func (s *Service) passwordAuthNMiddleware() fiber.Handler {
	const op errors.Op = "server.Service.passwordAuthNMiddleware"
	if s == nil {
		return serverErrorHandler()
	}
	return func(c *fiber.Ctx) error {
		// 1. Fetch unified request context from locals.
		reqCtx, err := getRequestContext(c)
		if err != nil {
			err = errors.New(op).Err(err)
			s.logger.ErrorWith().Err(err).Msg("getRequestContext failed")
			return c.Status(fiber.StatusInternalServerError).JSON(jsonInternalError)
		}

		// 2. Fetch the user by callsign.
		user, err := s.fetchUser(c.UserContext(), reqCtx.Request.Callsign)
		if err != nil {
			err = errors.New(op).Err(err)
			s.logger.ErrorWith().Err(err).Msg("s.fetchUser failed")
			return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
		}

		validPass, err := s.isValidPassword(user.PassHash, reqCtx.Request.Key)
		if err != nil {
			err = errors.New(op).Err(err)
			s.logger.ErrorWith().Err(err).Msg("s.isValidPassword failed")
			return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
		}

		if !validPass {
			s.logger.InfoWith().Str("callsign", reqCtx.Request.Callsign).Msg("Invalid password")
			return c.Status(fiber.StatusUnauthorized).JSON(jsonUnauthorized)
		}

		reqCtx.IsValid = validPass
		reqCtx.User = &user

		// The user's password is no longer needed after successful authn.
		// This prevents accidental leakage further down-stream.
		reqCtx.Request.Key = ""

		return c.Next()
	}
}
