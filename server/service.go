package server

import (
	"github.com/Station-Manager/database"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/iocdi"
	"github.com/gofiber/fiber/v2"
)

type Service struct {
	container *iocdi.Container
	db        *database.Service
	app       *fiber.App
}

func NewService() (*Service, error) {
	const op errors.Op = "server.NewService"
	svc := &Service{}

	if err := svc.initialize(); err != nil {
		return nil, errors.New(op).Err(err)
	}

	obj, err := svc.container.ResolveSafe(database.ServiceName)
	if err != nil {
		return nil, errors.New(op).Err(err).Msg("Failed to resolve database service")
	}
	dbSvc, ok := obj.(*database.Service)
	if !ok {
		return nil, errors.New(op).Msg("Failed to cast database service")
	}

	svc.db = dbSvc

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
		return errors.New(op).Err(err).Msg("Failed to open database")
	}

	return s.app.Shutdown()
}
