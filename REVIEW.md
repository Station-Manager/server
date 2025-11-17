# Server Package Review

This document captures a structured review of the `server` package with focus on security, performance, and concurrency/thread-safety, based on the code in `server/main.go` and `server/server/*` as of this session.

## 1. Architecture Overview

**Key files:**

- `main.go` – entry point, sets default DB to PostgreSQL via `config.EnvSmDefaultDB`, wires signal handling, starts and shuts down the server.
- `server/service.go` – defines `Service` struct (container, DB service, logging service, server config, Fiber app, validator, logbook cache) and `NewService`, `Start`, `Shutdown`.
- `server/internal.go` – initialization helpers:
  - `initialize` – sets up DI container, Fiber app, and validator.
  - `initializeContainer` – registers `config`, `logging`, `database` services into `iocdi.Container`.
  - `initializeGoFiber` – creates `fiber.App` with `goccy/go-json` encoder/decoder and timeouts, wires middleware and routing.
  - `resolveAndSetDatabaseService`, `resolveAndSetLoggingService`, `resolveAndSetServerConfig` – DI helpers pulling typed services/config from the container.
- `server/middleware.go` – `basicChecks` middleware that parses the request, validates it, authenticates (password or API key), loads user/logbook, and stores `requestData` in `fiber.Ctx.Locals`.
- `server/handlers.go` – `postDispatcherHandler` which dispatches requests by `types.RequestAction` to specific actions (`insertQsoAction`, `registerLogbookAction`, etc.).
- `server/register_logbook.go` – transactional creation of a logbook and associated API key.
- `server/cache.go` – in-memory logbook cache with TTL and capacity limit, plus helper `fetchLogbookWithCache` on `Service`.
- `server/consts.go`, `server/errors.go` – constants for common error messages and locals keys.

**Request flow:**

1. `main.go` creates `Service` via `server.NewService()`.
2. `Service.Start()` opens DB, runs migrations, and calls `app.Listen`/`ListenTLS`.
3. `initializeGoFiber` sets up:
   - Global middleware: `s.basicChecks()`.
   - `POST /api/v1/` handled by `postDispatcherHandler()`.
4. `basicChecks`:
   - Parses `types.PostRequest` from JSON body via `c.BodyParser`.
   - Validates base fields (`Action`, `Key`, `Callsign`, `Data`) via `validatePostRequest`.
   - Validates `Action` via `isValidAction`.
   - If `Action == RegisterLogbookAction`: fetches user by callsign and validates password.
   - Else (e.g., `InsertQsoAction`): validates API key and loads logbook (with cache).
   - Stores a `requestData` struct in `c.Locals` (includes `IsValid`, `Action`, `Data`, `Logbook`).
5. `postDispatcherHandler` reads `requestData` from `Locals`, checks `IsValid`, then dispatches by `Action` to specific handler functions.

**Shutdown path:**

- `main.go` creates a signal-aware context and starts `Service.Start()` in a goroutine.
- On SIGINT/SIGTERM, it calls `svc.Shutdown()`, which:
  - Shuts down Fiber app with a 30-second timeout context.
  - Closes DB.
  - Closes logger.

## 2. Security Review

### 2.1 Request Parsing & Validation

**Observations:**

- `basicChecks` parses body into `types.PostRequest` using `c.BodyParser(&request)`.
- `validatePostRequest` ensures `Action`, `Key`, `Callsign`, and `Data` are non-empty.
- Logbook registration handler (`registerLogbookAction`) reads JSON from `requestData.Data` into `types.Logbook` using `goccy/go-json` `Unmarshal`, then validates it with `validator.v10` (`s.validate.Struct(logbook)`).

**Strengths:**

- Centralized validation of the outer envelope (`PostRequest`) for all POST requests.
- Use of `validator.v10` for structured validation of `types.Logbook` (and presumably other types in other handlers).
- Invalid inputs return `400 Bad Request` with generic error JSON (`jsonBadRequest`).

**Risks / Gaps:**

1. **Request body size / DoS risk**
   - `fiber.New` configuration in `initializeGoFiber` does not set `BodyLimit`.
   - Large request bodies could be accepted and parsed, potentially causing memory pressure.

   **Recommendation:** Add a reasonable `BodyLimit` (e.g., 1–2MB, or configuration-driven) in `fiber.Config` and ensure it is tied to `types.ServerConfig`.

