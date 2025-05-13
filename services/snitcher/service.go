package snitcher

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"github.com/customeros/leads/enum"
	"github.com/customeros/leads/internal/config"
	nats_internal "github.com/customeros/leads/internal/nats"
	"github.com/customeros/leads/internal/proto/pb"
	"github.com/customeros/leads/internal/repository"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
)

type SnitcherService struct {
	config       *config.SnitcherConfig
	natsConn     *nats_internal.NATSConnections
	repositories *repository.Repositories
}

func NewSnitcherService(config *config.SnitcherConfig, repos *repository.Repositories, natsConn *nats_internal.NATSConnections) *SnitcherService {
	return &SnitcherService{
		config:       config,
		natsConn:     natsConn,
		repositories: repos,
	}
}

var SUBSCRIBED_SUBJECT = enum.EventAskSnitcher.String()

const (
	MAX_RESPONSE_SIZE = 1 * 1024 * 1024

	// consumer config
	CONSUMER_NAME         = "snitcher-consumer"
	ACK_WAIT              = 30 * time.Second
	MAX_DELIVERY_ATTEMPTS = 5
	MAX_ACK_PENDING       = 100
)

// Start begins listening for events using JetStream consumer
func (s *SnitcherService) Start(ctx context.Context) error {
	spans, ctx := telemetry.StartServiceSpan(ctx, "SnitcherService.Start")
	defer spans.Finish()

	// Create durable consumer for processing
	_, err := s.natsConn.JS.AddConsumer(nats_internal.LEADS_STREAM, &nats.ConsumerConfig{
		Durable:       CONSUMER_NAME,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       ACK_WAIT,
		MaxDeliver:    MAX_DELIVERY_ATTEMPTS,
		FilterSubject: SUBSCRIBED_SUBJECT,
		MaxAckPending: MAX_ACK_PENDING,
		DeliverPolicy: nats.DeliverAllPolicy,
	})
	if err != nil {
		spans.TraceError(err)
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	// Create pull subscription
	sub, err := s.natsConn.JS.PullSubscribe(
		SUBSCRIBED_SUBJECT,
		CONSUMER_NAME,
		nats.Bind(nats_internal.LEADS_STREAM, CONSUMER_NAME),
	)
	if err != nil {
		spans.TraceError(err)
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	// Start processing
	go s.processMessages(ctx, sub)

	return nil
}

// processMessages continuously processes messages one at a time
func (s *SnitcherService) processMessages(ctx context.Context, sub *nats.Subscription) {
	log.Println("Snitcher Service started")
	for {
		select {
		case <-ctx.Done():
			log.Println("Snitcher Service shutting down")
			return
		default:
			// Fetch single message with timeout
			msgs, err := sub.Fetch(1, nats.MaxWait(1*time.Second))
			if err != nil {
				if errors.Is(err, nats.ErrTimeout) {
					// No messages available, this is normal
					continue
				}
				log.Printf("Fetch error: %v", err)
				time.Sleep(100 * time.Millisecond) // Small backoff on error
				continue
			}

			// Process single message
			if len(msgs) > 0 {
				msgCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
				s.handleNatsMessage(msgCtx, msgs[0])
				cancel()
			}
		}
	}
}

// Close gracefully shuts down the service
func (s *SnitcherService) Stop() {
	if s.natsConn != nil {
		s.natsConn.Close()
	}
	return
}

func (s *SnitcherService) handleNatsMessage(ctx context.Context, msg *nats.Msg) {
	ctx = utils.WithCustomContextFromNats(ctx, msg)
	spans, ctx := telemetry.StartListenerSpan(ctx, "SnitcherService.handleNatsMessage")
	defer spans.Finish()

	if msg == nil {
		spans.TraceError(errors.New("nil nats message"))
		return
	}
	spans.TagString("nats.subject", msg.Subject)
	spans.TagString("nats.reply", msg.Reply)

	resp := &pb.IPAddressIdentifyResponse{}

	request := &pb.IPAddressIdentifyRequest{}
	err := proto.Unmarshal(msg.Data, request)
	if err != nil {
		errMsg := "Failed to parse request"
		resp.ErrorMessage = errMsg
		s.sendResponse(ctx, msg, resp)
		spans.TraceError(err)
		msg.Ack()
		return
	}

	resp = s.AskSnitcher(ctx, request.IpAddress)
	if resp == nil {
		spans.TraceError(errors.New("empty response"))
		msg.Ack()
		return
	}

	s.sendResponse(ctx, msg, resp)
	msg.Ack()
}

func (s *SnitcherService) sendResponse(ctx context.Context, req *nats.Msg, resp *pb.IPAddressIdentifyResponse) {
	spans, _ := telemetry.StartServiceSpan(ctx, "SnitcherService.sendResponse")
	defer spans.Finish()

	respMessage, err := proto.Marshal(resp)
	if err != nil {
		spans.TraceError(err)
		return
	}
	err = req.Respond(respMessage)
	if err != nil {
		spans.TraceError(err)
	}
}
