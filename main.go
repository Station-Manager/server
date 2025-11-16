package main

import (
	"context"
	"github.com/Station-Manager/config"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/server/server"
	"os"
	"os/signal"
	"syscall"
)

func init() {
	// Ensure the default database is set to PostgreSQL
	if err := os.Setenv(config.EnvSmDefaultDB, "pg"); err != nil {
		panic(err)
	}
}

func main() {
	// Create context that will be canceled on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	svc, err := server.NewService()
	if err != nil {
		dErr, ok := errors.AsDetailedError(err)
		if !ok {
			panic(err)
		}
		panic(dErr.Cause().Error())
	}

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- svc.Start()
	}()

	// Wait for interrupt signal or server error
	select {
	case <-ctx.Done():
		// Signal received, initiate graceful shutdown
		stop() // Stop receiving more signals
		if err := svc.Shutdown(); err != nil {
			panic(err)
		}
	case err := <-errChan:
		// Server error occurred
		if err != nil {
			panic(err)
		}
	}
}
