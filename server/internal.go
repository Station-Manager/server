package server

import (
	"github.com/Station-Manager/config"
	"github.com/Station-Manager/database"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/iocdi"
	"github.com/Station-Manager/logging"
	"github.com/Station-Manager/utils"
	"github.com/gofiber/fiber/v2"
	"reflect"
)

func (s *Service) initialize() error {
	const op errors.Op = "server.Service.initialize"
	if s == nil {
		return errors.New(op).Msg(errMsgNilService)
	}
	if err := s.initializeContainer(); err != nil {
		return errors.New(op).Err(err).Msg("Failed to initialize container")
	}

	if err := s.initializeGoFiber(); err != nil {
		return errors.New(op).Err(err).Msg("Failed to initialize goFiber")
	}

	return nil
}

func (s *Service) initializeContainer() error {
	const op errors.Op = "server.Service.initializeContainer"
	if s == nil {
		return errors.New(op).Msg(errMsgNilService)
	}

	s.container = iocdi.New()

	workingDir, err := utils.WorkingDir(".")
	if err != nil {
		return errors.New(op).Err(err)
	}

	if err = s.container.RegisterInstance("workingdir", workingDir); err != nil {
		return errors.New(op).Err(err)
	}
	if err = s.container.Register(config.ServiceName, reflect.TypeOf((*config.Service)(nil))); err != nil {
		return errors.New(op).Err(err)
	}
	if err = s.container.Register(logging.ServiceName, reflect.TypeOf((*logging.Service)(nil))); err != nil {
		return errors.New(op).Err(err)
	}
	if err = s.container.Register(database.ServiceName, reflect.TypeOf((*database.Service)(nil))); err != nil {
		return errors.New(op).Err(err)
	}
	if err = s.container.Build(); err != nil {
		return errors.New(op).Err(err).Msg(err.Error())
	}
	return nil
}

func (s *Service) initializeGoFiber() error {
	const op errors.Op = "server.Service.initializeGoFiber"
	if s == nil {
		return errors.New(op).Msg(errMsgNilService)
	}

	s.app = fiber.New()
	apiRoutes := s.app.Group("/api")
	versionOneRoutes := apiRoutes.Group("/v1")
	versionOneRoutes.Post("/register", s.addLogbookHandler())

	return nil
}

func (s *Service) resolveAndSetDatabaseService() (*database.Service, error) {
	const op errors.Op = "server.Service.resolveDatabase"
	if s == nil {
		return nil, errors.New(op).Msg(errMsgNilService)
	}

	obj, err := s.container.ResolveSafe(database.ServiceName)
	if err != nil {
		return nil, errors.New(op).Err(err).Msg("Failed to resolve database service")
	}
	dbSvc, ok := obj.(*database.Service)
	if !ok {
		return nil, errors.New(op).Msg("Failed to cast database service")
	}

	return dbSvc, nil
}

func (s *Service) resolveAndSetLoggingService() (*logging.Service, error) {
	const op errors.Op = "server.Service.resolveLogging"
	if s == nil {
		return nil, errors.New(op).Msg(errMsgNilService)
	}

	obj, err := s.container.ResolveSafe(logging.ServiceName)
	if err != nil {
		return nil, errors.New(op).Err(err).Msg("Failed to resolve logging service")
	}
	logSvc, ok := obj.(*logging.Service)
	if !ok {
		return nil, errors.New(op).Msg("Failed to cast logging service")
	}

	return logSvc, nil
}
