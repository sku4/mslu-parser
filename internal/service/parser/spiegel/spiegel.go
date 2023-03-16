package spiegel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	"github.com/sku4/mslu-parser/models"
	"github.com/sku4/mslu-parser/models/cli"
	"github.com/sku4/mslu-parser/models/spiegel"
	"github.com/sku4/mslu-parser/pkg/logger"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Spiegel struct {
	authCookie []*http.Cookie
}

func New() *Spiegel {
	return &Spiegel{}
}

const (
	searchUrl   = "https://www.spiegel.de/services/sitesearch/search?segments=%s&fields=%s&q=%s&after=%d&before=%d&page_size=50&page=%d"
	authUrl     = "https://gruppenkonto.spiegel.de/anmelden.html"
	authUrlHost = "https://gruppenkonto.spiegel.de"
	targetUrl   = "https://www.spiegel.de"
	cookieAuth  = "accessInfo"
)

func (s *Spiegel) Auth(ctx context.Context) error {
	s.authCookie = make([]*http.Cookie, 0, 10)
	args := cli.GetArgs(ctx)
	if args.Login == "" || args.Password == "" {
		return errors.New("login or password not set")
	}

	var client http.Client
	reqCsrf, err := http.NewRequest(http.MethodGet, authUrl, bytes.NewBuffer([]byte{}))
	if err != nil {
		return errors.New(fmt.Sprintf("error create request: %s", err.Error()))
	}
	respCsrf, err := client.Do(reqCsrf)
	if err != nil {
		return errors.New(fmt.Sprintf("error request csrf page: %s", err.Error()))
	}
	defer func() {
		_ = respCsrf.Body.Close()
	}()

	doc, err := goquery.NewDocumentFromReader(respCsrf.Body)
	if err != nil {
		return errors.Wrap(err, "create document reader")
	}

	csrfToken := ""
	doc.Find("#loginform input").Each(func(i int, s *goquery.Selection) {
		if v, b := s.Attr("name"); b && v == "_csrf" {
			csrfToken, _ = s.Attr("value")
		}
	})

	if csrfToken == "" {
		return errors.New("csrf token not found")
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	_ = w.WriteField("loginform", "loginform")
	_ = w.WriteField("_csrf", csrfToken)
	_ = w.WriteField("targetUrl", targetUrl)
	_ = w.WriteField("requestAccessToken", "true")
	_ = w.WriteField("loginform:step", "passwort")
	_ = w.WriteField("loginform:username", args.Login)
	_ = w.WriteField("loginform:password", args.Password)
	_ = w.WriteField("loginform:submit", "")
	_ = w.WriteField("javax.faces.ViewState", "stateless")
	_ = w.Close()

	reqAuth, err := http.NewRequest(http.MethodPost, authUrl, &b)
	if err != nil {
		return errors.New(fmt.Sprintf("error create request: %s", err.Error()))
	}
	reqAuth.Header.Set("Content-Type", w.FormDataContentType())
	reqAuth.Header.Set("Host", authUrlHost)
	reqAuth.Header.Set("Referer", authUrl)
	respAuth, err := client.Do(reqAuth)
	if err != nil {
		return errors.New(fmt.Sprintf("error request auth page: %s", err.Error()))
	}
	defer func() {
		_ = respAuth.Body.Close()
	}()

	resp := respAuth
	for resp != nil {
		s.authCookie = append(s.authCookie, resp.Cookies()...)
		resp = resp.Request.Response
	}

	hasCookieAuth := false
	for _, c := range s.authCookie {
		if c.Name == cookieAuth {
			hasCookieAuth = true
			break
		}
	}
	if !hasCookieAuth {
		return errors.New("error cookies not found")
	}

	return nil
}

func (s *Spiegel) Shutdown() error {
	log := logger.Get()
	log.Info("Saving articles to excel")

	return nil
}

func (s *Spiegel) SearchArticles(ctx context.Context, pageNum int) ([]models.ExcelUrl, error) {
	args := cli.GetArgs(ctx)
	before := time.Now()
	after := before.AddDate(0, 0, -args.SpiegelZeitraum)
	searchArticlesUrl := fmt.Sprintf(searchUrl, url.QueryEscape(args.SpiegelSegments), args.SpiegelInhalt,
		args.SpiegelSuchbegriff, after.Unix(), before.Unix(), pageNum)
	resp, err := s.request(ctx, searchArticlesUrl)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("status code %d", resp.StatusCode))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var search spiegel.Search
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "search read all")
	}
	err = json.Unmarshal(body, &search)
	if err != nil {
		return nil, errors.Wrap(err, "search unmarshal")
	}

	excelUrls := make([]models.ExcelUrl, 0)
	for _, result := range search.Results {
		if result.Url != "" {
			excelUrls = append(excelUrls, models.ExcelUrl{
				Url: result.Url,
			})
		}
	}

	if len(excelUrls) == 0 {
		return nil, models.ArticlesNotFoundError
	}

	return excelUrls, nil
}

func (s *Spiegel) DownloadArticle(ctx context.Context, excelUrl *models.ExcelUrl) (*models.Complex, error) {
	resp, err := s.request(ctx, excelUrl.Url)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("status code %d", resp.StatusCode))
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	modelComplex := &models.Complex{
		ExcelUrl: *excelUrl,
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		modelComplex.TooManyRequests = true

		return modelComplex, nil
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "create document reader")
	}

	subtitles := make([]string, 0)
	imageTitles := make([]string, 0)
	title := doc.Find("main article header h2 .align-middle").Text()
	overTitle := doc.Find("main article header h2 .text-primary-base").Text()
	lead := doc.Find("main article header .leading-loose").Text()
	doc.Find("main article section h3").Each(func(i int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) != "" {
			subtitles = append(subtitles, strings.TrimSpace(s.Text()))
		}
	})
	doc.Find("main article figcaption p").Each(func(i int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) != "" {
			imageTitles = append(imageTitles, strings.TrimSpace(s.Text()))
		}
	})

	if title == "" {
		return nil, models.ArticleNotFoundError
	}

	modelComplex.Title = strings.TrimSpace(title)
	modelComplex.OverTitle = strings.TrimSpace(overTitle)
	modelComplex.Lead = strings.TrimSpace(lead)
	modelComplex.Subtitles = subtitles
	modelComplex.ImageTitles = imageTitles

	return modelComplex, nil
}

func (s *Spiegel) request(ctx context.Context, url string) (*http.Response, error) {
	var client http.Client
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error create request: %s", err.Error()))
	}
	for _, c := range s.authCookie {
		req.AddCookie(c)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error request body page: %s", err.Error()))
	}

	return resp, nil
}
