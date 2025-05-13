package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/customeros/leads/enum"
)

type ScraperEvent struct {
	ID           string            `gorm:"column:id;type:varchar(50);primaryKey;not null" json:"id"`
	Timestamp    time.Time         `gorm:"column:timestamp;primaryKey;type:timestamptz;not null"`
	Event        enum.Events       `gorm:"column:event;type:varchar(50);index;not null" json:"event"`
	Publisher    enum.LeadsService `gorm:"column:publisher;type:varchar(50);index;not null" json:"publisher"`
	Domain       string            `gorm:"column:domain;type:varchar(50);index;not null" json:"domain"`
	Url          string            `gorm:"column:url;type:varchar(255);index" json:"url"`
	Payload      []byte            `gorm:"column:payload;type:bytea" json:"-"`
	HasError     bool              `gorm:"column:has_error;type:boolean" json:"hasError"`
	ErrorMessage string            `gorm:"column:error_message;type:varchar(255)" json:"errorMessage"`
}

func (e *ScraperEvent) TableName() string {
	return "scraper_events"
}

func (e *ScraperEvent) BeforeCreate(tx *gorm.DB) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	return nil
}

func (e *ScraperEvent) CreateTable(db *gorm.DB) error {
	// Check if table exists
	tableExists := false
	err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'scraper_events')").
		Scan(&tableExists).Error
	if err != nil {
		return fmt.Errorf("failed to check if scraper_events table exists: %w", err)
	}

	// Create table if it doesn't exist
	if !tableExists {
		if err = db.AutoMigrate(&ScraperEvent{}); err != nil {
			return fmt.Errorf("failed to create scraper_events table: %w", err)
		}
	}

	// Set up TimescaleDB features
	if err := initScraperEventTable(db); err != nil {
		return fmt.Errorf("failed to setup TimescaleDB for scraper_events: %w", err)
	}

	return nil
}

func initScraperEventTable(db *gorm.DB) error {
	// Convert to hypertable - this only needs to be done once
	if err := db.Exec(`SELECT create_hypertable('scraper_events', 'timestamp', 
		chunk_time_interval => INTERVAL '1 day',
		if_not_exists => TRUE)`).Error; err != nil {
		return err
	}

	// Create continuous aggregate for hourly scraper statistics
	if err := db.Exec(`
		CREATE MATERIALIZED VIEW IF NOT EXISTS hourly_scraper_stats
		WITH (timescaledb.continuous) AS
		SELECT
			time_bucket('1 hour', timestamp) AS hour,
			publisher,
			event,
			domain,
			has_error,
			count(*) AS event_count
		FROM scraper_events
		GROUP BY hour, publisher, event, domain, has_error
	`).Error; err != nil {
		return err
	}

	// Add refresh policy for continuous aggregate
	if err := db.Exec(`
		SELECT add_continuous_aggregate_policy('hourly_scraper_stats',
			start_offset => INTERVAL '1 day',
			end_offset => INTERVAL '1 hour',
			schedule_interval => INTERVAL '1 hour',
			if_not_exists => TRUE)
	`).Error; err != nil {
		return err
	}

	// Create additional indexes for common query patterns
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_scraper_events_has_error_timestamp ON scraper_events (has_error, timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_scraper_events_publisher_event ON scraper_events (publisher, event);
		CREATE INDEX IF NOT EXISTS idx_scraper_events_domain_timestamp ON scraper_events (domain, timestamp DESC);
	`).Error; err != nil {
		return err
	}
	return nil
}