2. **Raw `Data` field handling**
   - `types.PostRequest.Data` is a string that is parsed again (JSON) in `registerLogbookAction`.
   - There is no explicit bound on `len(request.Data)` or checks for overly large or deeply nested JSON.

   **Recommendation:**
   - Enforce a maximum length on `Data` (e.g., 64KB) before `json.Unmarshal`.
   - Consider reshaping `PostRequest` per action (e.g., `Logbook` subobject) to avoid double parsing and allow more direct validation.

3. **Validation completeness**
   - Security depends on validation tags in `types.Logbook` and other request structs.
   - Must ensure length constraints, allowed characters, and domain rules are consistently enforced.

   **Recommendation:**
   - Review `types.PostRequest`, `types.Logbook`, and other action payload types to ensure validation tags are present and strict.

### 2.2 Authentication & Authorization

**Authentication behavior in `basicChecks`:**

- For `RegisterLogbookAction`:
  - Fetches user with `fetchUser(ctx, request.Callsign)` which:
    - Uses `s.db.FetchUserByCallsignContext`.
    - Uses `adapters` to convert the DB model to `types.User`.
  - Validates password using `isValidPassword(user.PassHash, request.Key)` -> `apikey.VerifyPassword`.
  - Stores user in `c.Locals(localsUserDataKey, user)`.

- For other actions (e.g., `InsertQsoAction`):
  - Validates API key via `isValidApiKey(ctx, request.Key)`:
    - Checks non-empty key.
    - Splits prefix using `apikey.ParseApiKey`.
    - Fetches API key with `s.db.FetchAPIKeyByPrefixContext`.
    - Validates via `apikey.ValidateApiKey(fullKey, model.KeyHash)`.
    - Ensures `model.LogbookID != 0`.
  - Fetches logbook via `fetchLogbookWithCache(ctx, logbookId)`, which uses the cache and falls back to `s.db.FetchLogbookByIDContext`.

**Strengths:**

- Separation between password-based auth (for logbook registration) and API key-based auth (for other actions).
- API key validation is centrally implemented and relies on a dedicated `apikey` package (likely using secure hashing and constant-time comparisons).
- Errors during auth/logbook lookup are logged but return generic `401 Unauthorized` with `jsonUnauthorized`.

**Risks / Gaps:**

1. **Authorization granularity**
   - Middleware currently treats any valid API key as authorized for all non-registration actions.
   - There is no explicit role/permission model at the server layer; fine-grained authorization is not evident.

   **Recommendation:**
   - Introduce permission flags on API keys or logbooks (e.g., read-only, write, admin) and enforce them per `RequestAction`.

2. **User account state verification**
   - `fetchUser` includes a TODO to verify account status (e.g., email verification).

   **Recommendation:**
   - Extend `types.User` with fields such as `Verified` and `Disabled` and enforce them in the authentication path.

3. **Error messaging**
   - External HTTP responses are generic, which is good.
   - Need to ensure logs do not record sensitive credentials or full API keys.

   **Recommendation:**
   - Audit logging in `logging.Service` and handlers to ensure that `fullKey`, raw passwords, or other secrets are never logged.

### 2.3 Database Access and Injection Safety

- The `server` package uses higher-level methods of `database.Service` for all DB operations:
  - `FetchUserByCallsignContext`
  - `FetchAPIKeyByPrefixContext`
  - `InsertLogbookWithTxContext`
  - `InsertAPIKeyWithTxContext`
  - `FetchLogbookByIDContext`

**Risks:**

- Injection risk is primarily mitigated in the `database` package by using parameterized queries or an ORM (SQLBoiler). The `server` package itself never constructs raw SQL.

**Recommendation:**

- Spot-check implementations in `database` (e.g., `crud_logbook.go`, `crud_apikey.go`) to ensure that user inputs are always passed as parameters and not concatenated into SQL strings.

### 2.4 Error Handling and Logging

- Handlers wrap errors with `errors.New(op).Err(err)` for traceability and log them via `s.logger.ErrorWith()`.
- HTTP responses use generic JSON payloads:
  - `jsonBadRequest` for validation issues.
  - `jsonUnauthorized` for auth failures.
  - `jsonInternalError` for server errors.

**Strengths:**

- Internal error details (e.g., DB errors, stack context) are not exposed to clients.
- A consistent error wrapping strategy provides better logging and debugging.

