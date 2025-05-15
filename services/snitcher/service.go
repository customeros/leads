package snitcher

import (
	"context"
	"errors"
	"fmt"
	"github.com/customeros/customeros/packages/server/enums"
	"log"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

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
	subscription *nats.Subscription
}

func NewSnitcherService(config *config.SnitcherConfig, repos *repository.Repositories, natsConn *nats_internal.NATSConnections) *SnitcherService {
	return &SnitcherService{
		config:       config,
		natsConn:     natsConn,
		repositories: repos,
	}
}

var SUBSCRIBED_SUBJECT = enums.EventAskSnitcher.String()

const (
	MAX_RESPONSE_SIZE = 1 * 1024 * 1024
	QUEUE_GROUP       = "snitcher-queue-group" // Queue group for load balancing
)

// Start begins listening for events
func (s *SnitcherService) Start(ctx context.Context) error {
	requestConn, err := s.natsConn.GetNatsConnection(enums.StreamRequest)
	if err != nil {
		return fmt.Errorf("failed to get NATS connection: %w", err)
	}
	// Create a queue subscription for handling synchronous requests
	sub, err := requestConn.Conn.QueueSubscribe(SUBSCRIBED_SUBJECT, QUEUE_GROUP, func(msg *nats.Msg) {
		// First extract trace context into a new background context
		reqCtx := telemetry.ExtractTraceContextFromNatsMsg(context.Background(), msg)
		// Then add business context
		reqCtx = utils.WithCustomContextFromNats(reqCtx, msg)
		s.handleNatsMessage(reqCtx, msg)
	})
	if err != nil {
		return fmt.Errorf("failed to create queue subscription: %w", err)
	}

	// Set subscription options
	sub.SetPendingLimits(-1, -1) // No limits on pending messages
	s.subscription = sub

	// Listen for context cancellation to clean up
	go func() {
		<-ctx.Done()
		s.Unsubscribe()
	}()

	log.Println("🚀 Snitcher Service started and listening for requests (queue group: " + QUEUE_GROUP + ")")
	return nil
}

func (s *SnitcherService) handleNatsMessage(ctx context.Context, msg *nats.Msg) {
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
		return
	}

	resp = s.AskSnitcher(ctx, request.IpAddress)
	if resp == nil {
		spans.TraceError(errors.New("empty response"))
		return
	}

	s.sendResponse(ctx, msg, resp)
}

func (s *SnitcherService) sendResponse(ctx context.Context, req *nats.Msg, resp *pb.IPAddressIdentifyResponse) {
	spans, ctx := telemetry.StartServiceSpan(ctx, "SnitcherService.sendResponse")
	defer spans.Finish()

	if req.Reply == "" {
		spans.TraceError(errors.New("no reply subject provided"))
		return
	}

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

// Unsubscribe cleans up the NATS subscription
func (s *SnitcherService) Unsubscribe() {
	if s.subscription != nil {
		s.subscription.Unsubscribe()
		s.subscription = nil
		log.Println("🛑 Snitcher Service unsubscribed from NATS")
	}
}

// Stop gracefully shuts down the service
func (s *SnitcherService) Stop() {
	s.Unsubscribe()
	if s.natsConn != nil {
		s.natsConn.Close()
		log.Println("⏹️ Snitcher Service stopped")
	}
	return
}
