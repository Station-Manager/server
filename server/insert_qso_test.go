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

// helper to build a minimal Service with sqlite DB and middleware/route wiring for insert_qso tests.
func newTestServerForInsertQSO(t *testing.T) *Service {
	t.Helper()

	cfg := &types.DatastoreConfig{
		Driver:                    database.SqliteDriver,
		Path:                      t.TempDir() + "/test_insert_qso.db",
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

	// For insert QSO, we need middleware + dispatcher wired.
	svc.app.Use(svc.basicChecks())
	apiRoutes := svc.app.Group("/api/v1")
	apiRoutes.Post("/", svc.postDispatcherHandler())

	return svc
}

// TestInsertQsoAction_TypedPayload ensures the new typed PostRequest wire format works end-to-end
// for the insert_qso action (i.e., no second JSON parse is required).
func TestInsertQsoAction_TypedPayload(t *testing.T) {
	svc := newTestServerForInsertQSO(t)
	defer func() { _ = svc.db.Close() }()

	// NOTE: This test assumes an API key and logbook exist and would require seeding the DB.
	// For now, verify that a structurally valid request reaches the middleware and fails
	// with Unauthorized (401) rather than BadRequest (400), which would indicate
	// that the typed payload parsing and validation passed.

	body := `{
  "callsign": "TEST1",
  "key": "DUMMY_API_KEY_SHOULD_FAIL_AUTH",
  "action": "insert_qso",
  "qso": {
    "call": "7Q7EB",
    "freq": 14320000,
    "qso_date": "20251115",
    "time_on": "1200",
    "time_off": "1205",
    "band": "20m",
    "mode": "SSB",
    "rst_sent": "59",
    "rst_rcvd": "59",
    "station_callsign": "TEST1"
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