**Considerations:**

- Ensure that log message contents don’t include sensitive fields.
- `main.go` uses `github.com/rs/zerolog/log` with `log.Printf`, which may not be idiomatic; consider using structured logging consistently.

### 2.5 TLS & Deployment

- `Service.Start()` chooses between `ListenTLS` and `Listen` based on `config.TLSEnabled` and TLS cert/key paths.

**Recommendation:**

- Confirm that in production configuration, either `TLSEnabled` is true or traffic is always terminated at a secure reverse proxy.
- Validate TLS configuration (paths, ciphers, etc.) via the `config` package.

## 3. Performance Review

### 3.1 Fiber and JSON Configuration

- `initializeGoFiber` uses:

  ```go
  s.app = fiber.New(fiber.Config{
      AppName:      s.config.Name,
      JSONDecoder:  json.Unmarshal,
      JSONEncoder:  json.Marshal,
      ReadTimeout:  time.Duration(s.config.ReadTimeout) * time.Second,
      WriteTimeout: time.Duration(s.config.WriteTimeout) * time.Second,
      IdleTimeout:  time.Duration(s.config.IdleTimeout) * time.Second,
  })
  ```

- It uses `goccy/go-json` which is generally faster than the standard library.

**Strengths:**

- Timeouts are configurable and help prevent slow-client DoS.
- Fast JSON encoder/decoder is configured.

**Potential Improvements:**

- Add `BodyLimit` to control max request body size.
- Consider other `fiber.Config` settings if high throughput is required (e.g., `Prefork`, `Concurrency`) depending on deployment constraints.

### 3.2 Middleware and Hot Path

- `basicChecks` runs on every request:
  - Body parsing and envelope validation.
  - Action validation and authentication.
  - User/logbook lookups (DB + cache).

**Observations:**

- API key validation always hits the DB for the prefix record, then uses cache only for the logbook.
- Use of `inMemoryLogbookCache` likely improves performance for repeated access to the same logbook.

**Potential Optimizations:**

- Introduce a short-lived cache for API key prefix→logbook ID if profiling shows DB lookups are a bottleneck.
- Avoid double parsing of JSON payloads by designing request structures that embed typed objects rather than opaque `Data` strings.

### 3.3 DB Lifecycle and Migrations

- `Service.Start()` calls `s.db.Open()` and `s.db.Migrate()` before starting the HTTP server.

**Considerations:**

- Migrations at startup are typically fine but can be slow; ensure they are idempotent and fast.
- Connection pool settings (e.g., `MaxOpenConns`, `MaxIdleConns`, and timeouts) are configured in `types.DatastoreConfig` and should be tuned per environment.

### 3.4 Benchmarks (Current Status)

- No benchmarks were observed in `server/server` at the time of review.

**Recommendation:**

- Add benchmarks for:
  - `registerLogbookAction` and `insertQsoAction` using `httptest`.
  - Cache operations (`Get`, `Set`, `Invalidate`) under concurrent load.
  - JSON parsing/validation of representative payloads.

## 4. Concurrency / Thread-Safety Review

### 4.1 Lifecycle and Goroutines

- `main.go` starts the server in a goroutine:

  ```go
  errChan := make(chan error, 1)
  go func() { errChan <- svc.Start() }()
  ```

- Uses `signal.NotifyContext` to cancel on SIGINT/SIGTERM.
- `svc.Shutdown()` stops the Fiber app, then closes DB and logger in order.

**Strengths:**

- Graceful shutdown with timeouts.
- Error reporting via a buffered channel avoids deadlocks.

### 4.2 Shared State in `Service`

- Shared fields:
  - `db *database.Service`
  - `logger *logging.Service`
  - `config types.ServerConfig`
  - `app *fiber.App`
  - `validate *validator.Validate`
  - `logbookCache logbookCache`

**Thread-safety:**

- `database.Service` and `logging.Service` are assumed thread-safe (as typical for DB and logging services); confirm via their implementations.
- `types.ServerConfig` is read-only after initialization.
- `validator.Validate` is safe for concurrent use after configuration (no runtime mutation of validation rules).
- `logbookCache` is an interface with `inMemoryLogbookCache` implementation that uses `sync.RWMutex` to protect its map.

### 4.3 In-Memory Logbook Cache

