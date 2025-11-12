package main

import (
	"github.com/Station-Manager/config"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/server/server"
	"os"
)

func init() {
	if err := os.Setenv(config.EnvSmDefaultDB, "pg"); err != nil {
		panic(err)
	}
}

func main() {
	svc, err := server.NewService()
	if err != nil {
		dErr, ok := errors.AsDetailedError(err)
		if !ok {
			panic(err)
		}
		panic(dErr.Cause().Error())
	}

	if err = svc.Start(); err != nil {
		panic(err)
	}

	if err = svc.Shutdown(); err != nil {
		panic(err)
	}
}
