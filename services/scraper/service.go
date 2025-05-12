package scraper

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/customeros/leads/enum"
	"github.com/customeros/leads/internal/config"
	"github.com/customeros/leads/internal/database"
	nats_internal "github.com/customeros/leads/internal/nats"
	"github.com/customeros/leads/internal/proto/pb"
	"github.com/customeros/leads/internal/repository"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
)

type ScraperService interface {
	Crawl(ctx context.Context, domain string) error
}

type scraperService struct {
	config       *config.JinaConfig
	natsConn     *nats_internal.NATSConnections
	leadsDB      *database.DbConnections
	repositories *repository.Repositories
	visitedURLs  sync.Map
	limiter      chan struct{}
}

func NewScraperService(
	config *config.JinaConfig,
	natsConn *nats_internal.NATSConnections,
	leadsDB *database.DbConnections,
	repositories *repository.Repositories,
) ScraperService {
	return &scraperService{
		config:       config,
		natsConn:     natsConn,
		leadsDB:      leadsDB,
		repositories: repositories,
		limiter:      make(chan struct{}, 5),
	}
}

var SUBSCRIBED_SUBJECT = enum.EventWebtrackerCreated.String()

const (
	// queue group
	QUEUE_GROUP = "scraper-service"

	// consumer config
	CONSUMER_NAME         = "scraper-consumer"
	ACK_WAIT              = 30 * time.Second
	MAX_DELIVERY_ATTEMPTS = 5
	MAX_ACK_PENDING       = 100
	FETCH_BATCH_SIZE      = 50
	MAX_FETCH_WAIT        = 500 * time.Millisecond
	ERR_BACKOFF           = 100 * time.Millisecond
)

// Start begins listening for raw email events and processing them
func (s *scraperService) Start(ctx context.Context) error {
	// Create durable consumer for processing emails
	_, err := s.natsConn.JS.AddConsumer(nats_internal.LEADS_STREAM, &nats.ConsumerConfig{
		Durable:       CONSUMER_NAME,
		DeliverGroup:  QUEUE_GROUP,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       ACK_WAIT,
		MaxDeliver:    MAX_DELIVERY_ATTEMPTS,
		FilterSubject: SUBSCRIBED_SUBJECT,
		MaxAckPending: MAX_ACK_PENDING,
		DeliverPolicy: nats.DeliverAllPolicy,
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	// Create pull subscription
	sub, err := s.natsConn.JS.PullSubscribe(
		SUBSCRIBED_SUBJECT,
		CONSUMER_NAME,
		nats.Bind(nats_internal.LEADS_STREAM, CONSUMER_NAME),
	)
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	// Start processing
	go s.processRawEvents(ctx, sub)

	return nil
}

// processRawEvents continuously processes raw email events
func (s *scraperService) processRawEvents(ctx context.Context, sub *nats.Subscription) {
	log.Println("Scraper Service started")
	for {
		select {
		case <-ctx.Done():
			log.Println("Scraper Service shutting down")
			return
		default:
			s.processBatch(ctx, sub)
		}
	}
}

// processBatch fetches and processes a batch of messages
func (s *scraperService) processBatch(ctx context.Context, sub *nats.Subscription) {
	// Fetch messages batch
	msgs, err := sub.Fetch(FETCH_BATCH_SIZE, nats.MaxWait(MAX_FETCH_WAIT))
	if err != nil {
		s.handleFetchError(err)
		return
	}

	for _, msg := range msgs {
		msgCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		s.processMessage(msgCtx, msg)
		cancel()
	}
}

// handleFetchError handles errors that occur during message fetching
func (s *scraperService) handleFetchError(err error) {
	if errors.Is(err, nats.ErrTimeout) {
		// No messages available, this is normal
		return
	}
	log.Printf("Fetch error: %v", err)
	time.Sleep(ERR_BACKOFF) // Small backoff on error
}

// processMessage processes a single email message
func (s *scraperService) processMessage(ctx context.Context, msg *nats.Msg) {
	ctx = utils.WithCustomContextFromNats(ctx, msg)
	spans, ctx := telemetry.StartServiceSpan(ctx, "scraperService.processMessage")
	defer spans.Finish()

	if msg == nil {
		spans.TraceError(errors.New("nil nats message"))
		return
	}
	spans.TagString("nats.subject", msg.Subject)

	// Process the email
	err := s.handleNewTrackerCreated(ctx, msg)
	if err != nil {
		if !strings.Contains(err.Error(), "skipping") {
			spans.TraceError(err)
		}
		s.handleProcessingError(ctx, msg, err)
		return
	}

	msg.Ack()
	return
}

// handleProcessingError deals with errors during email processing
func (s *scraperService) handleProcessingError(ctx context.Context, msg *nats.Msg, err error) {
	metadata, _ := msg.Metadata()

	// Check if we should retry
	if metadata.NumDelivered <= uint64(MAX_DELIVERY_ATTEMPTS) {
		// Negative acknowledgment triggers redelivery
		msg.Nak()
	} else {
		// Max retries reached, acknowledge but publish to dead letter
		msg.Ack()
		s.publishError(ctx, msg, err)
	}
}

// Close gracefully shuts down the service
func (s *scraperService) Stop() {
	if s.natsConn != nil {
		s.natsConn.Close()
	}
	return
}

func (s *scraperService) publishError(ctx context.Context, msg *nats.Msg, err error) {
	spans, ctx := telemetry.StartServiceSpan(ctx, "scraperService.publishError")
	defer spans.Finish()

	errorEvent := &pb.ErrorEvent{
		Timestamp:    timestamppb.Now(),
		Subject:      msg.Subject,
		ErrorMessage: err.Error(),
		RawData:      msg.Data,
		Service:      pb.ServiceName_LEADS_PROXY_MANAGER_SERVICE,
	}

	data, err := proto.Marshal(errorEvent)
	if err != nil {
		spans.TraceError(err)
		log.Printf("Failed to marshal error event: %v", err)
		return
	}

	// Create message with headers
	newMsg := nats.NewMsg(enum.EventLeadError.String())
	newMsg.Data = data
	newMsg.Header.Set(nats_internal.HEADER_TENANT, utils.GetTenantFromContext(ctx))
	newMsg.Header.Set(nats_internal.HEADER_USERID, utils.GetUserIdFromContext(ctx))

	// Publish to the stored subject
	_, err = s.natsConn.JS.PublishMsg(newMsg)
	if err != nil {
		spans.TraceError(err)
		return
	}
}
