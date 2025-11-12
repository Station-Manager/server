package main

//type AppServer struct {
//	CfgService   *config.Service
//	DBService    *database.Service
//	App          *fiber.App
//	oneTimeStore sync.Map // key_name -> fullKey (only present until first GET retrieval)
//}
//
//// NewServer creates a new server instance.
//func NewServer(cfg *config.Service, db *database.Service) *AppServer {
//	return &AppServer{CfgService: cfg, DBService: db}
//}
//
//// Init sets up config, database, migrations and Fiber routes.
//func (s *AppServer) Init() error {
//	const op errors.Op = "server.Server.Init"
//	if s == nil {
//		return errors.New(op).Msg("nil server")
//	}
//	if s.CfgService == nil || s.DBService == nil {
//		return errors.New(op).Msg("dependencies not set")
//	}
//	if err := s.CfgService.Initialize(); err != nil {
//		return errors.New(op).Err(err)
//	}
//	if err := s.DBService.Logger.Initialize(); err != nil {
//		return errors.New(op).Err(err)
//	}
//	if err := s.DBService.Initialize(); err != nil {
//		return errors.New(op).Err(err)
//	}
//	if err := s.DBService.Open(); err != nil {
//		return errors.New(op).Err(err)
//	}
//	if err := s.DBService.Migrate(); err != nil {
//		return errors.New(op).Err(err)
//	}
//
//	app := fiber.New(fiber.Config{DisableStartupMessage: true})
//	// Routes
//	api := app.Group("/api/v1")
//	api.Post("/register", s.handleRegisterLogbook)
//
//	s.App = app
//	return nil
//}
//
//// Shutdown gracefully closes resources.
//func (s *AppServer) Shutdown(ctx context.Context) error {
//	const op errors.Op = "server.Server.Shutdown"
//	if s == nil {
//		return errors.New(op).Msg("nil server")
//	}
//	if s.DBService != nil {
//		if err := s.DBService.Close(); err != nil {
//			return errors.New(op).Err(err)
//		}
//	}
//	return nil
//}
//
//// handleCreateKey creates a new API key and stores its hash & prefix. Returns full key once.
//func (s *AppServer) handleRegisterLogbook(c *fiber.Ctx) error {
//	const op errors.Op = "server.Server.handleRegisterLogbook"
//	if s == nil {
//		return errors.New(op).Msg("nil server")
//	}
//	token := c.Get(fiber.HeaderAuthorization)
//
//	// TODO: check header value is valid
//	fmt.Println("Registering logbook:", token)
//
//	var body types.RegisterLogbookRequest
//	if err := c.BodyParser(&body); err != nil || body.Name == "" {
//		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
//	}
//
//	fmt.Println("Registering logbook:", body)
//
//	var logbook types.Logbook
//	adapter := adapters.New()
//	err := adapters.Copy(adapter, &logbook, &body)
//	if err != nil {
//		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
//	}
//
//	// Diagnostics: request deadline and DB pool stats
//	//if dl, ok := c.UserContext().Deadline(); ok {
//	//	logDebug("Request deadline:", dl)
//	//} else {
//	//	logDebug("Request deadline: none")
//	//}
//	//if debugServer {
//	//	s.DBService.LogStats("register:before-insert")
//	//}
//
//	// Use the request context so DB operations inherit request deadlines/cancellation.
//	if logbook, err = s.DBService.InsertLogbookContext(c.UserContext(), logbook); err != nil {
//		//fmt.Println("Error inserting logbook:", err)
//		//if debugServer {
//		//	s.DBService.LogStats("register:after-insert:error")
//		//}
//
//		if pgErr, ok := classifyPGError(err); ok {
//			if string(pgErr.Code) == "23505" { // unique_violation
//				msg := "resource already exists"
//				if pgErr.Constraint == "logbook_name_key" {
//					msg = "logbook name already exists"
//				}
//				return c.Status(http.StatusConflict).JSON(fiber.Map{"error": msg})
//			}
//		}
//		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "database error"})
//	}
//
//	// Post-diagnostics on success
//	//if debugServer {
//	//	s.DBService.LogStats("register:after-insert:ok")
//	//}
//
//	fmt.Println("Logbook registered:", logbook.ID)
//
//	full, prefix, hash, err := apikey.Generate(10)
//	full, err = apikey.FormatFullKey(full, 5)
//	if err != nil {
//		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "key generation failed"})
//	}
//
//	logbook.ApiKey = full
//
//	fmt.Println("Generated key: ", full, prefix, hash)
//
//	return c.Status(http.StatusCreated).JSON(logbook)
//}
//
//// handleGetKey returns metadata; if first retrieval it returns the full key and then wipes it.
////func (s *AppServer) handleGetKey(c *fiber.Ctx) error {
////	name := c.Params("name")
////	if name == "" {
////		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "missing name"})
////	}
////	// Lookup base record
////	query := "SELECT key_prefix, created_at, revoked_at, expires_at FROM api_keys WHERE key_name = $1"
////	driver := s.DBService.DatabaseConfig.Driver
////	if driver == database.SqliteDriver {
////		query = "SELECT key_prefix, created_at, revoked_at, expires_at FROM api_keys WHERE key_name = ?"
////	}
////	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
////	defer cancel()
////	rows, err := s.DBService.QueryContext(ctx, query, name)
////	if err != nil {
////		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "query failed"})
////	}
////	defer func(rows *sql.Rows) {
////		_ = rows.Close()
////	}(rows)
////	if !rows.Next() {
////		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "not found"})
////	}
////	var prefix string
////	var createdAt, revokedAt, expiresAt sql.NullTime
////	if scanErr := rows.Scan(&prefix, &createdAt, &revokedAt, &expiresAt); scanErr != nil {
////		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "scan failed"})
////	}
////	// Attempt one-time retrieval
////	fullKeyAny, hasFull := s.oneTimeStore.Load(name)
////	if hasFull {
////		s.oneTimeStore.Delete(name)
////	}
////	return c.JSON(fiber.Map{
////		"name":   name,
////		"prefix": prefix,
////		"full_key": func() string {
////			if hasFull {
////				return fullKeyAny.(string)
////			}
////			return ""
////		}(),
////		"created_at": func() string {
////			if createdAt.Valid {
////				return createdAt.Time.Format(time.RFC3339)
////			} else {
////				return ""
////			}
////		}(),
////		"revoked_at": func() string {
////			if revokedAt.Valid {
////				return revokedAt.Time.Format(time.RFC3339)
////			} else {
////				return ""
////			}
////		}(),
////		"expires_at": func() string {
////			if expiresAt.Valid {
////				return expiresAt.Time.Format(time.RFC3339)
////			} else {
////				return ""
////			}
////		}(),
////		"first_view": hasFull,
////	})
////}
//
//// classifyPGError unwraps an error chain (supporting both Unwrap() and Cause())
//// and returns a *pq.Error if present anywhere in the chain.
//func classifyPGError(err error) (*pq.Error, bool) {
//	for e := err; e != nil; {
//		if pg, ok := e.(*pq.Error); ok {
//			return pg, true
//		}
//		// Standard unwrapping
//		type unwrapper interface{ Unwrap() error }
//		if u, ok := e.(unwrapper); ok {
//			e = u.Unwrap()
//			continue
//		}
//		// friendsofgo/errors style
//		type causer interface{ Cause() error }
//		if c, ok := e.(causer); ok {
//			e = c.Cause()
//			continue
//		}
//		break
//	}
//	return nil, false
//}
