package session_manager

import (
	"context"
	"time"

	"go.uber.org/multierr"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"

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

func (s *sessionManager) ProcessActiveSessions(ctx context.Context) {
	spans, ctx := telemetry.StartServiceSpan(ctx, "sessionManager.ProcessActiveSessions")
	defer spans.Finish()

	// get active sessions
	cutoffTime := time.Now().Add(-WebSessionTimeoutPageExit)
	active, err := s.repositories.WebSessionRepository.GetActiveSessionsWithLookback(ctx, cutoffTime)

	if len(active) == 0 {
		spans.LogKV("result", "No active sessions to process")
		return
	}

	// find sessions to close
	sessionsToClose := s.sessionsToClose(ctx, active)
	if err != nil {
		spans.TraceError(err)
		return
	}
	spans.LogKV("result.sessionsToClose.count", len(sessionsToClose))

	// close sessions
	var errs error
	for _, session := range sessionsToClose {
		closeCtx := utils.SetTenantInContext(ctx, session.Tenant)
		err = s.closeSession(closeCtx, session)
		if err != nil {
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
		if session.LastEventType == enum.EventWebtrackerPageExit {
			sessionsToClose = append(sessionsToClose, session)
		}

		pageViewCutoffTime := time.Now().Add(-WebSessionTimeoutPageView)
		if session.LastEventType != enum.EventWebtrackerPageExit && session.LastEventAt.Before(pageViewCutoffTime) {
			sessionsToClose = append(sessionsToClose, session)
		}
	}
	return sessionsToClose
}

func (s *sessionManager) closeSession(ctx context.Context, session *models.WebSession) error {
	spans, ctx := telemetry.StartServiceSpan(ctx, "sessionManager.closeSession")
	defer spans.Finish()

	if session == nil {
		return nil
	}

	// create session closed event
	event := &pb.WebTrackerSessionClosed{
		SessionId: session.ID,
	}

	payload, err := proto.Marshal(event)
	if err != nil {
		spans.TraceError(err)
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
	err = s.leadsDB.WriteDB.Transaction(func(tx *gorm.DB) error {
		// update websession table
		err = s.repositories.WebSessionRepository.CloseSessionWithTxn(ctx, tx, session.ID)
		if err != nil {
			spans.TraceError(err)
			return err
		}

		err = s.repositories.Outbox.CreateWithTxn(ctx, tx, outbox)
		if err != nil {
			spans.TraceError(err)
			return err
		}
		return nil
	})
	if err != nil {
		spans.TraceError(err)
		return err
	}

	return nil
}
