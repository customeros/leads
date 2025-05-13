package repository

import (
	"context"
	"github.com/customeros/leads/internal/utils"
	"time"

	"gorm.io/gorm"

	"github.com/customeros/leads/enum"
	"github.com/customeros/leads/internal/database"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/telemetry"
)

type OutboxRepository interface {
	Create(ctx context.Context, event *models.OutboxEvent) error
	CreateWithTxn(ctx context.Context, txn *gorm.DB, event *models.OutboxEvent) error
	GetPendingEvents(ctx context.Context, limit int) ([]*models.OutboxEvent, error)
	MarkAsProcessing(ctx context.Context, id string, lockDuration time.Duration) error
	MarkAsCompleted(ctx context.Context, id string) error
	MarkAsFailed(ctx context.Context, id string, errorMessage string) error
	IncrementRetryCount(ctx context.Context, id string) error
	DeleteProcessedEvents(ctx context.Context, olderThan time.Duration) (int64, error)
}

type outboxRepository struct {
	read  *gorm.DB
	write *gorm.DB
}

func NewOutboxRepository(db *database.DbConnections) OutboxRepository {
	return &outboxRepository{
		read:  db.ReadDB,
		write: db.WriteDB,
	}
}

func (r *outboxRepository) Create(ctx context.Context, event *models.OutboxEvent) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "outboxRepository.Create")
	defer span.Finish()

	err := r.write.WithContext(ctx).Create(event).Error
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}

func (r *outboxRepository) CreateWithTxn(ctx context.Context, txn *gorm.DB, event *models.OutboxEvent) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "outboxRepository.CreateWithTxn")
	defer span.Finish()

	err := txn.WithContext(ctx).Create(event).Error
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}

func (r *outboxRepository) GetPendingEvents(ctx context.Context, limit int) ([]*models.OutboxEvent, error) {
	span, ctx := telemetry.StartPostgresSpan(ctx, "outboxRepository.GetPendingEvents")
	defer span.Finish()
	span.LogKV("limit", limit)

	var events []*models.OutboxEvent

	err := r.read.WithContext(ctx).
		Where("status = ? AND (lock_until IS NULL OR lock_until < ?)",
			enum.OutboxPending, time.Now()).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	if err != nil {
		span.TraceError(err)
		return nil, err
	}

	span.LogKV("result.count", len(events))
	return events, err
}

func (r *outboxRepository) MarkAsProcessing(ctx context.Context, id string, lockDuration time.Duration) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "outboxRepository.MarkAsProcessing")
	defer span.Finish()
	span.LogKV("id", id)

	lockUntil := time.Now().Add(lockDuration)

	result := r.write.WithContext(ctx).
		Model(&models.OutboxEvent{}).
		Where("id = ? AND (lock_until IS NULL OR lock_until < ?)", id, time.Now()).
		Updates(map[string]interface{}{
			"status":     enum.OutboxProcessing,
			"lock_until": lockUntil,
		})

	if result.Error != nil {
		span.TraceError(result.Error)
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (r *outboxRepository) MarkAsCompleted(ctx context.Context, id string) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "outboxRepository.MarkAsCompleted")
	defer span.Finish()
	span.LogKV("id", id)

	now := utils.Now()

	err := r.write.WithContext(ctx).
		Model(&models.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       enum.OutboxCompleted,
			"processed_at": now,
			"lock_until":   nil,
		}).Error

	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}

func (r *outboxRepository) MarkAsFailed(ctx context.Context, id string, errorMessage string) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "outboxRepository.MarkAsFailed")
	defer span.Finish()
	span.LogKV("errorMessage", errorMessage, "id", id)

	err := r.write.WithContext(ctx).
		Model(&models.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        enum.OutboxFailed,
			"error_message": errorMessage,
			"lock_until":    nil,
		}).Error

	if err != nil {
		span.TraceError(err)
		return err
	}
	return nil
}

func (r *outboxRepository) IncrementRetryCount(ctx context.Context, id string) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "outboxRepository.IncrementRetryCount")
	defer span.Finish()

	err := r.write.WithContext(ctx).
		Model(&models.OutboxEvent{}).
		Where("id = ?", id).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}

func (r *outboxRepository) DeleteProcessedEvents(ctx context.Context, olderThan time.Duration) (int64, error) {
	span, ctx := telemetry.StartPostgresSpan(ctx, "outboxRepository.DeleteProcessedEvents")
	defer span.Finish()

	cutoffTime := time.Now().Add(-olderThan)

	result := r.write.WithContext(ctx).
		Where("status = ? AND processed_at < ?",
			enum.OutboxCompleted, cutoffTime).
		Delete(&models.OutboxEvent{})

	if result.Error != nil {
		span.TraceError(result.Error)
		return 0, result.Error
	}

	return result.RowsAffected, nil
}
