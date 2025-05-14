package outbox_processor

import (
	"context"
	"time"

	"github.com/customeros/leads/internal/telemetry"
)

const (
	LIMIT    = 1000
	LOOKBACK = 72 * time.Hour
)

func (s *OutboxProcessor) Cleanup(ctx context.Context) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "outboxProcessor.Cleanup")
	defer span.Finish()

	_, err := s.repositories.Outbox.DeleteProcessedEvents(ctx, LOOKBACK, LIMIT)
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}
