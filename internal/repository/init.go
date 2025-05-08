package repository

import (
	"time"

	"gorm.io/gorm"

	"github.com/customeros/leads/internal/config"
	"github.com/customeros/leads/internal/database"
	"github.com/customeros/leads/internal/models"
)

type Repositories struct {
	APICallLogRepository APICallLogRepository
	ContentRepository    ContentRepository
	IPIntelligence       IPIntelligenceRepository
	Outbox               OutboxRepository
	WebSessionRepository WebSessionRepository
	WebTrackerEvent      WebTrackerEventRepository
	WebTracker           WebTrackerRepository
}

func InitRepositories(leadsDB, warehouseDB *database.DbConnections) *Repositories {
	return &Repositories{
		ContentRepository:    NewContentRepository(leadsDB),
		IPIntelligence:       NewIPIntelligenceRepository(leadsDB),
		Outbox:               NewOutboxRepository(leadsDB),
		WebSessionRepository: NewWebSessionRepository(leadsDB),
		WebTracker:           NewWebTrackerRepository(leadsDB),

		APICallLogRepository: NewAPICallLogRepository(warehouseDB),
		WebTrackerEvent:      NewWebTrackerEventRepository(warehouseDB),
	}
}

func MigrateLeadsDB(dbConfig *config.LeadsDatabaseConfig, leadsDB *gorm.DB) error {
	db, err := leadsDB.DB()
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(5)

	err = leadsDB.AutoMigrate(
		&models.IPIntelligence{},
		&models.OutboxEvent{},
		&models.WebTracker{},
		&models.WebSession{},
	)

	db.SetMaxIdleConns(dbConfig.MaxIdleConn)
	db.SetMaxOpenConns(dbConfig.MaxConn)
	db.SetConnMaxLifetime(time.Duration(dbConfig.ConnMaxLifetime) * time.Minute)

	return err
}

func MigrateDataWarehouse(dbConfig *config.DataWarehouseConfig, warehouseDB *gorm.DB) error {
	db, err := warehouseDB.DB()
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(5)

	err = warehouseDB.AutoMigrate(
		&models.APICallLog{},
		&models.WebTrackerEvent{},
	)

	db.SetMaxIdleConns(dbConfig.MaxIdleConn)
	db.SetMaxOpenConns(dbConfig.MaxConn)
	db.SetConnMaxLifetime(time.Duration(dbConfig.ConnMaxLifetime) * time.Minute)

	return err
}
