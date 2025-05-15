package content_profiler

import (
	"context"
	"errors"
	"fmt"
	"github.com/customeros/customeros/packages/server/enums"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/customeros/leads/interfaces"
	"github.com/customeros/leads/internal/database"
	nats_internal "github.com/customeros/leads/internal/nats"
	"github.com/customeros/leads/internal/proto/pb"
	"github.com/customeros/leads/internal/repository"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
)

type ContentProfiler struct {
	interfaces.NatsService
	natsConn     *nats_internal.NATSConnections
	leadsDB      *database.DbConnections
	repositories *repository.Repositories
}

func NewContentProfiler(
	natsConn *nats_internal.NATSConnections,
	leadsDB *database.DbConnections,
	repositories *repository.Repositories,
) *ContentProfiler {
	return &ContentProfiler{
		natsConn:     natsConn,
		leadsDB:      leadsDB,
		repositories: repositories,
	}
}

var SUBSCRIBED_SUBJECTS = []string{
	enums.EventWebpageScraped.String(),
	enums.EventWebpageClassified.String(),
	enums.EventWebpageProfiled.String(),
}

const (
	// queue group
	QUEUE_GROUP = "content-profiler-service"

	// consumer config
	CONSUMER_NAME         = "content-profiler-consumer"
	ACK_WAIT              = 30 * time.Second
	MAX_DELIVERY_ATTEMPTS = 5
	MAX_ACK_PENDING       = 100
	FETCH_BATCH_SIZE      = 50
	MAX_FETCH_WAIT        = 500 * time.Millisecond
	ERR_BACKOFF           = 100 * time.Millisecond
)

// Start begins listening for raw email events and processing them
func (s *ContentProfiler) Start(ctx context.Context) error {
	webNatsConn, err := s.natsConn.GetNatsConnection(enums.StreamWeb)
	if err != nil {
		return fmt.Errorf("failed to get NATS connection: %w", err)
	}
	// Create durable consumer for processing emails
	_, err = webNatsConn.JS.AddConsumer(enums.StreamWeb.String(), &nats.ConsumerConfig{
		Durable:        CONSUMER_NAME,
		DeliverGroup:   QUEUE_GROUP,
		AckPolicy:      nats.AckExplicitPolicy,
		AckWait:        ACK_WAIT,
		MaxDeliver:     MAX_DELIVERY_ATTEMPTS,
		FilterSubjects: SUBSCRIBED_SUBJECTS,
		MaxAckPending:  MAX_ACK_PENDING,
		DeliverPolicy:  nats.DeliverAllPolicy,
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	// Create pull subscription
	sub, err := webNatsConn.JS.PullSubscribe(
		">",
		CONSUMER_NAME,
		nats.Bind(enums.StreamWeb.String(), CONSUMER_NAME),
	)
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	// Start processing
	go s.processRawEvents(ctx, sub)

	return nil
}

// processRawEvents continuously processes raw email events
func (s *ContentProfiler) processRawEvents(ctx context.Context, sub *nats.Subscription) {
	log.Println("Content Profiler Service started")
	for {
		select {
		case <-ctx.Done():
			log.Println("Content Profiler Service shutting down")
			return
		default:
			s.processBatch(ctx, sub)
		}
	}
}

// processBatch fetches and processes a batch of messages
func (s *ContentProfiler) processBatch(ctx context.Context, sub *nats.Subscription) {
	// Fetch messages batch
	msgs, err := sub.Fetch(FETCH_BATCH_SIZE, nats.MaxWait(MAX_FETCH_WAIT))
	if err != nil {
		s.handleFetchError(err)
		return
	}

	for _, msg := range msgs {
		msgCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		s.routeMessage(msgCtx, msg)
		cancel()
	}
}

// handleFetchError handles errors that occur during message fetching
func (s *ContentProfiler) handleFetchError(err error) {
	if errors.Is(err, nats.ErrTimeout) {
		// No messages available, this is normal
		return
	}
	log.Printf("Fetch error: %v", err)
	time.Sleep(ERR_BACKOFF) // Small backoff on error
}

// processMessage processes a single email message
func (s *ContentProfiler) routeMessage(ctx context.Context, msg *nats.Msg) {
	ctx = utils.WithCustomContextFromNats(ctx, msg)
	spans, ctx := telemetry.StartServiceSpan(ctx, "contentProfiler.processMessage")
	defer spans.Finish()

	if msg == nil {
		spans.TraceError(errors.New("nil nats message"))
		return
	}
	spans.TagString("nats.subject", msg.Subject)

	var err error
	switch msg.Subject {
	case enums.EventWebpageScraped.String():
		err = s.handleWebpageScrapedEvent(ctx, msg)

	case enums.EventWebpageClassified.String():
		err = s.handleWebpageClassifiedEvent(ctx, msg)

	case enums.EventWebpageProfiled.String():
		err = s.handleWebpageProfiledEvent(ctx, msg)
	}

	if err != nil {
		spans.TraceError(err)
		s.handleProcessingError(ctx, msg, err)
		return
	}

	msg.Ack()
}

// handleProcessingError deals with errors during email processing
func (s *ContentProfiler) handleProcessingError(ctx context.Context, msg *nats.Msg, err error) {
	metadata, _ := msg.Metadata()

	// Check if we should retry
	if metadata.NumDelivered <= uint64(MAX_DELIVERY_ATTEMPTS) {
		// Negative acknowledgment triggers redelivery
		msg.Nak()
	} else {
		// Max retries reached, acknowledge but publish to dead letter
		msg.Ack()
		s.publishDLQ(ctx, msg, err)
	}
}

// Close gracefully shuts down the service
func (s *ContentProfiler) Stop() {
	if s.natsConn != nil {
		s.natsConn.Close()
	}
	return
}

func (s *ContentProfiler) publishDLQ(ctx context.Context, msg *nats.Msg, err error) {
	spans, ctx := telemetry.StartServiceSpan(ctx, "proxyManagerService.publishError")
	defer spans.Finish()

	errorEvent := &pb.ErrorEvent{
		Timestamp:    timestamppb.Now(),
		Subject:      msg.Subject,
		ErrorMessage: err.Error(),
		RawData:      msg.Data,
		Service:      pb.ServiceName_LEADS_PROXY_MANAGER_SERVICE, // TODO update name
	}

	data, err := proto.Marshal(errorEvent)
	if err != nil {
		spans.TraceError(err)
		log.Printf("Failed to marshal error event: %v", err)
		return
	}

	// Create message with headers
	newMsg := nats.NewMsg(nats_internal.DLQ_PREFIX + msg.Subject)
	newMsg.Data = data
	newMsg.Header.Set(nats_internal.HEADER_TENANT, utils.GetTenantFromContext(ctx))
	newMsg.Header.Set(nats_internal.HEADER_USERID, utils.GetUserIdFromContext(ctx))

	dlqConn, err := s.natsConn.GetNatsConnection(enums.StreamDLQ)
	if err != nil {
		spans.TraceError(err)
		return
	}

	// Publish to the stored subject
	_, err = dlqConn.JS.PublishMsg(newMsg)
	if err != nil {
		spans.TraceError(err)
		return
	}
}
