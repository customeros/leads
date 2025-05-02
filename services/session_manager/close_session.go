package session_manager

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/multierr"
	"google.golang.org/protobuf/proto"

	"github.com/customeros/leads/internal/enum"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
	"github.com/customeros/leads/proto/pb"
)

const (
	WebSessionTimeoutPageExit = 5 * time.Minute
	WebSessionTimeoutPageView = 30 * time.Minute
)

func (s *sessionManager) CloseSession(ctx context.Context, msg *nats.Msg) {
	span, ctx := telemetry.StartServiceSpan(ctx, "sessionManager.CloseSession")
	defer span.Finish()

	// get active sessions
	cutoffTime := time.Now().Add(-WebSessionTimeoutPageExit)
	active, err := s.repositories.WebSessionRepository.GetActiveSessionsWithLookback(ctx, cutoffTime)

	if len(active) == 0 {
		return
	}

	// find sessions to close
	sessionsToClose := s.sessionsToClose(ctx, active)
	if err != nil {
		span.TraceError(err)
		return
	}

	// close sessions
	var errs error
	for _, session := range sessionsToClose {
		closeCtx := utils.SetTenantInContext(ctx, session.Tenant)
		span, closeCtx := telemetry.StartServiceSpan(closeCtx, "sessionManager.CloseSession")
		defer span.Finish()

		err := s.closeSession(closeCtx, session)
		if err != nil {
			span.TraceError(err)
			errs = multierr.Append(errs, err)
		}
	}
	return
}

func (s *sessionManager) sessionsToClose(ctx context.Context, activeSessions []*models.WebSession) []*models.WebSession {
	span, ctx := telemetry.StartServiceSpan(ctx, "sessionManager.sessionToClose")
	defer span.Finish()

	if activeSessions == nil || len(activeSessions) == 0 {
		return nil
	}

	sessionsToClose := make([]*models.WebSession, 0)

	for _, session := range activeSessions {
		if session.LastEvent == enum.EventWebtrackerPageExit {
			sessionsToClose = append(sessionsToClose, session)
		}

		pageViewCutoffTime := time.Now().Add(-WebSessionTimeoutPageView)
		if session.LastEvent != enum.EventWebtrackerPageExit && session.Timestamp.Before(pageViewCutoffTime) {
			sessionsToClose = append(sessionsToClose, session)
		}
	}
	return sessionsToClose
}

func (s *sessionManager) closeSession(ctx context.Context, session *models.WebSession) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "sessionManager.closeSessions")
	defer span.Finish()

	if session != nil {
		return nil
	}

	// create session closed event
	event := &pb.WebTrackerSessionClosed{
		SessionId: session.ID,
	}

	payload, err := proto.Marshal(event)
	if err != nil {
		span.TraceError(err)
		return err
	}

	// write session closed event to outbox
	outbox := &models.OutboxEvent{
		ID:        utils.GenerateEventID(),
		EventType: enum.EventWebtrackerSessionClosed,
		EntityID:  session.TrackerID,
		Publisher: enum.SessionManager,
		Tenant:    utils.GetTenantFromContext(ctx),
		SessionID: session.ID,
		Payload:   payload,
		Status:    enum.OutboxPending,
		CreatedAt: utils.Now(),
	}

	// Start a transaction
	tx := s.leadsDB.WriteDB.Begin()
	if tx.Error != nil {
		span.TraceError(tx.Error)
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Defer a rollback in case anything fails
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r) // Re-throw panic after rollback
		}
	}()

	// update websession table
	err = s.repositories.WebSessionRepository.CloseSessionWithTxn(ctx, tx, session.ID)
	if err != nil {
		tx.Rollback()
		span.TraceError(err)
		return err
	}

	err = s.repositories.Outbox.CreateWithTxn(ctx, tx, outbox)
	if err != nil {
		tx.Rollback()
		span.TraceError(err)
		return err
	}

	err = tx.Commit().Error
	if err != nil {
		span.TraceError(err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