- Implementation details from `cache.go`:

  ```go
  type inMemoryLogbookCache struct {
      mu         sync.RWMutex
      entries    map[int64]logbookCacheEntry
      maxEntries int
  }
  ```

- `Get` acquires `RLock`, reads the entry, then unlocks; if the entry is expired, it calls `Invalidate` which acquires `Lock` and deletes the entry.
- `Set` acquires `Lock`, ensures `entries` is initialized, and if `len(entries) >= maxEntries`, evicts one arbitrary entry.

**Assessment:**

- Implementation is race-free and safe for concurrent access.
- Lazy expiration plus simple eviction strategy keeps complexity low.

**Considerations:**

- For high cardinality or very high QPS, the simple eviction policy and single map might be a bottleneck; sharding or more advanced caching may be needed later.

### 4.4 Transaction Handling and Test Failure

- There is a unit test in `register_logbook_test.go`:

  ```go
  func TestRegisterLogbookRollbackOnApiKeyFailure(t *testing.T) {
      // ...
      // NOTE: For SQLite migrations currently do not create api_keys table,
      // so API key insertion will fail, triggering rollback of the inserted logbook.
  }
  ```

- The test builds a SQLite-backed `database.Service`, initializes it, then:
  - Calls `registerLogbookAction` via Fiber, which:
    - Begins a transaction.
    - Inserts a logbook.
    - Attempts to insert an API key (expected to fail because the `api_keys` table is missing in SQLite migrations).
    - Calls `tx.Rollback()` on failure.
  - Then directly calls `dbSvc.InsertLogbookContext` to insert the same logbook name again, expecting it to succeed if the rollback was effective.

**Actual behavior observed during review:**

- The test currently fails with:

  > expected rollback to remove original row; second insert failed: Internal system error.

- This indicates that the first logbook insert is still committed despite the rollback, implying a problem in the transaction implementation in the `database` package.

**Assessment:**

- From the `server` side, transaction usage in `registerLogbookAction` is correct:
  - `BeginTxContext`
  - insert logbook
  - insert API key
  - `Rollback()` on any error
  - `Commit()` only once at the end if all operations succeed.

- The failure implies that `BeginTxContext` / `InsertLogbookWithTxContext` / `InsertAPIKeyWithTxContext` in `database` may not be correctly tying all operations to the same transaction, or rollback is not properly wired.

**Recommendation (database layer):**

- Inspect and fix the transaction handling in `database` so that:
  - The `*sql.Tx` from `BeginTxContext` is consistently used for all writes.
  - Rollback properly reverts any inserts.
  - The test `TestRegisterLogbookRollbackOnApiKeyFailure` passes.

### 4.5 Race Detection

- Running `go test ./...` under `server` currently fails because of the rollback test, not because of data races.
- A follow-up after fixing DB transaction behavior should be running:

  ```bash
  cd server
  go test -race ./...
  ```

  to confirm the absence of data races.

## 5. Recommendations Summary

### Security

- Add `BodyLimit` to the Fiber configuration to mitigate large request body DoS.
- Bound `Data` field size and validate nested payloads strictly.
- Implement user state checks (e.g., verified/disabled) in `fetchUser` or authentication path.
- Introduce finer-grained authorization semantics per API key or per action.
- Audit logging to ensure secrets (passwords, API keys) are never logged.

### Performance

- Consider caching API key prefix→logbook mappings for high-traffic deployments.
- Make logbook cache TTL and `maxEntries` configurable via `types.ServerConfig`.
- Avoid double JSON parsing by designing request payloads around typed subobjects instead of opaque `Data` strings for each action.
- Add benchmarks for critical handlers and cache operations.

### Concurrency / Correctness

- Transaction usage in `registerLogbookAction` is correct from the server perspective; fix the underlying `database` transaction implementation so rollback semantics work as expected.
- After fixing, run tests with `-race` to validate thread-safety.
- Monitor cache contention and, if needed, evolve to more scalable caching structures.

---

This `REVIEW.md` reflects an in-depth review of the current `server` implementation, focusing on security, performance, and concurrency/thread-safety considerations, and outlines concrete recommendations for improvements and follow-up work.

## 6. Findings by Severity

This section classifies the most important findings by severity to guide remediation priorities.

### Critical

