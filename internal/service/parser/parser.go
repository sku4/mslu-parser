package parser

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/sku4/mslu-parser/internal/repository"
	"github.com/sku4/mslu-parser/internal/service/parser/zeit"
	"github.com/sku4/mslu-parser/models"
	"github.com/sku4/mslu-parser/models/cli"
	"github.com/sku4/mslu-parser/pkg/logger"
	"sync"
)

//go:generate mockgen -source=parser.go -destination=mocks/parser.go

type iProfile interface {
	Run(context.Context) error
	Shutdown() error
	SearchArticles(ctx context.Context, pageNum int) (*[]models.ExcelUrl, error)
	DownloadArticle(ctx context.Context, excelUrl *models.ExcelUrl) (*models.Complex, error)
}

type Service struct {
	repos        *repository.Repository
	profile      iProfile
	urlsChan     chan models.ExcelUrl
	complexChan  chan models.Complex
	completeChan chan struct{}
}

func NewService(repos *repository.Repository) *Service {
	return &Service{
		repos:        repos,
		completeChan: make(chan struct{}),
		urlsChan:     make(chan models.ExcelUrl, 10000),
		complexChan:  make(chan models.Complex, 10000),
	}
}

func (s *Service) Run(ctx context.Context) (err error) {
	args := cli.GetArgs(ctx)
	switch args.Profile {
	case "zeit":
		s.profile = zeit.New(s.repos)
	default:
		return errors.New(fmt.Sprintf("Profile '%s' not found", args.Profile))
	}

	if err = s.profile.Run(ctx); err != nil {
		return err
	}

	if err = s.parse(ctx); err != nil {
		return err
	}

	return nil
}

func (s *Service) Shutdown() error {
	if s.profile != nil {
		if err := s.profile.Shutdown(); err != nil {
			return err
		}
	}

	select {
	case <-s.completeChan:
	}

	return nil
}

func (s *Service) parse(ctx context.Context) (err error) {
	wg := &sync.WaitGroup{}

	// search new articles
	wg.Add(1)
	go func() {
		_ = s.searchArticles(ctx, wg)
	}()

	// download articles
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = s.downloadArticles(ctx, wg)
		}()
	}

	// save to file
	wgs := &sync.WaitGroup{}
	wgs.Add(1)
	go func() {
		_ = s.saveArticles(ctx, wgs)
	}()

	wg.Wait()
	close(s.complexChan)
	wgs.Wait()
	close(s.completeChan)

	return nil
}

func (s *Service) searchArticles(ctx context.Context, wg *sync.WaitGroup) error {
	defer wg.Done()
	if s.profile == nil {
		return models.ProfileNotInitError
	}
	log := logger.Get()

	pageNum := 1
	for {
		select {
		case <-ctx.Done():
			close(s.urlsChan)

			return nil
		default:
		}
		excelUrls, err := s.profile.SearchArticles(ctx, pageNum)
		if err != nil {
			if !errors.Is(err, models.ArticlesNotFoundError) {
				log.Errorf("Search articles pageNum %d error: %s", pageNum, err.Error())
			}
			break
		}
		for _, excelUrl := range *excelUrls {
			s.urlsChan <- excelUrl
		}
		pageNum++
	}
	close(s.urlsChan)

	return nil
}

func (s *Service) downloadArticles(ctx context.Context, wg *sync.WaitGroup) error {
	defer wg.Done()
	if s.profile == nil {
		return models.ProfileNotInitError
	}
	log := logger.Get()

	for excelUrl := range s.urlsChan {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		modelComplex, err := s.profile.DownloadArticle(ctx, &excelUrl)
		if err != nil {
			log.Errorf("Download article (%s) error: %s", excelUrl.Url, err.Error())
			break
		}
		s.complexChan <- *modelComplex
		// todo remove
		//time.Sleep(time.Second * 5)
	}

	return nil
}

func (s *Service) saveArticles(ctx context.Context, wgs *sync.WaitGroup) error {
	defer wgs.Done()
	for cx := range s.complexChan {
		_ = cx

	}

	return nil
}
