package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/customeros/leads/dto"
	leads_errors "github.com/customeros/leads/errors"
	"github.com/customeros/leads/internal/database"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
)

type WebTrackerRepository interface {
	Create(ctx context.Context, tracker *models.WebTracker) error
	CreateWithTxn(ctx context.Context, txn *gorm.DB, tracker *models.WebTracker) error
	GetByID(ctx context.Context, id string) (*models.WebTracker, error)
	GetByDomain(ctx context.Context, id string) (*models.WebTracker, error)
	GetActiveTrackers(ctx context.Context) ([]models.WebTracker, error)
	Update(ctx context.Context, record dto.WebTrackerUpdate) error
	UpdateLastEventAt(ctx context.Context, trackerID string, timestamp time.Time) error
	UpdateLastEventAtWithTxn(ctx context.Context, txn *gorm.DB, trackerID string, timestamp time.Time) error
	Archive(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) error
	GetCNAMEChecks(ctx context.Context) ([]models.WebTracker, error)
	CNAMEConfiguredWithTxn(ctx context.Context, txn *gorm.DB, id string) error
}

// GormWebTrackerRepository implements WebTrackerRepository using GORM
type webTrackerRepository struct {
	read  *gorm.DB
	write *gorm.DB
}

// NewWebTrackerRepository creates a new WebTracker repository
func NewWebTrackerRepository(db *database.DbConnections) WebTrackerRepository {
	return &webTrackerRepository{
		read:  db.ReadDB,
		write: db.WriteDB,
	}
}

// Create inserts a new WebTracker
func (r *webTrackerRepository) Create(ctx context.Context, tracker *models.WebTracker) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webTrackerRepository.Create")
	defer span.Finish()

	tenant := utils.GetTenantFromContext(ctx)
	if tenant == "" {
		err := leads_errors.ErrTenantMissing
		span.TraceError(err)
		return err
	}
	tracker.Tenant = tenant

	if tracker.ID == "" {
		tracker.ID = uuid.New().String()
	}

	result := r.write.WithContext(ctx).Create(tracker)
	return result.Error
}

func (r *webTrackerRepository) CreateWithTxn(ctx context.Context, txn *gorm.DB, tracker *models.WebTracker) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webTrackerRepository.Create")
	defer span.Finish()

	tenant := utils.GetTenantFromContext(ctx)
	if tenant == "" {
		err := leads_errors.ErrTenantMissing
		span.TraceError(err)
		return err
	}
	tracker.Tenant = tenant

	// Generate ID if not provided
	if tracker.ID == "" {
		tracker.ID = uuid.New().String()
	}

	return txn.Create(tracker).Error
}

// GetByID retrieves a WebTracker by ID
func (r *webTrackerRepository) GetByID(ctx context.Context, id string) (*models.WebTracker, error) {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webTrackerRepository.GetByID")
	defer span.Finish()

	tenant := utils.GetTenantFromContext(ctx)
	if tenant == "" {
		err := leads_errors.ErrTenantMissing
		span.TraceError(err)
		return nil, err
	}

	var tracker models.WebTracker
	result := r.read.WithContext(ctx).
		Where("id = ?", id).
		Where("tenant = ?", tenant).
		Where("is_archived = ?", false).
		First(&tracker)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, leads_errors.ErrWebtrackerNotFound
		}
		return nil, result.Error
	}
	return &tracker, nil
}

func (r *webTrackerRepository) GetByDomain(ctx context.Context, domain string) (*models.WebTracker, error) {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webTrackerRepository.GetByDomain")
	defer span.Finish()

	var tracker models.WebTracker
	result := r.read.WithContext(ctx).
		Where("domain = ?", domain).
		First(&tracker)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &tracker, nil
}

// GetActiveTrackers retrieves all active non-archived WebTrackers
func (r *webTrackerRepository) GetActiveTrackers(ctx context.Context) ([]models.WebTracker, error) {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webTrackerRepository.GetActiveTrackers")
	defer span.Finish()

	tenant := utils.GetTenantFromContext(ctx)
	if tenant == "" {
		err := leads_errors.ErrTenantMissing
		span.TraceError(err)
		return nil, err
	}

	var trackers []models.WebTracker
	result := r.read.WithContext(ctx).
		Where("tenant = ?", tenant).
		Where("is_archived = ?", false).
		Where("is_proxy_active = ?", true).
		Find(&trackers)
	return trackers, result.Error
}

