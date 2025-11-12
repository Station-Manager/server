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

//func defaultAppConfig() types.AppConfig {
//	return types.AppConfig{
//		DatastoreConfig: types.DatastoreConfig{
//			Driver:                    database.PostgresDriver,
//			Host:                      "localhost",
//			Port:                      5432,
//			User:                      "smuser",
//			Password:                  "1q2w3e4r",
//			Database:                  "station_manager",
//			SSLMode:                   "disable",
//			MaxOpenConns:              10,
//			MaxIdleConns:              10,
//			ConnMaxLifetime:           10,
//			ConnMaxIdleTime:           5,
//			ContextTimeout:            20,
//			TransactionContextTimeout: 10,
//		},
//
//		LoggingConfig: types.LoggingConfig{
//			Level:                  "debug",
//			WithTimestamp:          true,
//			ConsoleLogging:         true,
//			FileLogging:            false,
//			RelLogFileDir:          "logs",
//			SkipFrameCount:         3,
//			LogFileMaxSizeMB:       100,
//			LogFileMaxAgeDays:      30,
//			LogFileMaxBackups:      5,
//			ShutdownTimeoutMS:      10000,
//			ShutdownTimeoutWarning: true,
//		},
//	}
//}
