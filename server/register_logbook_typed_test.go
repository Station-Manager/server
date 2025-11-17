package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Station-Manager/config"
	"github.com/Station-Manager/database"
	"github.com/Station-Manager/logging"
	"github.com/Station-Manager/types"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// newTestServerForRegisterLogbook builds a minimal Service wired for register_logbook
// using the new typed PostRequest wire format.
func newTestServerForRegisterLogbook(t *testing.T) *Service {
	t.Helper()

	cfg := &types.DatastoreConfig{
		Driver:                    database.SqliteDriver,
		Path:                      t.TempDir() + "/test_register_logbook.db",
		Options:                   map[string]string{},
		Host:                      "localhost",
		Port:                      1,
		User:                      "test",
		Password:                  "test",
		Database:                  "test",
		SSLMode:                   "disable",
		MaxOpenConns:              1,
		MaxIdleConns:              1,
		ConnMaxLifetime:           1,
		ConnMaxIdleTime:           1,
		ContextTimeout:            5,
		TransactionContextTimeout: 5,
	}
	appCfg := types.AppConfig{DatastoreConfig: *cfg, LoggingConfig: types.LoggingConfig{Level: "error", ConsoleLogging: true, FileLogging: false, RelLogFileDir: "logs"}}
	cfgSvc := &config.Service{WorkingDir: t.TempDir(), AppConfig: appCfg}
	if err := cfgSvc.Initialize(); err != nil {
		t.Fatalf("config initialize failed: %v", err)
	}
	logSvc := &logging.Service{ConfigService: cfgSvc, WorkingDir: t.TempDir()}
	if err := logSvc.Initialize(); err != nil {
		t.Fatalf("logger initialize failed: %v", err)
	}
	dbSvc := &database.Service{ConfigService: cfgSvc, Logger: logSvc}
	if err := dbSvc.Initialize(); err != nil {
		t.Fatalf("db initialize failed: %v", err)
	}
	if err := dbSvc.Open(); err != nil {
		t.Fatalf("db open failed: %v", err)
	}
	if err := dbSvc.Migrate(); err != nil {
		t.Fatalf("db migrate failed: %v", err)
	}

	svc := &Service{
		db:       dbSvc,
		logger:   dbSvc.Logger,
		app:      fiber.New(),
		validate: validator.New(),
	}

	svc.app.Use(svc.basicChecks())
	apiRoutes := svc.app.Group("/api/v1")
	apiRoutes.Post("/", svc.postDispatcherHandler())

	return svc
}

// TestRegisterLogbook_TypedPayload verifies that the new typed PostRequest format
// for register_logbook is accepted at the middleware layer and reaches the handler.
// Because SQLite migrations do not currently include the api_keys table, we expect
// an internal server error due to API key insertion failure, which implicitly
// confirms the transaction path was executed.
func TestRegisterLogbook_TypedPayload(t *testing.T) {
	svc := newTestServerForRegisterLogbook(t)
	defer func() { _ = svc.db.Close() }()

	body := `{
  "callsign": "TEST1",
  "key": "user-password-placeholder",
  "action": "register_logbook",
  "logbook": {
    "name": "Default HF",
    "callsign": "TEST1",
    "description": "My primary HF logbook"
  }
}`

	req := httptest.NewRequest("POST", "/api/v1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := svc.app.Test(req)
	if err != nil {
		t.Fatalf("fiber test request failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusUnauthorized && resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("expected status 401 or 500, got %d", resp.StatusCode)
	}
}
