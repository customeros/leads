package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/customeros/leads/internal/telemetry"

	"gorm.io/gorm"

	"github.com/customeros/leads/enum"
	"github.com/customeros/leads/internal/database"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/utils"
)

// WebSessionRepository defines the interface for web session operations
type WebSessionRepository interface {
	Save(ctx context.Context, session *models.WebSession) error
	CreateWithTxn(ctx context.Context, tx *gorm.DB, session *models.WebSession) error
	GetActiveSessionsWithLookback(ctx context.Context, olderThan time.Time) ([]*models.WebSession, error)
	GetInactiveSessions(ctx context.Context) ([]*models.WebSession, error)
	UpdateLastEvent(ctx context.Context, sessionID string, event enum.Events, timestamp time.Time) error
	CloseSessionWithTxn(ctx context.Context, tx *gorm.DB, sessionID string) error
	GetActiveSessionByTrackerAndVisitor(ctx context.Context, trackerID, visitorID string) (*models.WebSession, error)
}

// Implementation errors
var (
	ErrSessionNotFound  = errors.New("session not found")
	ErrInvalidSession   = errors.New("invalid session data")
	ErrTrackerIdMissing = errors.New("tracker ID missing")
)

type webSessionRepository struct {
	read  *gorm.DB
	write *gorm.DB
}

func NewWebSessionRepository(leadsDB *database.DbConnections) WebSessionRepository {
	return &webSessionRepository{
		read:  leadsDB.ReadDB,
		write: leadsDB.WriteDB,
	}
}

// Save stores a web session
func (r *webSessionRepository) Save(ctx context.Context, session *models.WebSession) error {
	spans, ctx := telemetry.StartPostgresSpan(ctx, "webSessionRepository.Save")
	defer spans.Finish()

	if session == nil || session.ID == "" {
		return ErrInvalidSession
	}
	if session.TrackerID == "" {
		return ErrTrackerIdMissing
	}

	spans.TagEntity(session.ID)

	session.Tenant = utils.GetTenantFromContext(ctx)

	// Use context with DB
	tx := r.write.WithContext(ctx)

	// Check if the session exists
	var count int64
	err := tx.Model(&models.WebSession{}).
		Where("id = ?", session.ID).
		Count(&count).Error
	if err != nil {
		spans.TraceError(err)
		return fmt.Errorf("failed to check session existence: %w", err)
	}

	// Create or update
	if count == 0 {
		// Create
		err = tx.Create(session).Error
		if err != nil {
			spans.TraceError(err)
			return fmt.Errorf("failed to create session: %w", err)
		}
	} else {
		// Update
		err = tx.Model(&models.WebSession{}).
			Where("id = ?", session.ID).
			Updates(session).Error
		if err != nil {
			spans.TraceError(err)
			return fmt.Errorf("failed to update session: %w", err)
		}
	}
	return nil
}

func (r *webSessionRepository) CreateWithTxn(ctx context.Context, tx *gorm.DB, session *models.WebSession) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webSessionRepository.CreateWithTxn")
	defer span.Finish()

	if session == nil || session.ID == "" {
		return ErrInvalidSession
	}
	if session.TrackerID == "" {
		return ErrTrackerIdMissing
	}
	span.TagEntity(session.ID)
	session.StartedAt = utils.NowIfZero(session.StartedAt)
	session.LastEventAt = utils.NowIfZero(session.LastEventAt)

	session.Tenant = utils.GetTenantFromContext(ctx)

	err := tx.Create(session).Error
	if err != nil {
		span.TraceError(err)
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

func (r *webSessionRepository) GetActiveSessionsWithLookback(ctx context.Context, olderThan time.Time) ([]*models.WebSession, error) {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webSessionRepository.GetActiveSessionsWithLookback")
	defer span.Finish()

	var sessions []*models.WebSession

	err := r.read.WithContext(ctx).
		Where("is_active = ? AND last_event_at < ?", true, olderThan).
		Find(&sessions).Error
	if err != nil {
		span.TraceError(err)
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}

	return sessions, nil
}

func (r *webSessionRepository) GetInactiveSessions(ctx context.Context) ([]*models.WebSession, error) {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webSessionRepository.GetInactiveSessions")
	defer span.Finish()

	var sessions []*models.WebSession

	err := r.read.WithContext(ctx).
		Where("is_active = ?", false).
		Find(&sessions).Error
	if err != nil {
		span.TraceError(err)
		return nil, fmt.Errorf("failed to get inactive sessions: %w", err)
	}

	return sessions, nil
}

func (r *webSessionRepository) CloseSessionWithTxn(ctx context.Context, tx *gorm.DB, sessionID string) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webSessionRepository.CloseSessionWithTxn")
	defer span.Finish()

	result := tx.WithContext(ctx).
		Model(&models.WebSession{}).
		Where("id = ?", sessionID).
		Update("is_active", false)

	if result.Error != nil {
		span.TraceError(result.Error)
		return fmt.Errorf("failed to update session status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrSessionNotFound
	}

	return nil
}

func (r *webSessionRepository) UpdateLastEvent(ctx context.Context, sessionID string, event enum.Events, timestamp time.Time) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webSessionRepository.UpdateLastEvent")
	defer span.Finish()

	result := r.read.WithContext(ctx).
		Model(&models.WebSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"last_event": event,
			"timestamp":  timestamp,
		})

	if result.Error != nil {
		span.TraceError(result.Error)
		return fmt.Errorf("failed to update last event: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrSessionNotFound
	}

	return nil
}

func (r *webSessionRepository) GetActiveSessionByTrackerAndVisitor(ctx context.Context, trackerID, visitorID string) (*models.WebSession, error) {
	span, ctx := telemetry.StartPostgresSpan(ctx, "webSessionRepository.GetActiveSessionByTrackerAndVisitor")
	defer span.Finish()
	span.LogKV("trackerID", trackerID, "visitorID", visitorID)

	tenant := utils.GetTenantFromContext(ctx)

	var session models.WebSession

	err := r.read.WithContext(ctx).
		Where("tracker_id = ? AND visitor_id = ? AND tenant = ? AND is_active = ?",
			trackerID, visitorID, tenant, true).
		First(&session).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.LogKV("result.found", false)
			return nil, nil
		}
		span.TraceError(err)
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}

	span.LogKV("result.ID", session.ID)
	return &session, nil
}
