package web_event_processor

import (
	"gorm.io/gorm"

	interfaces "github.com/customeros/leads/interfaces"
	nats_internal "github.com/customeros/leads/internal/nats"
	"github.com/customeros/leads/internal/repository"
)

type webEventProcessor struct {
	natsConn     *nats_internal.NATSConnections
	leadsWriteDB *gorm.DB
	repositories *repository.Repositories
}

func NewWebEventProcessor(
	natsConn *nats_internal.NATSConnections,
	leadsWriteDB *gorm.DB,
	repositories *repository.Repositories,
) interfaces.WebEventProcessor {
	return &webEventProcessor{
		natsConn:     natsConn,
		leadsWriteDB: leadsWriteDB,
		repositories: repositories,
	}
}
