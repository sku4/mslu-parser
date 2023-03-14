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
	Auth(context.Context) error
	Shutdown() error
	SearchArticles(ctx context.Context, pageNum int) ([]models.ExcelUrl, error)
	DownloadArticle(ctx context.Context, excelUrl *models.ExcelUrl) (*models.Complex, error)
}

type Service struct {
	repos                *repository.Repository
	profile              iProfile
	urlsChan             chan models.ExcelUrl
	complexChan          chan models.Complex
	completeChan         chan struct{}
	isParseRun           bool
	urls                 map[string]*models.ExcelRow
	tooManyRequestsLimit int
	rwMutex              *sync.RWMutex
}

func NewService(repos *repository.Repository) *Service {
	return &Service{
		repos:        repos,
		completeChan: make(chan struct{}, 1),
		urlsChan:     make(chan models.ExcelUrl, 10000),
		complexChan:  make(chan models.Complex, 10000),
		rwMutex:      &sync.RWMutex{},
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

	s.urls, err = s.repos.Excel.GetUsedUrls(ctx)
	if err != nil {
		return err
	}

	if err = s.profile.Auth(ctx); err != nil {
		return err
	}

	s.tooManyRequestsLimit = 5
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

	if s.isParseRun {
		select {
		case <-s.completeChan:
		}
	}

	return nil
}

func (s *Service) parse(ctx context.Context) (err error) {
	s.isParseRun = true
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
	s.completeChan <- struct{}{}

	return nil
}

func (s *Service) searchArticles(ctx context.Context, wg *sync.WaitGroup) error {
	defer wg.Done()
	if s.profile == nil {
		return models.ProfileNotInitError
	}
	log := logger.Get()
	args := cli.GetArgs(ctx)

	pageNum := 1
	countLimit := args.Count
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
				log.Warnf("Search articles pageNum %d error: %s", pageNum, err.Error())
			}
			break
		}
		for _, excelUrl := range excelUrls {
			if countLimit == 0 {
				close(s.urlsChan)

				return nil
			}
			excelRow, hasUrl := s.urls[excelUrl.Url]
			excelUrl.ExcelRow = excelRow
			if !hasUrl || args.Update {
				s.urlsChan <- excelUrl
				countLimit--
			}
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

		s.rwMutex.RLock()
		if s.tooManyRequestsLimit == 0 {
			return nil
		}
		s.rwMutex.RUnlock()
		modelComplex, err := s.profile.DownloadArticle(ctx, &excelUrl)
		if err != nil {
			log.Errorf("Download article (%s) error: %s", excelUrl.Url, err.Error())
		}
		if modelComplex != nil && modelComplex.TooManyRequests {
			s.rwMutex.Lock()
			s.tooManyRequestsLimit--
			s.rwMutex.Unlock()
			log.Errorf("Download article too many requests (%s)", excelUrl.Url)
		} else if err == nil {
			s.complexChan <- *modelComplex
		}
	}

	return nil
}

func (s *Service) saveArticles(ctx context.Context, wgs *sync.WaitGroup) error {
	defer wgs.Done()
	log := logger.Get()

	for cx := range s.complexChan {
		err := s.repos.Excel.SetComplex(ctx, cx)
		if err != nil {
			log.Errorf("Save articles error: %s", err.Error())
			return errors.Wrap(err, "Save articles")
		}
	}

	return nil
}
