package repository

import (
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/customeros/leads/internal/config"
	"github.com/customeros/leads/internal/database"
	"github.com/customeros/leads/internal/models"
)

type Repositories struct {
	APICallLog      APICallLogRepository
	Content         ContentRepository
	IPIntelligence  IPIntelligenceRepository
	Outbox          OutboxRepository
	ScraperEvent    ScraperEventRepository
	WebSession      WebSessionRepository
	WebTrackerEvent WebTrackerEventRepository
	WebTracker      WebTrackerRepository
}

func InitRepositories(leadsDB, warehouseDB *database.DbConnections) *Repositories {
	InitTimescaleTables(warehouseDB.WriteDB)
	return &Repositories{
		APICallLog:      NewAPICallLogRepository(warehouseDB),
		Content:         NewContentRepository(leadsDB),
		IPIntelligence:  NewIPIntelligenceRepository(leadsDB),
		Outbox:          NewOutboxRepository(leadsDB),
		ScraperEvent:    NewScraperEventRepository(warehouseDB),
		WebSession:      NewWebSessionRepository(leadsDB),
		WebTrackerEvent: NewWebTrackerEventRepository(warehouseDB),
		WebTracker:      NewWebTrackerRepository(leadsDB),
	}
}

func InitTimescaleTables(db *gorm.DB) {
	err := (&models.APICallLog{}).CreateTable(db)
	if err != nil {
		log.Fatalf("Unable to create WebTrackerEvent table in Warehouse")
	}
	err = (&models.ScraperEvent{}).CreateTable(db)
	if err != nil {
		log.Fatalf("Unable to create ScraperEvent table in Warehouse")
	}
	err = (&models.WebTrackerEvent{}).CreateTable(db)
	if err != nil {
		log.Fatalf("Unable to create WebTrackerEvent table in Warehouse")
	}
}

func MigrateLeadsDB(dbConfig *config.LeadsDatabaseConfig, leadsDB *gorm.DB) error {
	db, err := leadsDB.DB()
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(5)

	err = leadsDB.AutoMigrate(
		&models.Content{},
		&models.IPIntelligence{},
		&models.OutboxEvent{},
		&models.WebSession{},
		&models.WebTracker{},
	)

	db.SetMaxIdleConns(dbConfig.MaxIdleConn)
	db.SetMaxOpenConns(dbConfig.MaxConn)
	db.SetConnMaxLifetime(time.Duration(dbConfig.ConnMaxLifetime) * time.Minute)

	return err
}
