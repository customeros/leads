package interfaces

import (
	"context"

	"github.com/customeros/leads/proto/pb"
)

type WebEventProcessor interface {
	Process(ctx context.Context, webtrackerID string, event *pb.WebTrackerEvent)
}
