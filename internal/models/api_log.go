package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/customeros/leads/internal/enum"
)

type APICallLog struct {
	ID           string         `gorm:"column:id;primaryKey;type:varchar(25)"`
	Timestamp    time.Time      `gorm:"column:timestamp;primaryKey;type:timestamptz;not null"`
	Vendor       enum.APIVendor `gorm:"column:vendor;type:varchar(255);index;not null"`
	Method       string         `gorm:"column:method;type:varchar(255);not null"`
	URL          string         `gorm:"column:url;type:varchar(255);not null"`
	RequestID    string         `gorm:"column:request_id;type:varchar(55);not null"`
	RequestBody  []byte         `gorm:"column:request_body;type:bytea"`
	Duration     int            `gorm:"column:duration;type:int;not null"`
	StatusCode   *int           `gorm:"column:status_code;type:int;not null"`
	ResponseBody *[]byte        `gorm:"column:response_body;type:bytea"`
	ErrorMessage *string        `gorm:"column:error_message;type:text"`
}

func (a *APICallLog) TableName() string {
	return "api_call_logs"
}

func (a *APICallLog) BeforeCreate(tx *gorm.DB) error {
	if a.Timestamp.IsZero() {
		a.Timestamp = time.Now()
	}
	return nil
}

// CreateTable creates the api_call_logs table if it doesn't exist and sets up TimescaleDB features
func (a *APICallLog) CreateTable(db *gorm.DB) error {
	// Check if table exists
	tableExists := false
	err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'api_call_logs')").
		Scan(&tableExists).Error
	if err != nil {
		return fmt.Errorf("failed to check if api_call_logs table exists: %w", err)
	}

	// Create table if it doesn't exist
	if !tableExists {
		if err := db.AutoMigrate(&APICallLog{}); err != nil {
			return fmt.Errorf("failed to create api_call_logs table: %w", err)
		}
	}

	// Set up TimescaleDB features
	if err := initAPICallLogTable(db); err != nil {
		return fmt.Errorf("failed to setup TimescaleDB for api_call_logs: %w", err)
	}

	return nil
}

// setupAPICallLogTimescaleDB initializes the TimescaleDB specifics for the api_call_logs table
func initAPICallLogTable(db *gorm.DB) error {
	// Convert to hypertable - this only needs to be done once
	if err := db.Exec(`SELECT create_hypertable('api_call_logs', 'timestamp', 
		chunk_time_interval => INTERVAL '1 day',
		if_not_exists => TRUE)`).Error; err != nil {
		return err
	}

	// Create continuous aggregate for hourly API call statistics
	if err := db.Exec(`
		CREATE MATERIALIZED VIEW IF NOT EXISTS hourly_api_call_stats
		WITH (timescaledb.continuous) AS
		SELECT
			time_bucket('1 hour', timestamp) AS hour,
			vendor,
			method,
			status_code,
			avg(duration) AS avg_duration,
			count(*) AS call_count,
			count(CASE WHEN error_message IS NOT NULL THEN 1 END) AS error_count
		FROM api_call_logs
		GROUP BY hour, vendor, method, status_code
	`).Error; err != nil {
		return err
	}

	// Add refresh policy for continuous aggregate
	if err := db.Exec(`
		SELECT add_continuous_aggregate_policy('hourly_api_call_stats',
			start_offset => INTERVAL '1 day',
			end_offset => INTERVAL '1 hour',
			schedule_interval => INTERVAL '1 hour',
			if_not_exists => TRUE)
	`).Error; err != nil {
		return err
	}

	// Create additional indexes for common query patterns
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_api_call_logs_vendor_timestamp ON api_call_logs (vendor, timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_api_call_logs_status_timestamp ON api_call_logs (status_code, timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_api_call_logs_request_id ON api_call_logs (request_id);
		CREATE INDEX IF NOT EXISTS idx_api_call_logs_timestamp_duration ON api_call_logs (timestamp DESC, duration) 
			WHERE duration > 1000; -- Index for slow API calls (over 1 second)
	`).Error; err != nil {
		return err
	}
	return nil
}
