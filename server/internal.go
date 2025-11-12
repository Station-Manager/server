package server

import (
	"github.com/Station-Manager/config"
	"github.com/Station-Manager/database"
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/iocdi"
	"github.com/Station-Manager/logging"
	"github.com/Station-Manager/utils"
	"reflect"
)

func (s *Service) initialize() error {
	const op errors.Op = "server.Service.initialize"
	if s == nil {
		return errors.New(op).Msg("nil server")
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
