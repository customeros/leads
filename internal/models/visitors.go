package models

import (
	"time"

	"github.com/customeros/leads/enum"
)

type WebVisitor struct {
	// Primary identification
	ID        string `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	TrackerID string `gorm:"column:lead_id;type:varchar(255);index;not null"`
	ContactID string `gorm:"column:lead_id;type:varchar(255);index;not null"`
	CompanyID string `gorm:"column:lead_id;type:varchar(255);index;not null"`
	CookieID  string `gorm:"column:session_id;type:varchar(255);uniqueIndex"`
	IPAddress string `gorm:"column:session_id;type:varchar(255);uniqueIndex"`

	// visitor timing
	FirstVisitAt time.Time  `gorm:"column:session_started_at;type:timestamptz;not null;index"`
	LastVisitAt  *time.Time `gorm:"column:session_started_at;type:timestamptz;not null;index"`
	TotalVisits  int        `gorm:"column:session_duration;type:integer"` // in seconds

	IsIdentified bool `gorm:"column:is_significant_session;type:boolean;default:false"`

	// Context info
	DeviceType enum.DeviceType `gorm:"column:device_type;type:varchar(20)"`
	Language   string          `gorm:"column:language;type:varchar(10)"`

	// System fields
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}
