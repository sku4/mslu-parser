package excel

import (
	"context"
	"github.com/sku4/mslu-parser/models"
)

type Excel struct {
}

func New() *Excel {
	return &Excel{}
}

func (e *Excel) GetUsedUrls(context.Context) (map[string]*models.ExcelRow, error) {
	ss := make(map[string]*models.ExcelRow, 0)

	return ss, nil
}

func (e *Excel) SetComplex(context.Context, models.Complex) error {
	return nil
}
