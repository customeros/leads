package leads_errors

import "errors"

var (
	ErrTenantMissing      = errors.New("Tenant not set on context")
	ErrUserIdMissing      = errors.New("UserID not set on context")
	ErrNatsMessageNil     = errors.New("Nats message is empty")
	ErrWebtrackerNotFound = errors.New("Webtracker not found")
	ErrWebtrackerExists   = errors.New("Webtracker already exists")
	ErrCNAMERecordExists  = errors.New("CNAME record already exists")
	ErrWebpageNotFound    = errors.New("Webpage not found")
)
