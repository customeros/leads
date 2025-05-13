package services

import (
	"context"
	"fmt"

	"github.com/customeros/leads/interfaces"
	"github.com/customeros/leads/internal/config"
	"github.com/customeros/leads/internal/database"
	nats_internal "github.com/customeros/leads/internal/nats"
	"github.com/customeros/leads/internal/repository"
	"github.com/customeros/leads/services/content_profiler"
	"github.com/customeros/leads/services/ipdata"
	"github.com/customeros/leads/services/outbox_processor"
	"github.com/customeros/leads/services/proxy_manager"
	"github.com/customeros/leads/services/scraper"
	"github.com/customeros/leads/services/session_manager"
	"github.com/customeros/leads/services/snitcher"
	"github.com/customeros/leads/services/web_event_processor"
	"github.com/customeros/leads/services/webtracker"
)

type Services struct {
	ContentProfiler   *content_profiler.ContentProfiler
	IPDataService     *ipdata.IPDataService
	OutboxProcessor   *outbox_processor.OutboxProcessor
	ProxyManager      proxy_manager.ProxyManagerService
	ScraperService    scraper.ScraperService
	SessionManager    session_manager.SessionManager
	SnitcherService   *snitcher.SnitcherService
	WebEventProcessor interfaces.WebEventProcessor
	WebtrackerService webtracker.WebtrackerService
}

func (s *Services) Start(ctx context.Context) error {
	services := []struct {
		name    string
		starter func(context.Context) error
	}{
		{"Content Profiler", s.ContentProfiler.Start},
		{"Session Manager Service", s.SessionManager.Start},
		{"Snitcher Service", s.SnitcherService.Start},
		{"IPData Service", s.IPDataService.Start},
		{"Scraper Service", s.ScraperService.Start},
	}

	for _, svc := range services {
		if err := svc.starter(ctx); err != nil {
			return fmt.Errorf("failed to start %s service: %w", svc.name, err)
		}
	}

	return nil
}

func (s *Services) Stop(ctx context.Context) {
	services := []struct {
		name    string
		stopper func(context.Context)
	}{
		{"Content Profiler", func(ctx context.Context) { s.ContentProfiler.Stop() }},
		{"Session Manager Service", func(ctx context.Context) { s.SessionManager.Stop() }},
		{"Snitcher Service", func(ctx context.Context) { s.SnitcherService.Stop() }},
		{"IPData Service", func(ctx context.Context) { s.IPDataService.Stop() }},
		{"Scraper Service", func(ctx context.Context) { s.ScraperService.Stop() }},
	}

	for _, service := range services {
		service.stopper(ctx)
	}
}

func InitServices(config *config.Config, leadsDB *database.DbConnections, natsConn *nats_internal.NATSConnections, repositories *repository.Repositories) *Services {
	return &Services{
		ContentProfiler:   content_profiler.NewContentProfiler(natsConn, leadsDB, repositories),
		IPDataService:     ipdata.NewIPDataService(config.IPDataConfig, natsConn, repositories),
		OutboxProcessor:   outbox_processor.NewOutboxProcessor(natsConn, repositories),
		ProxyManager:      proxy_manager.NewProxyManagerService(natsConn, leadsDB, repositories),
		ScraperService:    scraper.NewScraperService(config.JinaConfig, natsConn, leadsDB, repositories),
		SessionManager:    session_manager.NewSessionManager(natsConn, leadsDB, repositories),
		SnitcherService:   snitcher.NewSnitcherService(config.SnitcherConfig, repositories, natsConn),
		WebEventProcessor: web_event_processor.NewWebEventProcessor(natsConn, leadsDB.WriteDB, repositories),
		WebtrackerService: webtracker.NewWebtrackerService(leadsDB.WriteDB, repositories),
	}
}
