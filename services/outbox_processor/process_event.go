package outbox_processor

import (
	"context"
	"errors"
	"fmt"
	"github.com/customeros/customeros/packages/server/enums"
	"strings"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"github.com/customeros/leads/internal/models"
	nats_internal "github.com/customeros/leads/internal/nats"
	"github.com/customeros/leads/internal/proto/pb"
	"github.com/customeros/leads/internal/telemetry"
)

func (s *OutboxProcessor) processEvent(ctx context.Context, event *models.OutboxEvent) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "OutboxProcessor.processEvent")
	defer span.Finish()
	span.TagEventType(event.EventType.String())
	span.TagEntity(event.ID)

	switch {
	case strings.HasPrefix(event.EventType.String(), "webtracker"):
		return s.processWebTrackerEvent(ctx, event)

	case strings.HasPrefix(event.EventType.String(), "proxy"):
		// TODO
		return nil

	case strings.HasPrefix(event.EventType.String(), "lead"):
		// TODO
		return nil

	case strings.HasPrefix(event.EventType.String(), "webpage"):
		return s.processWebscraperEvent(ctx, event)

	default:
		err := errors.New("event type not implemented")
		span.TraceError(err)
		return err
	}
}

func (s *OutboxProcessor) processWebscraperEvent(ctx context.Context, event *models.OutboxEvent) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "OutboxProcessor.processWebscraperEvent")
	defer span.Finish()

	scrapedEvent := &pb.WebpageScraped{}
	err := proto.Unmarshal(event.Payload, scrapedEvent)
	if err != nil {
		span.TraceError(err)
		return err
	}

	eventLog := &models.ScraperEvent{
		ID:        event.ID,
		Timestamp: event.CreatedAt,
		Event:     event.EventType,
		Publisher: event.Publisher,
		Domain:    scrapedEvent.Url,
		Url:       scrapedEvent.Url,
		Payload:   event.Payload,
	}

	err = s.repositories.ScraperEvent.Create(ctx, eventLog)
	if err != nil {
		span.TraceError(err)
		return err
	}

	err = s.publishEvent(ctx, event, enums.StreamWeb)
	if err != nil {
		span.TraceError(err)
		return err
	}
	return nil
}

func (s *OutboxProcessor) processWebTrackerEvent(ctx context.Context, event *models.OutboxEvent) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "OutboxProcessor.processWebTrackerEvent")
	defer span.Finish()
	span.TagEventType(event.EventType.String())
	span.TagEntity(event.ID)

	// write event to data warehouse
	eventLog := &models.WebTrackerEvent{
		ID:        event.ID,
		Event:     event.EventType,
		Publisher: event.Publisher,
		Timestamp: event.CreatedAt,
		Tenant:    event.Tenant,
		TrackerID: event.EntityID,
		SessionID: event.SessionID,
		Payload:   event.Payload,
	}

	err := s.repositories.WebTrackerEvent.Create(ctx, eventLog)
	if err != nil {
		span.TraceError(err)
		return err
	}

	// determine if event needs to be published
	if !strings.HasPrefix(event.EventType.String(), "webtracker.event") {
		err = s.publishEvent(ctx, event, enums.StreamWebtracker)
		if err != nil {
			span.TraceError(err)
			return err
		}
	}

	return nil
}

func (s *OutboxProcessor) publishEvent(ctx context.Context, event *models.OutboxEvent, stream enums.NatsStream) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "OutboxProcessor.publishEvent")
	defer span.Finish()
	span.TagEventType(event.EventType.String())
	span.TagEntity(event.ID)

	// Create message with headers
	msg := nats.NewMsg(event.EventType.String())
	msg.Data = event.Payload
	msg.Header.Set(nats_internal.HEADER_TENANT, event.Tenant)

	streamConn, err := s.natsConn.GetNatsConnection(stream)
	if err != nil {
		span.TraceError(err)
		return fmt.Errorf("failed to get nats connection for stream %s: %w", stream, err)
	}

	// Publish to the stored subject
	_, err = streamConn.JS.PublishMsg(msg)
	if err != nil {
		span.TraceError(err)
		return fmt.Errorf("failed to publish outbox event: %w", err)
	}

	return nil
}
