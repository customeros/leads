package models

import (
	"time"

	"github.com/customeros/leads/internal/enum"
)

type WebSession struct {
	// Primary identification
	ID        string `gorm:"column:id;primaryKey;type:varchar(255)"`
	TrackerID string `gorm:"column:tracker_id;type:varchar(255);index;not null"`
	Tenant    string `gorm:"column:tenant;type:varchar(255);index;not null"`
	VisitorID string `gorm:"column:visitor_id;type:varchar(255);index;not null"`
	ContactID string `gorm:"column:contact_id;type:varchar(255);index;not null"`
	CompanyID string `gorm:"column:company_id;type:varchar(255);index;not null"`
	IP        string `gorm:"column:ip;type:varchar(255)"`

	// Session metadata
	Active bool `gorm:"column:is_active;type:boolean;default:true"`

	// Session timing
	StartedAt       time.Time  `gorm:"column:started_at;type:timestamptz;not null;index"`
	LastEventAt     *time.Time `gorm:"column:last_event_at;type:timestamptz;index"`
	EndedAt         *time.Time `gorm:"column:ended_at;type:timestamptz"`
	SessionDuration int        `gorm:"column:session_duration;type:integer"` // in seconds

	// Session metrics
	PageviewCount   int         `gorm:"column:pageview_count;type:integer;default:0"`
	EngagementDepth float64     `gorm:"column:engagement_depth;type:float;default:0"` // Composite score
	LastEventType   enum.Events `gorm:"column:engagement_depth;type:float;default:0"` // Composite score

	// Entry point attribution
	EntryPage        string              `gorm:"column:entry_page;type:varchar(255)"`
	ExitPage         string              `gorm:"column:exit_page;type:varchar(255)"`
	Channel          enum.Channel        `gorm:"column:channel;type:varchar(50)"`
	Source           enum.AdPlatform     `gorm:"column:source;type:varchar(50)"`
	SourcePlatform   enum.SocialPlatform `gorm:"column:source_platform;type:varchar(50)"`
	ViewedOnPlatform enum.SocialPlatform `gorm:"column:viewed_on_platform;type:varchar(50)"`
	ReferrerDomain   string              `gorm:"column:referrer_domain;type:varchar(255)"`
	CampaignName     string              `gorm:"column:gclid;type:varchar(255)"`
	CampaignID       string              `gorm:"column:gclid;type:varchar(255)"`

	// Aggregated UTM for entry page
	UTMSource   string `gorm:"column:utm_source;type:varchar(100)"`
	UTMMedium   string `gorm:"column:utm_medium;type:varchar(100)"`
	UTMCampaign string `gorm:"column:utm_campaign;type:varchar(100)"`
	UTMContent  string `gorm:"column:utm_content;type:varchar(100)"`
	UTMTerm     string `gorm:"column:utm_term;type:varchar(100)"`

	// Journey context
	CustomerJourneyStage enum.CustomerJourneyStage `gorm:"column:journey_stage;type:varchar(50)"`
	IsSignificantSession bool                      `gorm:"column:is_significant_session;type:boolean;default:false"`

	// Context info
	DeviceType enum.DeviceType `gorm:"column:device_type;type:varchar(20)"`
	Language   string          `gorm:"column:language;type:varchar(10)"`

	// System fields
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:now()"`
}

func (WebSession) TableName() string {
	return "web_sessions"
}
