package repository

import (
	"context"
	"github.com/sku4/mslu-parser/internal/repository/excel"
	"github.com/sku4/mslu-parser/models"
)

//go:generate mockgen -source=repository.go -destination=mocks/repository.go

type Excel interface {
	GetUsedUrls(context.Context) (map[uint32]*models.ExcelRow, error)
	SetComplex(context.Context, models.Complex) error
	Close() error
}

type Repository struct {
	Excel
}

func NewRepository() *Repository {
	return &Repository{
		Excel: excel.New(),
	}
}
