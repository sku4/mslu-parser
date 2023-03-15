package zeit

import (
	"bytes"
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	"github.com/sku4/mslu-parser/internal/repository"
	"github.com/sku4/mslu-parser/models"
	"github.com/sku4/mslu-parser/models/cli"
	"github.com/sku4/mslu-parser/pkg/logger"
	"mime/multipart"
	"net/http"
	"strings"
)

type Zeit struct {
	repos      *repository.Repository
	authCookie []*http.Cookie
}

func New(repos *repository.Repository) *Zeit {
	return &Zeit{
		repos: repos,
	}
}

const (
	searchUrl        = "https://www.zeit.de/suche/index?q=&mode=%s&type=%s&p=%d"
	authUrl          = "https://meine.zeit.de/anmelden"
	cookieAuthPrefix = "zeit_sso_"
)

func (z *Zeit) Auth(ctx context.Context) error {
	z.authCookie = make([]*http.Cookie, 0, 4)
	args := cli.GetArgs(ctx)
	if args.Login == "" || args.Password == "" {
		return errors.New("login or password not set")
	}

	reqCsrf, err := http.NewRequest(http.MethodGet, authUrl, bytes.NewBuffer([]byte{}))
	if err != nil {
		return errors.New(fmt.Sprintf("error create request: %s", err.Error()))
	}
	var client http.Client
	respCsrf, err := client.Do(reqCsrf)
	if err != nil {
		return errors.New(fmt.Sprintf("error request csrf page: %s", err.Error()))
	}
	defer func() {
		_ = respCsrf.Body.Close()
	}()

	csrfCookies := respCsrf.Cookies()
	csrfToken := ""
	for _, cookie := range csrfCookies {
		if cookie.Name == "csrf_token" {
			csrfToken = cookie.Value
			break
		}
	}

	if csrfToken == "" {
		return errors.New("csrf token not found")
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	_ = w.WriteField("entry_service", "sonstige")
	_ = w.WriteField("product_id", "sonstige")
	_ = w.WriteField("return_url", "")
	_ = w.WriteField("email", args.Login)
	_ = w.WriteField("pass", args.Password)
	_ = w.WriteField("permanent", "on")
	_ = w.WriteField("csrf_token", csrfToken)
	_ = w.Close()

	reqAuth, err := http.NewRequest(http.MethodPost, authUrl, &b)
	if err != nil {
		return errors.New(fmt.Sprintf("error create request: %s", err.Error()))
	}
	for _, cookie := range csrfCookies {
		reqAuth.AddCookie(cookie)
	}
	reqAuth.Header.Set("Content-Type", w.FormDataContentType())
	reqAuth.Header.Set("Host", "meine.zeit.de")
	reqAuth.Header.Set("Referer", "https://meine.zeit.de/anmelden")
	respAuth, err := client.Do(reqAuth)
	if err != nil {
		return errors.New(fmt.Sprintf("error request auth page: %s", err.Error()))
	}
	defer func() {
		_ = respAuth.Body.Close()
	}()

	resp := respAuth
	for resp != nil {
		z.authCookie = append(z.authCookie, resp.Cookies()...)
		resp = resp.Request.Response
	}

	hasCookieAuthPrefix := false
	for _, c := range z.authCookie {
		if strings.Contains(c.Name, cookieAuthPrefix) {
			hasCookieAuthPrefix = true
			break
		}
	}
	if !hasCookieAuthPrefix {
		return errors.New("error cookies not found")
	}

	firstCookie := z.authCookie[0]
	zonConsentCookie := &http.Cookie{
		Name:     "zonconsent",
		Value:    "2023-03-14T16:29:12.611Z",
		Domain:   firstCookie.Domain,
		Expires:  firstCookie.Expires,
		Path:     firstCookie.Path,
		Secure:   firstCookie.Secure,
		HttpOnly: firstCookie.HttpOnly,
	}
	z.authCookie = append(z.authCookie, zonConsentCookie)

	return nil
}

func (z *Zeit) Shutdown() error {
	log := logger.Get()
	log.Info("Saving articles to excel")

	return nil
}

func (z *Zeit) SearchArticles(ctx context.Context, pageNum int) ([]models.ExcelUrl, error) {
	args := cli.GetArgs(ctx)
	url := fmt.Sprintf(searchUrl, args.ZeitMode, args.ZeitType, pageNum)
	resp, err := z.request(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("status code %d", resp.StatusCode))
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "create document reader")
	}

	excelUrls := make([]models.ExcelUrl, 0)
	doc.Find("a.zon-teaser-standard__faux-link").Each(func(i int, s *goquery.Selection) {
		hasZPlus := s.Parent().Find(".zon-teaser-standard__heading svg.zplus-logo").Length() > 0
		href, exists := s.Attr("href")
		if exists && href != "" && !hasZPlus {
			excelUrls = append(excelUrls, models.ExcelUrl{
				Url: href,
			})
		}
	})

	if len(excelUrls) == 0 {
		return nil, models.ArticlesNotFoundError
	}

	return excelUrls, nil
}

func (z *Zeit) DownloadArticle(ctx context.Context, excelUrl *models.ExcelUrl) (*models.Complex, error) {
	resp, err := z.request(ctx, excelUrl.Url)
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
	title := doc.Find(".article-header h1 .article-heading__title").Text()
	overTitle := doc.Find(".article-header .article-heading__kicker").Text()
	lead := doc.Find(".article-header .summary").Text()
	doc.Find("h2.article__subheading article__item").Each(func(i int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) != "" {
			subtitles = append(subtitles, strings.TrimSpace(s.Text()))
		}
	})

	if title == "" {
		return nil, models.ArticleNotFoundError
	}

	modelComplex.Title = strings.TrimSpace(title)
	modelComplex.OverTitle = strings.TrimSpace(overTitle)
	modelComplex.Lead = strings.TrimSpace(lead)
	modelComplex.Subtitles = subtitles

	return modelComplex, nil
}

func (z *Zeit) request(ctx context.Context, url string) (*http.Response, error) {
	var client http.Client
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error create request: %s", err.Error()))
	}
	for _, c := range z.authCookie {
		req.AddCookie(c)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error request body page: %s", err.Error()))
	}

	return resp, nil
}
