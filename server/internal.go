package server

import (
	"github.com/Station-Manager/config"
	"github.com/Station-Manager/database"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/iocdi"
	"github.com/Station-Manager/logging"
	"github.com/Station-Manager/types"
	"github.com/Station-Manager/utils"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"reflect"
	"time"
)

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

func (s *Service) initializeService() error {
	const op errors.Op = "server.Service.initializeService"
	if s == nil {
		return errors.New(op).Msg(errMsgNilService)
	}

	var err error
	if s.db, err = s.resolveAndSetDatabaseService(); err != nil {
		return errors.New(op).Err(err)
	}

	if s.logger, err = s.resolveAndSetLoggingService(); err != nil {
		return errors.New(op).Err(err)
	}

	if s.config, err = s.resolveAndSetServerConfig(); err != nil {
		return errors.New(op).Err(err)
	}

	s.validate = validator.New(validator.WithRequiredStructEnabled())

	// Initialize the in-memory logbook cache with default settings.
	s.logbookCache = newInMemoryLogbookCache()

	return nil
}

func (s *Service) initializeGoFiber() error {
	const op errors.Op = "server.Service.initializeGoFiber"
	if s == nil {
		return errors.New(op).Msg(errMsgNilService)
	}

	s.app = fiber.New(fiber.Config{
		AppName:      s.config.Name,
		JSONDecoder:  json.Unmarshal,
		JSONEncoder:  json.Marshal,
		ReadTimeout:  time.Duration(s.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(s.config.IdleTimeout) * time.Second,
		BodyLimit:    s.config.BodyLimit,
	})

	// Our middleware for basic/common request checking
	s.app.Use(s.basicChecks())

	// Our base route
	apiRoutes := s.app.Group("/api/v1")

	// Every request goes to the dispatcherHandler.
	apiRoutes.Post("/", s.postDispatcherHandler())

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

func (s *Service) resolveAndSetServerConfig() (types.ServerConfig, error) {
	const op errors.Op = "server.Service.resolveConfig"
	emptyRetVal := types.ServerConfig{}
	if s == nil {
		return emptyRetVal, errors.New(op).Msg(errMsgNilService)
	}
	obj, err := s.container.ResolveSafe(config.ServiceName)
	if err != nil {
		return emptyRetVal, errors.New(op).Err(err).Msg("Failed to resolve config service")
	}
	cfgSvc, ok := obj.(*config.Service)
	if !ok {
		return emptyRetVal, errors.New(op).Msg("Failed to cast config service")
	}

	svrCfg, err := cfgSvc.ServerConfig()
	if err != nil {
		return emptyRetVal, errors.New(op).Err(err).Msg("Failed to get server config")
	}

	//TODO: Config validation

	return svrCfg, nil
}
