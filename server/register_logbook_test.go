package server

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Station-Manager/config"
	"github.com/Station-Manager/database"
	"github.com/Station-Manager/logging"
	"github.com/Station-Manager/types"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"strings"
)

// newTestDatabaseService creates a sqlite-backed database service suitable for tests.
func newTestDatabaseService(t *testing.T) *database.Service {
	cfg := &types.DatastoreConfig{
		Driver:                    database.SqliteDriver,
		Path:                      t.TempDir() + "/test.db",
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
	return dbSvc
}

// TestRegisterLogbookRollbackOnApiKeyFailure ensures rollback semantics.
// NOTE: For SQLite migrations currently do not create api_keys table, so API key insertion
// will fail, triggering rollback of the inserted logbook.
func TestRegisterLogbookRollbackOnApiKeyFailure(t *testing.T) {
	dbSvc := newTestDatabaseService(t)
	defer func() { _ = dbSvc.Close() }()

	// Build a minimal server Service manually (bypassing container initialization).
	svc := &Service{
		db:       dbSvc,
		logger:   dbSvc.Logger,
		app:      fiber.New(),
		validate: validator.New(),
	}

	// Choose a unique logbook name to check presence after rollback.
	logbookName := "rollback_logbook_test"
	logbookJSON := `{"name":"` + logbookName + `","callsign":"TEST1","description":"rollback scenario"}`

	// Route that primes locals and invokes the action directly.
	svc.app.Post("/register", func(c *fiber.Ctx) error {
		c.Locals(localsRequestDataKey, requestData{Data: logbookJSON})
		c.Locals(localsUserDataKey, types.User{ID: 1})
		return svc.registerLogbookAction(c)
	})

	req := httptest.NewRequest("POST", "/register", nil)
	resp, err := svc.app.Test(req)
	if err != nil {
		t.Fatalf("fiber test request failed: %v", err)
	}

	// Expect internal server error due to API key insertion failure triggering rollback.
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("expected status %d got %d", fiber.StatusInternalServerError, resp.StatusCode)
	}

	// Now attempt to insert the same logbook name outside of the transaction directly via the DB API.
	// If the previous partial insert had committed, this should violate UNIQUE(name) and fail.
	second, secondErr := dbSvc.InsertLogbookContext(context.Background(), types.Logbook{Name: logbookName, Callsign: "TEST1", Description: "after rollback"})
	if secondErr != nil {
		// Failure indicates the original insert remained (rollback failed).
		t.Fatalf("expected rollback to remove original row; second insert failed: %v", secondErr)
	}
	if second.ID == 0 {
		t.Fatalf("second insert returned zero ID; unexpected")
	}
}

// Sanity test: ensure handler returns error when context is nil.
func TestRegisterLogbookNilContext(t *testing.T) {
	svc := &Service{}
	err := svc.registerLogbookAction(nil)
	if err == nil || !strings.Contains(err.Error(), errMsgNilContext) {
		t.Fatalf("expected error containing %q; got %v", errMsgNilContext, err)
	}
}
