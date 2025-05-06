package models

import (
	"time"
)

type TopicInterestContact struct {
	// Primary identification
	ID        string `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	VisitorID string `gorm:"column:lead_id;type:varchar(255);index;not null"`
	ContactID string `gorm:"column:lead_id;type:varchar(255);index;not null"`
	CompanyID string `gorm:"column:lead_id;type:varchar(255);index;not null"`

	// Session timing
	FirstInterestAt time.Time  `gorm:"column:session_started_at;type:timestamptz;not null;index"`
	LastInterestAt  *time.Time `gorm:"column:session_started_at;type:timestamptz;not null;index"`

	// Session metrics
	Topic              int       `gorm:"column:pageview_count;type:integer;default:0"`
	InterestConfidence float64   `gorm:"column:engagement_depth;type:float;default:0"` // Composite score
	CreatedAt          time.Time `gorm:"column:created_at;not null;index"`             // When the event was created
}

type TopicInterestCompany struct {
	// Primary identification
	ID        string `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	VisitorID string `gorm:"column:lead_id;type:varchar(255);index;not null"`
	ContactID string `gorm:"column:lead_id;type:varchar(255);index;not null"`
	CompanyID string `gorm:"column:lead_id;type:varchar(255);index;not null"`

	// Session timing
	FirstInterestAt time.Time  `gorm:"column:session_started_at;type:timestamptz;not null;index"`
	LastInterestAt  *time.Time `gorm:"column:session_started_at;type:timestamptz;not null;index"`

	// Session metrics
	Topic              int       `gorm:"column:pageview_count;type:integer;default:0"`
	InterestConfidence float64   `gorm:"column:engagement_depth;type:float;default:0"` // Composite score
	CreatedAt          time.Time `gorm:"column:created_at;not null;index"`             // When the event was created
}
