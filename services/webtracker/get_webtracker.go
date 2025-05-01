package webtracker

import (
	"context"

	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/telemetry"
)

func (s *webtrackerService) GetWebtrackerByOrigin(ctx context.Context, origin string) (*models.WebTracker, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.GetWebtrackerByOrigin")
	defer span.Finish()

	return s.repositories.WebTracker.GetByDomain(ctx, origin)
}
