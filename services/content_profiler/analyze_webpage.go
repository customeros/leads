package content_profiler

import (
	"context"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	leads_errors "github.com/customeros/leads/errors"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/proto/pb"
)

func (s *contentProfiler) handleWebpageScrapedEvent(ctx context.Context, msg *nats.Msg) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.handleWebpageScrapedEvent")
	defer span.Finish()

	// parse nats message
	if msg == nil {
		err := leads_errors.ErrNatsMessageNil
		span.TraceError(err)
		return err
	}
	message, err := s.parseWebpageScrapedEvent(ctx, msg)
	if err != nil {
		span.TraceError(err)
		return err
	}

	// get webpage content
	webpage, err := s.repositories.Content.GetByUrl(ctx, message.Url)
	if err != nil {
		span.TraceError(err)
		return err
	}
	if webpage == nil {
		err := leads_errors.ErrWebpageNotFound
		span.TraceError(err)
		return err
	}

	err = s.profileWebpage(ctx, webpage)
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}

func (s *contentProfiler) profileWebpage(ctx context.Context, webpage *models.Content) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.profileWebpage")
	defer span.Finish()

	return nil
}

func (s *contentProfiler) parseWebpageScrapedEvent(ctx context.Context, msg *nats.Msg) (*pb.WebpageScraped, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.parseWebpageScapedEvent")
	defer span.Finish()

	message := &pb.WebpageScraped{}
	err := proto.Unmarshal(msg.Data, message)
	if err != nil {
		span.TraceError(err)
		return nil, err
	}
	return message, nil
}
