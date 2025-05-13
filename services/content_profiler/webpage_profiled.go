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

func (s *ContentProfiler) handleWebpageProfiledEvent(ctx context.Context, msg *nats.Msg) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.handleWebpageProfiledEvent")
	defer span.Finish()

	// parse nats message
	if msg == nil {
		err := leads_errors.ErrNatsMessageNil
		span.TraceError(err)
		return err
	}
	message, err := s.parseWebpageProfiledEvent(ctx, msg)
	if err != nil {
		span.TraceError(err)
		return err
	}

	// update record
	err = s.repositories.Content.UpdateIntentScores(ctx, &models.Content{
		ID:                      message.ContentId,
		ProblemRecognitionScore: int(message.ProblemRecognitionScore),
		SolutionResearchScore:   int(message.SolutionResearchScore),
		EvaluationScore:         int(message.EvaluationScore),
		PurchaseReadinessScore:  int(message.PurchaseReadinessScore),
	})
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}

func (s *ContentProfiler) parseWebpageProfiledEvent(ctx context.Context, msg *nats.Msg) (*pb.WebpageProfiled, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.parseWebpageProfiledEvent")
	defer span.Finish()

	result := &pb.WebpageProfiled{}
	err := proto.Unmarshal(msg.Data, result)
	if err != nil {
		span.TraceError(err)
		return nil, err
	}

	return result, nil
}
