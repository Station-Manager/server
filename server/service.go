package server

import (
	"context"
	"fmt"
	"github.com/Station-Manager/database"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/iocdi"
	"github.com/Station-Manager/logging"
	"github.com/Station-Manager/types"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"time"
)

type requestData struct {
	IsValid   bool
	Action    types.RequestAction
	Data      string
	LogbookID int64
}

type Service struct {
	container *iocdi.Container
	db        *database.Service
	logger    *logging.Service
	config    types.ServerConfig
	app       *fiber.App
	validate  *validator.Validate
}

// NewService creates a new server instance and initializes all its dependencies.
func NewService() (*Service, error) {
	const op errors.Op = "server.NewService"
	svc := &Service{}

	var err error
	if err = svc.initialize(); err != nil {
		return nil, errors.New(op).Err(err)
	}

	if svc.db, err = svc.resolveAndSetDatabaseService(); err != nil {
		return nil, errors.New(op).Err(err)
	}

	if svc.logger, err = svc.resolveAndSetLoggingService(); err != nil {
		return nil, errors.New(op).Err(err)
	}

	if svc.config, err = svc.resolveAndSetServerConfig(); err != nil {
		return nil, errors.New(op).Err(err)
	}

	return svc, nil
}

// Start starts the server.
func (s *Service) Start() error {
	const op errors.Op = "server.Service.Start"
	if s == nil {
		return errors.New(op).Msg(errMsgNilService)
	}

	if err := s.db.Open(); err != nil {
		s.logger.ErrorWith().Err(err).Msg("Failed to open database")
		return errors.New(op).Err(err).Msg("s.db.Open")
	}

	if err := s.db.Migrate(); err != nil {
		return errors.New(op).Err(err).Msg("Failed to migrate database")
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	if s.config.TLSEnabled {
		return s.app.ListenTLS(addr, s.config.TLSCertFile, s.config.TLSKeyFile)
	} else {
		return s.app.Listen(addr)
	}
}

// Shutdown gracefully terminates the service by shutting down the server, closing database connections, and the logger.
func (s *Service) Shutdown() error {
	const op errors.Op = "server.Service.Shutdown"
	if s == nil {
		return errors.New(op).Msg(errMsgNilService)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown Fiber app first to stop accepting new requests
	if err := s.app.ShutdownWithContext(ctx); err != nil {
		s.logger.ErrorWith().Err(err).Msg("Failed to shutdown Fiber app")
		return errors.New(op).Err(err).Msg("s.app.Shutdown")
	}

	// Close the database after all requests are done
	if err := s.db.Close(); err != nil {
		s.logger.ErrorWith().Err(err).Msg("Failed to close database")
		return errors.New(op).Err(err).Msg("s.db.Close")
	}

	// Close logger last
	if err := s.logger.Close(); err != nil {
		return errors.New(op).Err(err).Msg("s.logger.Close")
	}

	return nil
}
