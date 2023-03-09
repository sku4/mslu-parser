package parser

import (
	"context"
	"github.com/sku4/mslu-parser/internal/repository"
)

//go:generate mockgen -source=parser.go -destination=mocks/parser.go

type Service struct {
	repos *repository.Repository
}

func (s *Service) Run(context.Context) error {
	return nil
}

func (s *Service) Shutdown() error {
	return nil
}

func NewService(repos *repository.Repository) *Service {
	return &Service{
		repos: repos,
	}
}
