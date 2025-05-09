package scraper

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/customeros/mailsherpa/domaincheck"
	"github.com/pkg/errors"

	"github.com/customeros/leads/internal/telemetry"
)

const (
	MaxCrawlDepth   = 5
	MaxPagesToCrawl = 100
)

func (s *scraperService) Crawl(ctx context.Context, domain string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	spans, ctx := telemetry.StartServiceSpan(ctx, "scraperService.Crawl")
	defer spans.Finish()

	_, primaryDomain := domaincheck.PrimaryDomainCheck(domain)
	if primaryDomain == "" {
		err := errors.New("Not a valid domain")
		spans.TraceError(err)
		return err
	}

	startUrl := fmt.Sprintf("https://%s", primaryDomain)

	workspaceDomains := []string{primaryDomain}
	results := make(chan string, MaxPagesToCrawl)
	errChan := make(chan error, 1)

	var wg sync.WaitGroup
	wg.Add(1)

	// Start recursive crawl
	go s.crawlRecursive(ctx, startUrl, workspaceDomains, 0, &wg, results, errChan)

	// Wait for completion in separate goroutine
	go func() {
		wg.Wait()
		close(results)
	}()

	return nil
}

func (s *scraperService) crawlRecursive(
	ctx context.Context,
	url string,
	workspaceDomains []string,
	depth int,
	wg *sync.WaitGroup,
	results chan<- string,
	errChan chan<- error,
) {
	defer wg.Done()

	// Check for already visited URLs
	if _, visited := s.visitedURLs.LoadOrStore(url, true); visited {
		return
	}

	// Check context and depth
	select {
	case <-ctx.Done():
		return
	default:
		if depth >= MaxCrawlDepth {
			return
		}
	}

	// Create span for this URL
	spans, ctx := telemetry.StartServiceSpan(ctx, "scraperService.crawlRecursive")
	defer spans.Finish()
	spans.LogKV("url", url)
	spans.LogKV("depth", depth)

	// Scrape current URL
	content, err := s.ScrapeWithJina(ctx, url)
	if err != nil {
		select {
		case errChan <- errors.Wrapf(err, "failed to scrape %s", url):
		default:
		}
		return
	}

	// Send URL to results
	results <- url

	// Get new links
	links, err := s.linksToCrawl(ctx, content, workspaceDomains)
	if err != nil {
		spans.TraceError(err)
		return
	}

	// Rate limiting channel
	limiter := make(chan struct{}, 5)

	// Crawl new links
	for _, link := range links {
		select {
		case <-ctx.Done():
			return
		case limiter <- struct{}{}:
			wg.Add(1)
			go func(link string) {
				defer func() { <-limiter }()
				s.crawlRecursive(ctx, link, workspaceDomains, depth+1, wg, results, errChan)
			}(link)
		}
	}
}

func shouldSkipURL(url string) bool {
	skipKeywords := []string{
		"login", "logout", "signin", "signout", "register", "signup",
		"account", "profile", "dashboard", "settings", "preferences",
		"cart", "checkout", "order", "payment", "billing",
		"mailto", "tel", "api", "webhook", "feed", "rss",
		"staging", "stage", "dev", "test", "beta", "sandbox",
		"session", "token", "search", "filter", "sort", "page",
		"share", "support", "help", "faq", "ticket", "contact",
		"calendar", "date", "archive", "tag", "admin", "manage",
		"status", "password",
	}

	lowercaseURL := strings.ToLower(url)
	for _, keyword := range skipKeywords {
		if strings.Contains(lowercaseURL, keyword) {
			return true
		}
	}
	return false
}
