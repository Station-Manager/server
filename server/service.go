package server

import (
	"github.com/Station-Manager/database"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/iocdi"
	"github.com/Station-Manager/logging"
	"github.com/gofiber/fiber/v2"
)

type Service struct {
	container *iocdi.Container
	db        *database.Service
	logger    *logging.Service
	app       *fiber.App
}

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

	return svc, nil
}

func (s *Service) Start() error {
	const op errors.Op = "server.Service.Start"
	if s == nil {
		return errors.New(op).Msg(errMsgNilService)
	}

	if err := s.db.Open(); err != nil {
		return errors.New(op).Err(err).Msg("Failed to open database")
	}

	if err := s.db.Migrate(); err != nil {
		return errors.New(op).Err(err).Msg("Failed to migrate database")
	}

	return s.app.Listen(":3000")
}

func (s *Service) Shutdown() error {
	const op errors.Op = "server.Service.Shutdown"
	if s == nil {
		return errors.New(op).Msg(errMsgNilService)
	}

	if err := s.db.Close(); err != nil {
		s.logger.ErrorWith().Err(err).Msg("Failed to close database")
	}

	if err := s.logger.Close(); err != nil {
		// What to do here?
	}

	return s.app.Shutdown()
}