- **Broken transactional rollback for logbook + API key creation (SQLite)**  
  - *Description*: `TestRegisterLogbookRollbackOnApiKeyFailure` currently fails. Despite calling `tx.Rollback()` when API key insertion fails, the original logbook row appears to remain in the database, causing a unique-constraint failure on a subsequent insert of the same name.  
  - *Impact*: Risk of partially committed state (logbook without corresponding API key) if the same pattern exists in production DBs. This can lead to inconsistent system state and surprises for clients.  
  - *Location*: `server/server/register_logbook.go` (usage) and `database` package (transaction implementation, e.g., `BeginTxContext`, `InsertLogbookWithTxContext`, `InsertAPIKeyWithTxContext`).

### High

- **Request body size / DoS risk**  
  - *Description*: `fiber.Config` does not set `BodyLimit`, so extremely large request bodies can be accepted and parsed.  
  - *Impact*: Potential memory exhaustion or degraded performance under malicious or misbehaving clients.  
  - *Location*: `server/server/internal.go` → `initializeGoFiber`.

- **Lack of user account state checks during authentication**  
  - *Description*: `fetchUser` includes a TODO to verify whether the user account is verified/active, but there is no implemented check.  
  - *Impact*: Unverified or disabled accounts could potentially perform sensitive actions such as registering logbooks.  
  - *Location*: `server/server/middleware.go` → `fetchUser`.

- **Coarse-grained authorization for API keys**  
  - *Description*: Any valid API key appears to authorize all non-registration actions, without a permission model differentiating read/write/admin or specific actions.  
  - *Impact*: Over-privileged API keys increase blast radius if compromised and limit ability to restrict actions appropriately.  
  - *Location*: `server/server/middleware.go` (`basicChecks` and `isValidAction` interaction) and downstream handlers.

### Medium

- **Unbounded `Data` field contents**  
  - *Description*: `types.PostRequest.Data` is a string that can contain arbitrarily large JSON, which is parsed again with `json.Unmarshal` per handler (e.g., `registerLogbookAction`). There are no explicit size or complexity checks.  
  - *Impact*: Potential CPU and memory overhead on large or deeply nested payloads; makes DoS easier and complicates validation.  
  - *Location*: `server/server/middleware.go` (`basicChecks`) and `server/server/register_logbook.go`.

- **Lack of benchmarks for hot paths**  
  - *Description*: There are no benchmarks for critical request paths or cache operations.  
  - *Impact*: Harder to measure performance regressions or improvements and to justify design trade-offs.  
  - *Location*: `server/server` (missing tests/benchmarks).

- **API key validation always hits DB (no key-level cache)**  
  - *Description*: API key validation uses DB lookup for every request; only logbook retrieval is cached.  
  - *Impact*: Under high load with repeated use of the same API key, DB may become a bottleneck.  
  - *Location*: `server/server/internal.go` → `isValidApiKey`, `server/server/middleware.go` → `basicChecks`.

### Low

- **Double JSON parsing of payloads**  
  - *Description*: Request envelope is parsed once into `types.PostRequest`, then `Data` is parsed again into action-specific types (e.g., `types.Logbook`).  
  - *Impact*: Some allocation/CPU overhead; not critical but can add up under high load.  
  - *Location*: `server/server/middleware.go`, `server/server/register_logbook.go`, and other action handlers.

- **Non-idiomatic logging use in `main.go`**  
  - *Description*: Use of `github.com/rs/zerolog/log` with `log.Printf` is non-standard; ideally, use structured logging consistently.  
  - *Impact*: Minor: consistency and observability aesthetics rather than correctness or safety.  
  - *Location*: `server/main.go`.

## 7. Prioritized Remediation Checklist

This checklist is ordered roughly by impact and dependency. Each item can be tracked as a separate issue or task.

### Phase 1 – Correctness & Safety (Critical / High)

1. **Fix transactional rollback semantics in `database` package**  
   - [ ] Inspect `database.Service.BeginTxContext` and `InsertLogbookWithTxContext` / `InsertAPIKeyWithTxContext` to ensure all writes use the same `*sql.Tx` and that rollback reverts partial writes.  
   - [ ] Adjust implementation as needed (e.g., avoid using the root `*sql.DB` inside `Insert*WithTxContext`).  
   - [ ] Re-run `TestRegisterLogbookRollbackOnApiKeyFailure` in `server/server` until it passes.  
   - [ ] Add a similar transaction test at the `database` level if one doesn’t exist.

