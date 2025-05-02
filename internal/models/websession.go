package models

import (
	"time"

	"github.com/customeros/leads/internal/enum"
)

type WebSession struct {
	ID        string      `gorm:"column:id;type:varchar(50);primaryKey;not null"`
	Tenant    string      `gorm:"column:tenant;type:varchar(50);index;not null"`
	TrackerID string      `gorm:"column:tracker_id;type:varchar(50);index;not null"`
	SessionID string      `gorm:"column:session_id;type:varchar(50);index"`
	LastEvent enum.Events `gorm:"column:last_event;type:varchar(50);index;not null"`
	Timestamp time.Time   `gorm:"not null;index"`
	isActive  bool        `gorm:"column:is_active;type:bool;default:true;index;not null"`
}

// TableName overrides the table name
func (WebSession) TableName() string {
	return "web_sessions"
}
