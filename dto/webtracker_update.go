package dto

import "time"

type WebTrackerUpdate struct {
	ID                string
	CNAMEHost         *string
	IsCNAMEConfigured *bool
	CNAMECheckCount   *uint
	CheckCNAMEAfter   *time.Time
	IsProxyActive     *bool
	LastEventAt       *time.Time
}