2. **Introduce request body size limits**  
   - [ ] Add a `BodyLimit` field (or equivalent) to `types.ServerConfig` and load it from config.  
   - [ ] Configure `fiber.New` in `initializeGoFiber` to use this limit.  
   - [ ] Add tests to ensure requests exceeding the limit are rejected with a clear error (e.g., `413 Payload Too Large` or `400 Bad Request`).

3. **Implement user account state checks**  
   - [ ] Extend `types.User` with fields representing verification and enabled/disabled status (if not already present).  
   - [ ] Update `fetchUser` and/or `basicChecks` to reject users who are unverified or disabled, returning `401`/`403` as appropriate.  
   - [ ] Add tests that cover disabled/unverified user scenarios for `RegisterLogbookAction`.

4. **Design and enforce finer-grained authorization**  
   - [ ] Define a simple permission model (e.g., per-API-key flags such as `CanInsertQso`, `CanRegisterLogbook`, `IsAdmin`).  
   - [ ] Persist these flags in the `database` package (tables/migrations) and expose them via the relevant fetch methods.  
   - [ ] Update `basicChecks` and/or handlers to enforce permissions per `RequestAction`.  
   - [ ] Add tests that ensure unauthorized actions are denied even when API keys are otherwise valid.

5. **Audit logging for sensitive information**  
   - [ ] Review `logging.Service` configuration and all `s.logger.*` usages in `server/server`.  
   - [ ] Ensure no logs include plaintext passwords, full API keys (`fullKey`), or other secrets.  
   - [ ] Add tests or code comments to explicitly forbid logging secrets (e.g., lint rules or helper functions that scrub values).

### Phase 2 – Robustness & Performance (Medium)

6. **Bound and validate `Data` field size and structure**  
   - [ ] Decide on a maximum length for `PostRequest.Data` (e.g., 64KB) and enforce it in `basicChecks`.  
   - [ ] For actions like `registerLogbookAction`, validate that JSON structures are not excessively deep or complex (where practical).  
   - [ ] Add tests with large payloads to ensure they are handled safely and predictably.

7. **Introduce API key-level caching (optional but beneficial)**  
   - [ ] Design a small cache for API key prefix→logbook ID and key metadata, with short TTL to avoid staleness.  
   - [ ] Implement the cache (thread-safe map or reuse the existing cache pattern) and integrate it into `isValidApiKey`.  
   - [ ] Add benchmarks and tests to verify behavior under concurrent access and high load.

8. **Make logbook cache configurable**  
   - [ ] Add logbook cache TTL and `maxEntries` to `types.ServerConfig` (or a nested cache-specific config).  
   - [ ] Wire these values into `newInMemoryLogbookCache` or a factory function.  
   - [ ] Add tests to confirm that configuration values are honored and that eviction behaves as expected.

9. **Add benchmarks for critical paths**  
   - [ ] Add microbenchmarks for `registerLogbookAction` and `insertQsoAction` using `httptest` and a lightweight DB setup (e.g., SQLite).  
   - [ ] Add microbenchmarks for `inMemoryLogbookCache.Get` and `.Set` under varying contention scenarios.  
   - [ ] Use `go test -bench` and `-benchmem` to capture baseline metrics and track future changes.

### Phase 3 – Cleanup & Quality (Low / Ongoing)

10. **Reduce double JSON parsing where possible**  
    - [ ] Explore refactoring `types.PostRequest` to include typed fields for each action (e.g., `Logbook` for `RegisterLogbookAction`, `QSO` for `InsertQsoAction`) instead of a raw `Data` string.  
    - [ ] Update handlers and validation logic accordingly.  
    - [ ] Measure performance impact using benchmarks.

11. **Standardize logging in `main.go` and elsewhere**  
    - [ ] Replace the direct `log.Printf` usage with structured logging through your `logging.Service` or idiomatic `zerolog` usage.  
    - [ ] Ensure startup/shutdown errors are logged consistently with the rest of the system.

12. **Static analysis and race detection as a routine**  
    - [ ] Integrate `go vet`, `golangci-lint`, and `go test -race` into CI for the `server` module and its dependencies.  
    - [ ] Fix any reported issues, especially those from `gosec`, `staticcheck`, and race detection.

---

This extended review now includes both a severity-based summary of issues and a prioritized remediation checklist to guide incremental improvements to the `server` package.
