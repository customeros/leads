package content_profiler

import (
	"context"
	"github.com/customeros/customeros/packages/server/enums"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	leads_errors "github.com/customeros/leads/errors"
	"github.com/customeros/leads/internal/models"
	nats_internal "github.com/customeros/leads/internal/nats"
	"github.com/customeros/leads/internal/proto/pb"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
)

func (s *ContentProfiler) handleWebpageScrapedEvent(ctx context.Context, msg *nats.Msg) error {
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

func (s *ContentProfiler) startWebpageProfiling(ctx context.Context, webpage *models.Content) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.profileWebpage")
	defer span.Finish()

	// content classification
	err := s.requestContentClassification(ctx, webpage)
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

func (s *ContentProfiler) requestWebpageIntentProfile(ctx context.Context, contentID string) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.requestWebpageIntentProfile")
	defer span.Finish()

	event := &pb.RequestWebpageIntent{ContentId: contentID}

	payload, err := proto.Marshal(event)
	if err != nil {
		span.TraceError(err)
		return err
	}

	// Create message with headers
	msg := nats.NewMsg(enums.EventRequestWebpageIntent.String())
	msg.Data = payload
	msg.Header.Set(nats_internal.HEADER_TENANT, utils.GetTenantFromContext(ctx))
	msg.Header.Set(nats_internal.HEADER_USERID, utils.GetUserIdFromContext(ctx))

	requestConn, err := s.natsConn.GetNatsConnection(enums.StreamRequest)
	if err != nil {
		span.TraceError(err)
		return err
	}

	_, err = requestConn.JS.PublishMsg(msg)
	if err != nil {
		span.TraceError(err)
		return err
	}
	return nil
}

func (s *ContentProfiler) requestContentClassification(ctx context.Context, webpage *models.Content) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.requestContentClassification")
	defer span.Finish()

	event := &pb.RequestWebpageClassification{
		ContentId: webpage.ID,
		Domain:    webpage.Domain,
		Url:       webpage.Url,
		Content:   webpage.Content,
	}

	payload, err := proto.Marshal(event)
	if err != nil {
		span.TraceError(err)
		return err
	}

	// Create message with headers
	msg := nats.NewMsg(enums.EventRequestWebpageClassification.String())
	msg.Data = payload
	msg.Header.Set(nats_internal.HEADER_TENANT, utils.GetTenantFromContext(ctx))
	msg.Header.Set(nats_internal.HEADER_USERID, utils.GetUserIdFromContext(ctx))

	requestConn, err := s.natsConn.GetNatsConnection(enums.StreamRequest)
	if err != nil {
		span.TraceError(err)
		return err
	}

	_, err = requestConn.JS.PublishMsg(msg)
	if err != nil {
		span.TraceError(err)
		return err
	}
	return nil
}

func (s *ContentProfiler) parseWebpageScrapedEvent(ctx context.Context, msg *nats.Msg) (*pb.WebpageScraped, error) {
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
