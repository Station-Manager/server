package service

import (
	"github.com/Station-Manager/config"
	"github.com/Station-Manager/database"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/iocdi"
	"github.com/Station-Manager/logging"
	"github.com/Station-Manager/server/service/frontend"
	"github.com/Station-Manager/types"
	"github.com/Station-Manager/utils"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"reflect"
	"time"
)

// initializeContainer sets up the dependency injection container with required service registrations and configurations.
// It registers key service instances and ensures the container is built successfully.
// Returns an error if the initialization process encounters any issues.
func (s *Service) initializeContainer() error {
	const op errors.Op = "server.Service.initializeContainer"

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

// initializeService sets up and initializes the service dependencies including database, logger, configuration, and validators.
// It ensures that essential services are resolved and configured properly before the service becomes operational.
// Returns an error if any component fails during initialization.
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

// initializeGoFiber initializes the Fiber application with configurations defined in the Service struct.
// Returns an error if the Service instance is nil or configuration fails.
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

	s.app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "*",
		AllowMethods: "GET,POST",
	}))

	s.initializeRoutes()

	return nil
}

// initializeRoutes configures API route groups and handlers for the service with associated middleware.
func (s *Service) initializeRoutes() {
	s.app.Get("/", filesystem.New(filesystem.Config{
		Root:         frontend.FileSystem(),
		Index:        "index.html",
		NotFoundFile: "index.html",
	}))

	// Health check endpoint - lightweight liveness/readiness probe
	s.app.Get("/health", s.healthHandler)

	// The base API group with common middleware applied to all routes.
	api := s.app.Group("/api", s.requestContextMiddleware())

	// The logbook routes require password authentication as a minimum because
	// API keys are per-logbook and not shared across users.
	logbookRoutes := api.Group("/logbook", s.passwordAuthNMiddleware())
	logbookRoutes.Post("/register", s.registerLogbookHandler)

	// The QSO routes require an API key authentication.
	qsoRoutes := api.Group("/qso", s.apikeyAuthNMiddleware())
	qsoRoutes.Post("/insert", s.insertQsoHandler)
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
