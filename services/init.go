package services

import (
	"context"

	"github.com/customeros/leads/internal/config"
	"github.com/customeros/leads/internal/database"
	nats_internal "github.com/customeros/leads/internal/nats"
	"github.com/customeros/leads/internal/repository"
	"github.com/customeros/leads/services/ipdata"
	"github.com/customeros/leads/services/outbox_processor"
	"github.com/customeros/leads/services/proxy_manager"
	"github.com/customeros/leads/services/session_manager"
	"github.com/customeros/leads/services/snitcher"
	"github.com/customeros/leads/services/web_event_processor"
	"github.com/customeros/leads/services/webtracker"
)

type Services struct {
	IPDataService     *ipdata.IPDataService
	OutboxProcessor   *outbox_processor.OutboxProcessor
	ProxyManager      proxy_manager.ProxyManagerService
	SessionManager    session_manager.SessionManager
	SnitcherService   *snitcher.SnitcherService
	WebEventProcessor web_event_processor.WebEventProcessor
	WebtrackerService webtracker.WebtrackerService
}

func (s *Services) Start(ctx context.Context) error {
	return nil
}

func (s *Services) Stop(ctx context.Context) {
	return
}

func InitServices(config *config.Config, leadsDB *database.DbConnections, natsConn *nats_internal.NATSConnections, repositories *repository.Repositories) *Services {
	return &Services{
		IPDataService:     ipdata.NewIPDataService(config.IPDataConfig, natsConn, repositories),
		OutboxProcessor:   outbox_processor.NewOutboxProcessor(natsConn, repositories),
		ProxyManager:      proxy_manager.NewProxyManagerService(natsConn, leadsDB, repositories),
		SessionManager:    session_manager.NewSessionManager(natsConn, leadsDB, repositories),
		SnitcherService:   snitcher.NewSnitcherService(config.SnitcherConfig, repositories, natsConn),
		WebEventProcessor: web_event_processor.NewWebEventProcessor(natsConn, leadsDB.WriteDB, repositories),
		WebtrackerService: webtracker.NewWebtrackerService(leadsDB.WriteDB, repositories),
	}
}
