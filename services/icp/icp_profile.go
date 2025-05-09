package icp

import (
	"context"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"github.com/customeros/leads/internal/enum"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
	"github.com/customeros/leads/proto/pb"
)

func (s *icpService) handleICPProfileRequest(ctx context.Context, msg *nats.Msg) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "icpService.handleICPProfileRequest")
	defer span.Finish()

	// parse message
	message, err := s.parseICPProfileRequest(ctx, msg)
	if err != nil {
		span.TraceError(err)
		return err
	}

	// crawl domain
	err = s.scraperService.Crawl(ctx, message.Domain)
	if err != nil {
		span.TraceError(err)
		return err
	}

	// write website crawaled event
	err = s.repositories.ScraperEvent.Create(ctx, &models.ScraperEvent{
		Event:     enum.EventWebsiteCrawled,
		Publisher: enum.ICPService,
		Timestamp: utils.Now(),
		Domain:    message.Domain,
	})
	if err != nil {
		span.TraceError(err)
		return err
	}

	// build ICP profile
	err = s.buildICPProfile(ctx, message.Domain)
	if err != nil {
		span.TraceError(err)
		return err
	}
	return nil
}

func (s *icpService) buildICPProfile(ctx context.Context, domain string) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "icpService.buildICPProfile")
	defer span.Finish()

	// grab homepage, product pages, and case studies

	// pass to llm to build ICP

	// publish icp created event

	return nil
}

func (s *icpService) parseICPProfileRequest(ctx context.Context, msg *nats.Msg) (*pb.ICPProfileRequest, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "icpService.parseICPProfileRequest")
	defer span.Finish()

	message := &pb.ICPProfileRequest{}
	err := proto.Unmarshal(msg.Data, message)
	if err != nil {
		span.TraceError(err)
		return nil, err
	}

	return message, nil
}
