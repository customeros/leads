package models

import (
	"time"
)

type JourneyStageContact struct {
	// Primary identification
	ID        string `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	VisitorID string `gorm:"column:lead_id;type:varchar(255);index;not null"`
	ContactID string `gorm:"column:lead_id;type:varchar(255);index;not null"`
	CompanyID string `gorm:"column:lead_id;type:varchar(255);index;not null"`

	// Session metrics
	Stage           int       `gorm:"column:pageview_count;type:integer;default:0"`
	StageConfidence float64   `gorm:"column:engagement_depth;type:float;default:0"` // Composite score
	StageEntryAt    time.Time `gorm:"column:session_started_at;type:timestamptz;not null;index"`
	CreatedAt       time.Time `gorm:"column:created_at;not null;index"` // When the event was created
}

type JourneyStageCompany struct {
	// Primary identification
	ID        string `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	CompanyID string `gorm:"column:lead_id;type:varchar(255);index;not null"`

	// Session metrics
	Stage           int       `gorm:"column:pageview_count;type:integer;default:0"`
	StageConfidence float64   `gorm:"column:engagement_depth;type:float;default:0"` // Composite score
	StageEntryAt    time.Time `gorm:"column:session_started_at;type:timestamptz;not null;index"`
	CreatedAt       time.Time `gorm:"column:created_at;not null;index"` // When the event was created
}
