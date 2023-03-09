package service

import (
	"context"
	"github.com/sku4/mslu-parser/internal/repository"
	"github.com/sku4/mslu-parser/internal/service/parser"
)

//go:generate mockgen -source=service.go -destination=mocks/service.go

type Parser interface {
	Run(context.Context) error
	Shutdown() error
}

type Service struct {
	Parser
}

func NewService(ctx context.Context, repos *repository.Repository) *Service {
	return &Service{
		Parser: parser.NewService(repos),
	}
}
