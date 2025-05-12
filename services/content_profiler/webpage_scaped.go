package content_profiler

import (
	"context"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"github.com/customeros/leads/enum"
	leads_errors "github.com/customeros/leads/errors"
	"github.com/customeros/leads/internal/models"
	nats_internal "github.com/customeros/leads/internal/nats"
	"github.com/customeros/leads/internal/proto/pb"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
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

	err = s.startWebpageProfiling(ctx, webpage)
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}

func (s *contentProfiler) startWebpageProfiling(ctx context.Context, webpage *models.Content) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.profileWebpage")
	defer span.Finish()

	// content classification
	err := s.requestContentClassification(ctx, webpage.ID)
	if err != nil {
		span.TraceError(err)
	}

	// intent signals
	err = s.requestWebpageIntentProfile(ctx, webpage.ID)
	if err != nil {
		span.TraceError(err)
	}

	return err
}

func (s *contentProfiler) requestWebpageIntentProfile(ctx context.Context, contentID string) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.requestWebpageIntentProfile")
	defer span.Finish()

	event := &pb.RequestWebpageIntent{ContentId: contentID}

	payload, err := proto.Marshal(event)
	if err != nil {
		span.TraceError(err)
		return err
	}

	// Create message with headers
	msg := nats.NewMsg(enum.EventRequestWebpageIntent.String())
	msg.Data = payload
	msg.Header.Set(nats_internal.HEADER_TENANT, utils.GetTenantFromContext(ctx))
	msg.Header.Set(nats_internal.HEADER_USERID, utils.GetUserIdFromContext(ctx))

	_, err = s.natsConn.JS.PublishMsg(msg)
	if err != nil {
		span.TraceError(err)
		return err
	}
	return nil
}

func (s *contentProfiler) requestContentClassification(ctx context.Context, contentID string) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.requestContentClassification")
	defer span.Finish()

	event := &pb.RequestWebpageClassification{ContentId: contentID}

	payload, err := proto.Marshal(event)
	if err != nil {
		span.TraceError(err)
		return err
	}

	// Create message with headers
	msg := nats.NewMsg(enum.EventRequestWebpageClassification.String())
	msg.Data = payload
	msg.Header.Set(nats_internal.HEADER_TENANT, utils.GetTenantFromContext(ctx))
	msg.Header.Set(nats_internal.HEADER_USERID, utils.GetUserIdFromContext(ctx))

	_, err = s.natsConn.JS.PublishMsg(msg)
	if err != nil {
		span.TraceError(err)
		return err
	}
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