// Update updates an existing WebTracker
func (r *webTrackerRepository) Update(ctx context.Context, dto dto.WebTrackerUpdate) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webTrackerRepository.Update")
	defer span.Finish()

	if dto.ID == "" {
		return errors.New("tracker ID cannot be empty")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	if dto.CNAMEHost != nil {
		updates["cname_host"] = *dto.CNAMEHost
	}

	if dto.LastEventAt != nil {
		updates["last_event_at"] = *dto.LastEventAt
	}

	if dto.IsCNAMEConfigured != nil {
		updates["is_cname_configured"] = *dto.IsCNAMEConfigured
	}

	if dto.IsProxyActive != nil {
		updates["is_proxy_active"] = *dto.IsProxyActive
	}

	if dto.CNAMECheckCount != nil {
		updates["cname_check_count"] = dto.CNAMECheckCount
	}

	if dto.CheckCNAMEAfter != nil {
		updates["check_cname_after"] = dto.CheckCNAMEAfter
	}

	result := r.write.WithContext(ctx).
		Model(&models.WebTracker{}).
		Where("id = ?", dto.ID).
		Updates(updates)

	if result.RowsAffected == 0 {
		return errors.New("webtracker not found")
	}
	return result.Error
}

// UpdateLastEventAt updates the LastEventAt timestamp for a specific tracker
func (r *webTrackerRepository) UpdateLastEventAt(ctx context.Context, trackerID string, timestamp time.Time) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webTrackerRepository.UpdateLastEventAt")
	defer span.Finish()

	result := r.write.WithContext(ctx).
		Model(&models.WebTracker{}).
		Where("id = ?", trackerID).
		Update("last_event_at", timestamp)

	if result.Error != nil {
		span.TraceError(result.Error)
		return result.Error
	}

	if result.RowsAffected == 0 {
		err := fmt.Errorf("no tracker found with ID: %s", trackerID)
		span.TraceError(err)
		return err
	}

	return nil
}

func (r *webTrackerRepository) UpdateLastEventAtWithTxn(ctx context.Context, txn *gorm.DB, trackerID string, timestamp time.Time) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webTrackerRepository.UpdateLastEventAtWithTxn")
	defer span.Finish()

	result := txn.Model(&models.WebTracker{}).
		Where("id = ?", trackerID).
		Update("last_event_at", timestamp)

	if result.Error != nil {
		span.TraceError(result.Error)
		return result.Error
	}

	if result.RowsAffected == 0 {
		err := fmt.Errorf("no tracker found with ID: %s", trackerID)
		span.TraceError(err)
		return err
	}

	return nil
}

// Archive marks a WebTracker as archived
func (r *webTrackerRepository) Archive(ctx context.Context, id string) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webTrackerRepository.Archive")
	defer span.Finish()

	tenant := utils.GetTenantFromContext(ctx)
	if tenant == "" {
		err := leads_errors.ErrTenantMissing
		span.TraceError(err)
		return err
	}

	now := time.Now()
	result := r.write.WithContext(ctx).
		Model(&models.WebTracker{}).
		Where("id = ?", id).
		Where("tenant = ?", tenant).
		Updates(map[string]interface{}{
			"is_archived":     true,
			"is_proxy_active": false,
			"updated_at":      now,
		})

	if result.RowsAffected == 0 {
		return errors.New("webtracker not found")
	}
	return result.Error
}

// Restore unarchives a WebTracker
func (r *webTrackerRepository) Restore(ctx context.Context, id string) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webTrackerRepository.Restore")
	defer span.Finish()

	now := time.Now()
	result := r.write.WithContext(ctx).
		Model(&models.WebTracker{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_archived": false,
			"updated_at":  now,
		})

	if result.RowsAffected == 0 {
		return errors.New("webtracker not found")
	}
	return result.Error
}

func (r *webTrackerRepository) GetCNAMEChecks(ctx context.Context) ([]models.WebTracker, error) {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webTrackerRepository.GetPendingCNAMEChecks")
	defer span.Finish()

	var trackers []models.WebTracker

	now := time.Now()

	err := r.read.WithContext(ctx).
		Where("is_cname_configured = ?", false).
		Where("(check_cname_after <= ?)", now).
		Where("is_archived = ?", false).
		Find(&trackers).Error
	if err != nil {
		return nil, err
	}

	return trackers, nil
}

func (r *webTrackerRepository) CNAMEConfiguredWithTxn(ctx context.Context, txn *gorm.DB, id string) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webTrackerRepository.CNAMEConfigured")
	defer span.Finish()

	result := txn.Model(&models.WebTracker{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_cname_configured": true,
			"check_cname_after":   nil,
			"cname_check_count":   nil,
		})

	if result.RowsAffected == 0 {
		return errors.New("webtracker not found")
	}
	return result.Error
}
