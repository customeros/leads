package scraper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/customeros/mailsherpa/domaincheck"

	"github.com/customeros/leads/internal/clients"
	"github.com/customeros/leads/internal/enum"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
)

var (
	ErrPaymentRequired = errors.New("jina balance requires topup")
	ErrUnprocessable   = errors.New("jina cannot process webpage")
)

func (s *scraperService) ScrapeWithJina(ctx context.Context, url string) (string, error) {
	spans, ctx := telemetry.StartServiceSpan(ctx, "webscraperService.ScrapeWithJina")
	defer spans.Finish()
	spans.LogKV("url", url)

	cleanUrl := cleanUrl(url)

	// check cache
	cachedData, err := s.checkCache(ctx, url)
	if err != nil {
		spans.TraceError(err)
		return "", err
	}
	if cachedData != "" {
		return cachedData, nil
	}

	// fetch page contents
	contents, err := s.fetchPageWithJina(ctx, url)
	if err != nil {
		switch err {
		case ErrPaymentRequired:
			spans.TraceError(err)
			return "", nil

		case ErrUnprocessable:
			spans.LogKV("error", ErrUnprocessable)
			return "", err

		default:
			spans.TraceError(err)
			return "", err
		}
	}

	// write scraped content to db
	_, primaryDomain := domaincheck.PrimaryDomainCheck(utils.ExtractDomain(url))

	switch {
	case strings.Contains(contents, "403 Forbidden") || strings.Contains(contents, "Robot Challenge"):
		err := s.handleScraperError(ctx, primaryDomain, cleanUrl, ErrUnprocessable.Error())
		if err != nil {
			spans.TraceError(err)
			return "", err
		}
		return "", nil

	case strings.Contains(contents, ""):
		err := s.handleScraperError(ctx, primaryDomain, cleanUrl, "no content")
		if err != nil {
			spans.TraceError(err)
			return "", err
		}
		return "", nil

	default:
		content, links := processWebContent(ctx, contents)

		err := s.repositories.ContentRepository.Create(ctx, &models.Content{
			Domain:  primaryDomain,
			Url:     url,
			Content: content,
			Links:   links,
		})
		if err != nil {
			spans.TraceError(err)
			return "", err
		}

		return contents, nil
	}
}

func (s *scraperService) handleScraperError(ctx context.Context, domain, url, errMsg string) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "scraperService.handleScraperError")
	defer span.Finish()

	err := s.repositories.ContentRepository.Create(ctx, &models.Content{
		Domain:       domain,
		Url:          url,
		ErrorMessage: errMsg,
	})
	if err != nil {
		span.TraceError(err)
		return err
	}
	return nil
}

func processWebContent(ctx context.Context, content string) (string, []string) {
	spans, ctx := telemetry.StartServiceSpan(ctx, "WebscraperService.postProcessWebContent")
	defer spans.Finish()

	// extract links and save
	sections := strings.Split(content, "Links/Buttons:")
	if len(sections) < 2 {
		cleanContent := processMarkdownWebpage(content)
		return cleanContent, nil
	}

	content = processMarkdownWebpage(sections[0])
	links := extractLinks(sections[1])

	spans.LogKV("result.links.count", len(links))
	spans.LogKV("result.content", content)
	spans.LogObjectAsJson("result.links", links)
	return content, links
}

func (s *scraperService) fetchPageWithJina(ctx context.Context, url string) (string, error) {
	spans, ctx := telemetry.StartServiceSpan(ctx, "WebscraperService.fetchPage")
	defer spans.Finish()

	if s.config.ApiKey == "" {
		err := errors.New("jina API key not set")
		spans.TraceError(err)
		return "", err
	}

	requestUrl := s.config.Url + url
	spans.LogKV("requestUrl", requestUrl)

	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		spans.TraceError(err)
		return "", err
	}

	// Align headers with working curl command
	req.Header.Set("Authorization", "Bearer "+s.config.ApiKey)
	req.Header.Set("X-Retain-Images", "none")      // Don't retain images
	req.Header.Set("X-With-Links-Summary", "true") // Include links summary

	// Add a timeout
	client := clients.NewLoggingClient(s.repositories.APICallLogRepository, enum.VendorJina)

	resp, err := client.Do(req)
	if err != nil {
		spans.TraceError(err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case 402:
			return "", ErrPaymentRequired

		case 422:
			return "", ErrUnprocessable

		default:
			err = fmt.Errorf("error code: %d", resp.StatusCode)
			spans.TraceError(err)
			return "", err
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("error reading response body: %v", err)
		spans.TraceError(err)
		return "", err
	}

	return string(body), nil
}

func (s *scraperService) checkCache(ctx context.Context, url string) (string, error) {
	spans, ctx := telemetry.StartServiceSpan(ctx, "webscraperService.checkCache")
	defer spans.Finish()

	record, err := s.repositories.ContentRepository.GetByUrl(ctx, url)
	if err != nil {
		spans.TraceError(err)
		return "", err
	}
	if record == nil || record.Content == "" {
		return "", nil
	}

	return record.Content, nil
}

func cleanUrl(url string) string {
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimSuffix(url, "#")
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	return url
}
