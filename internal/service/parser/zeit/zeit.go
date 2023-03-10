package zeit

import (
	"context"
	"github.com/sku4/mslu-parser/internal/repository"
	"github.com/sku4/mslu-parser/models"
	"github.com/sku4/mslu-parser/pkg/logger"
)

type Zeit struct {
	repos *repository.Repository
	urls  map[string]models.ExcelRow
}

func New(repos *repository.Repository) *Zeit {
	return &Zeit{
		repos: repos,
	}
}

const (
	url = "https://www.zeit.de/suche/index?q=&mode=1y&type=article"
)

func (z *Zeit) Run(ctx context.Context) (err error) {
	if err = z.Auth(ctx); err != nil {
		return err
	}

	z.urls, err = z.repos.Excel.GetUsedUrls(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (z *Zeit) Auth(ctx context.Context) error {
	return nil
}

func (z *Zeit) Shutdown() error {
	log := logger.Get()
	log.Info("Saving articles to excel")

	return nil
}

func (z *Zeit) SearchArticles(ctx context.Context, pageNum int) (*[]models.ExcelUrl, error) {
	if pageNum == 1 {
		return &[]models.ExcelUrl{
			{}, {}, {}, {}, {}, {}, {}, {}, {}, {},
			{}, {}, {}, {}, {}, {}, {}, {}, {}, {},
			{}, {}, {}, {}, {}, {}, {}, {}, {}, {},
		}, nil
	}

	return nil, models.ArticlesNotFoundError
}

func (z *Zeit) DownloadArticle(ctx context.Context, excelUrl *models.ExcelUrl) (*models.Complex, error) {
	return &models.Complex{
		ExcelUrl: *excelUrl,
	}, nil
}
