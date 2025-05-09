package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/customeros/leads/internal/database"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
)

type ScraperEventRepository interface {
	Create(ctx context.Context, event *models.ScraperEvent) error
}

type scraperEventRepository struct {
	read  *gorm.DB
	write *gorm.DB
}

func NewScraperEventRepository(db *database.DbConnections) ScraperEventRepository {
	return &scraperEventRepository{
		read:  db.ReadDB,
		write: db.WriteDB,
	}
}

func (r *scraperEventRepository) Create(ctx context.Context, event *models.ScraperEvent) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "scraperEventRepository.Create")
	defer span.Finish()

	// Generate ID if not provided
	if event.ID == "" {
		event.ID = utils.GenerateNanoIDWithPrefix("scrp", 16)
	}

	return r.write.WithContext(ctx).Create(event).Error
}
