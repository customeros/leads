package webtracker

import (
	"context"
	"github.com/pkg/errors"

	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/telemetry"
)

func (s *webtrackerService) IsCNAMEConfigured(ctx context.Context, webtrackerID string) (bool, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.IsCNAMEConfigured")
	defer span.Finish()

	trackerRecord, err := s.repositories.WebTracker.GetByID(ctx, webtrackerID)
	if err != nil {
		span.TraceError(err)
		return false, err
	}
	if trackerRecord == nil {
		err := errors.New("Cannot find webtracker")
		span.TraceError(err)
		return false, err
	}

	return trackerRecord.IsCNAMEConfigured, nil
}

func (s *webtrackerService) GetWebtrackerByOrigin(ctx context.Context, origin string) (*models.WebTracker, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.GetWebtrackerByOrigin")
	defer span.Finish()

	return s.repositories.WebTracker.GetByDomain(ctx, origin)
}

func (s *webtrackerService) GetWebtracker(ctx context.Context, webtrackerID string) (*models.WebTracker, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.GetWebtracker")
	defer span.Finish()

	trackerRecord, err := s.repositories.WebTracker.GetByID(ctx, webtrackerID)
	if err != nil {
		span.TraceError(err)
		return nil, err
	}
	if trackerRecord == nil {
		err := errors.New("Cannot find webtracker")
		span.TraceError(err)
		return nil, err
	}
	return trackerRecord, nil
}

func (s *webtrackerService) GetActiveWebtrackers(ctx context.Context) ([]models.WebTracker, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.GetActiveWebtrackers")
	defer span.Finish()

	return s.repositories.WebTracker.GetActiveTrackers(ctx)
}

func (s *webtrackerService) GetWebtrackers(ctx context.Context) ([]models.WebTracker, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.GetWebtrackers")
	defer span.Finish()

	return s.repositories.WebTracker.GetTrackers(ctx)
}
