package content_profiler

import (
	"context"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	leads_errors "github.com/customeros/leads/errors"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/proto/pb"
	"github.com/customeros/leads/internal/telemetry"
)

func (s *contentProfiler) handleWebpageClassifiedEvent(ctx context.Context, msg *nats.Msg) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.handleWebpageClassifiedEvent")
	defer span.Finish()

	// parse nats message
	if msg == nil {
		err := leads_errors.ErrNatsMessageNil
		span.TraceError(err)
		return err
	}
	message, err := s.parseWebpageClassifiedEvent(ctx, msg)
	if err != nil {
		span.TraceError(err)
		return err
	}

	// update record
	err = s.repositories.Content.Update(ctx, &models.Content{
		ID:                  message.ContentId,
		PrimaryTopic:        message.PrimaryTopic,
		SecondaryTopics:     message.SecondaryTopics,
		SolutionFocus:       message.SolutionFocus,
		ContentType:         message.ContentType,
		IndustryVertical:    message.IndustryVertical,
		KeyPainPoints:       message.KeyPainPoints,
		ValueProposition:    message.ValueProposition,
		ReferencedCustomers: message.ReferencedCustomers,
	})
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}

func (s *contentProfiler) parseWebpageClassifiedEvent(ctx context.Context, msg *nats.Msg) (*pb.WebpageClassified, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.parseWebpageClassifiedEvent")
	defer span.Finish()

	result := &pb.WebpageClassified{}
	err := proto.Unmarshal(msg.Data, result)
	if err != nil {
		span.TraceError(err)
		return nil, err
	}

	return result, nil
}
