package models

import (
	"time"

	"github.com/lib/pq"
)

type Content struct {
	// Primary identification
	ID      string         `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	Domain  string         `gorm:"column:domain;type:varchar(255);not null"`
	Url     string         `gorm:"column:url;type:varchar(255);not null"`
	Content string         `gorm:"column:content;type:text;not null"`
	Links   pq.StringArray `gorm:"column:links;type:text[]"`

	// Content classification
	PrimaryTopic        string         `gorm:"column:primary_topic;type:varchar(255)"`
	SecondaryTopics     pq.StringArray `gorm:"column:secondary_topics;type:text[]"`
	SolutionFocus       pq.StringArray `gorm:"column:solution_focus;type:text[];index"`
	ContentType         string         `gorm:"column:content_type;type:varchar(255)"`
	IndustryVertical    string         `gorm:"column:industry_vertical;type:varchar(255)"`
	KeyPainPoints       pq.StringArray `gorm:"column:key_pain_points;type:text[]"`
	ValueProposition    string         `gorm:"column:value_proposition;type:varchar(255)"`
	ReferencedCustomers pq.StringArray `gorm:"column:referencedCustomers;type:text[]"`

	// Intent signals - using integer scoring (1-4)
	ProblemRecognitionScore int `gorm:"column:problem_recognition_score;type:smallint"`
	SolutionResearchScore   int `gorm:"column:solution_research_score;type:smallint"`
	EvaluationScore         int `gorm:"column:evaluation_score;type:smallint"`
	PurchaseReadinessScore  int `gorm:"column:purchase_readiness_score;type:smallint"`

	// Metadata
	CreatedAt    time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    *time.Time `gorm:"column:updated_at;autoUpdateTime"`
	ErrorMessage string     `gorm:"column:error_message;type:varchar(255);not null"`
}

func (Content) TableName() string {
	return "content"
}
