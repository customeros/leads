package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/customeros/leads/enum"
)

type APICallLog struct {
	ID           string         `gorm:"column:id;type:varchar(25);not null"`
	Timestamp    time.Time      `gorm:"column:timestamp;type:timestamptz;not null"`
	Vendor       enum.APIVendor `gorm:"column:vendor;type:varchar(255);not null"`
	Method       string         `gorm:"column:method;type:varchar(255);not null"`
	URL          string         `gorm:"column:url;type:varchar(255);not null"`
	RequestBody  []byte         `gorm:"column:request_body;type:bytea"`
	Duration     int            `gorm:"column:duration;type:int;not null"`
	StatusCode   *int           `gorm:"column:status_code;type:int"`
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
	// Disable GORM's auto migrations for this table
	db = db.Set("gorm:table_options", "")

	// Check if table exists
	tableExists := false
	err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'api_call_logs')").
		Scan(&tableExists).Error
	if err != nil {
		return fmt.Errorf("failed to check if api_call_logs table exists: %w", err)
	}

	// Create table manually to ensure proper structure for TimescaleDB
	if !tableExists {
		err := db.Exec(`
			CREATE TABLE IF NOT EXISTS api_call_logs (
				id VARCHAR(25) NOT NULL,
				timestamp TIMESTAMPTZ NOT NULL,
				vendor VARCHAR(255) NOT NULL,
				method VARCHAR(255) NOT NULL,
				url VARCHAR(255) NOT NULL,
				request_body BYTEA,
				duration INT NOT NULL,
				status_code INT NOT NULL,
				response_body BYTEA,
				error_message TEXT,
				PRIMARY KEY (id, timestamp)
			)
		`).Error
		if err != nil {
			return fmt.Errorf("failed to create api_call_logs table: %w", err)
		}
	}

	// Set up TimescaleDB features
	if err = initAPICallLogTable(db); err != nil {
		return fmt.Errorf("failed to setup TimescaleDB for api_call_logs: %w", err)
	}

	return nil
}

// setupAPICallLogTimescaleDB initializes the TimescaleDB specifics for the api_call_logs table
func initAPICallLogTable(db *gorm.DB) error {
	// Set up TimescaleDB hypertable
	if err := db.Exec("SELECT create_hypertable('api_call_logs', 'timestamp', if_not_exists => TRUE)").Error; err != nil {
		return fmt.Errorf("failed to create hypertable: %w", err)
	}

	// Create indexes that include the timestamp column (required for hypertables)
	indexes := []string{
		// Composite index with vendor and timestamp for efficient querying
		"CREATE INDEX IF NOT EXISTS idx_api_call_logs_vendor_timestamp ON api_call_logs (vendor, timestamp DESC)",
		// Index for timestamp alone (useful for time-based queries)
		"CREATE INDEX IF NOT EXISTS idx_api_call_logs_timestamp ON api_call_logs (timestamp DESC)",
		// Composite index for ID lookups with timestamp
		"CREATE INDEX IF NOT EXISTS idx_api_call_logs_id_timestamp ON api_call_logs (id, timestamp DESC)",
	}

	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// Set chunk time interval (optional, adjust as needed)
	if err := db.Exec("SELECT set_chunk_time_interval('api_call_logs', INTERVAL '1 day')").Error; err != nil {
		return fmt.Errorf("failed to set chunk time interval: %w", err)
	}

	return nil
}
