package scraper

import (
	"context"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"github.com/customeros/leads/internal/proto/pb"
	"github.com/customeros/leads/internal/telemetry"
)

func (s *scraperService) handleNewTrackerCreated(ctx context.Context, msg *nats.Msg) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "scraperService.handleNewTrackerCreated")
	defer span.Finish()

	message, err := s.parseNatsMessage(ctx, msg)
	if err != nil {
		span.TraceError(err)
		return err
	}

	// Launch crawl in a goroutine so we can return immediately
	go func() {
		// Create a new context since the parent will be canceled when the function returns
		backgroundCtx := context.Background()
		crawlSpan, crawlCtx := telemetry.StartServiceSpan(backgroundCtx, "scraperService.backgroundCrawl")
		defer crawlSpan.Finish()

		if err := s.Crawl(crawlCtx, message.Domain); err != nil {
			crawlSpan.TraceError(err)
		}
	}()

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
