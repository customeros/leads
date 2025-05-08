package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/customeros/leads/internal/telemetry"
	"time"

	"gorm.io/gorm"

	"github.com/customeros/leads/internal/database"
	"github.com/customeros/leads/internal/enum"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/utils"
)

// WebSessionRepository defines the interface for web session operations
type WebSessionRepository interface {
	Save(ctx context.Context, session *models.WebSession) error
	GetActiveSessionsWithLookback(ctx context.Context, olderThan time.Time) ([]*models.WebSession, error)
	GetInactiveSessions(ctx context.Context) ([]*models.WebSession, error)
	UpdateLastEvent(ctx context.Context, sessionID string, event enum.Events, timestamp time.Time) error
	CloseSessionWithTxn(ctx context.Context, tx *gorm.DB, sessionID string) error
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

	session.Tenant = utils.GetTenantFromContext(ctx)

	// Use context with DB
	tx := r.write.WithContext(ctx)

	// Check if the session exists
	var count int64
	err := tx.Model(&models.WebSession{}).
		Where("id = ?", session.ID).
		Count(&count).Error
	if err != nil {
		return fmt.Errorf("failed to check session existence: %w", err)
	}

	// Create or update
	if count == 0 {
		// Create
		err := tx.Create(session).Error
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
	} else {
		// Update
		err := tx.Model(&models.WebSession{}).
			Where("id = ?", session.ID).
			Updates(session).Error
		if err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}
	}
	return nil
}

func (r *webSessionRepository) GetActiveSessionsWithLookback(ctx context.Context, olderThan time.Time) ([]*models.WebSession, error) {
	var sessions []*models.WebSession

	err := r.read.WithContext(ctx).
		Where("is_active = ? AND timestamp < ?", true, olderThan).
		Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}

	return sessions, nil
}

func (r *webSessionRepository) GetInactiveSessions(ctx context.Context) ([]*models.WebSession, error) {
	var sessions []*models.WebSession

	err := r.read.WithContext(ctx).
		Where("is_active = ?", false).
		Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get inactive sessions: %w", err)
	}

	return sessions, nil
}

func (r *webSessionRepository) CloseSessionWithTxn(ctx context.Context, tx *gorm.DB, sessionID string) error {
	result := tx.WithContext(ctx).
		Model(&models.WebSession{}).
		Where("id = ?", sessionID).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to update session status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrSessionNotFound
	}

	return nil
}

func (r *webSessionRepository) UpdateLastEvent(ctx context.Context, sessionID string, event enum.Events, timestamp time.Time) error {
	result := r.read.WithContext(ctx).
		Model(&models.WebSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"last_event": event,
			"timestamp":  timestamp,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update last event: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrSessionNotFound
	}

	return nil
}
