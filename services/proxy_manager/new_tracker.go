package proxy_manager

import (
	"context"

	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/proto/pb"
)

func (s *proxyManagerService) handleNewTrackerCreated(ctx context.Context, message *pb.WebTrackerCreated) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "proxyManagerService.handleNewTrackerCreated")
	defer span.Finish()

	return nil
}
