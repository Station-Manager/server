package server

import (
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/iocdi"
)

type Service struct {
	container *iocdi.Container
}

func NewService() (*Service, error) {
	const op errors.Op = "server.NewService"
	svc := &Service{}

	if err := svc.initialize(); err != nil {
		return nil, errors.New(op).Err(err)
	}

	return svc, nil
}

func (s *Service) Start() error {
	return nil
}

func (s *Service) Shutdown() error {
	return nil
}
