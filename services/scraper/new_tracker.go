package scraper

import (
	"context"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/proto/pb"
)

func (s *scraperService) handleNewTrackerCreated(ctx context.Context, msg *nats.Msg) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "scraperService.handleNewTrackerCreated")
	defer span.Finish()

	message, err := s.parseNatsMessage(ctx, msg)
	if err != nil {
		span.TraceError(err)
		return err
	}

	_, err = s.Crawl(ctx, message.Domain)
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}

func (s *scraperService) parseNatsMessage(ctx context.Context, msg *nats.Msg) (*pb.WebTrackerCreated, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "scraperService.parseNewTrackerMessage")
	defer span.Finish()

	message := &pb.WebTrackerCreated{}
	err := proto.Unmarshal(msg.Data, message)
	if err != nil {
		span.TraceError(err)
		return nil, err
	}
	return message, nil
}
