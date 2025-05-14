package webtracker

import (
	"context"
	"fmt"
	"github.com/customeros/customeros/packages/server/enums"
	"net"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"

	"github.com/customeros/leads/enum"
	leads_errors "github.com/customeros/leads/errors"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/proto/pb"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
)

const (
	DEFAULT_CNAME_HOST  = "cos"
	CNAME_TARGET_DOMAIN = "custoscdn.com"
)

func (s *webtrackerService) CreateWebtracker(ctx context.Context, webtracker *models.WebTracker) (*models.WebTracker, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "websiteRegistrationService.CreateWebtracker")
	defer span.Finish()

	err := validateCreateWebtrackerRequest(webtracker)
	if err != nil {
		span.TraceError(err)
		return nil, err
	}

	err = s.validateWebtrackerDoesNotExist(ctx, webtracker.Domain)
	if err != nil {
		span.TraceError(err)
		return nil, err
	}

	webTrackerRecord, err := s.buildWebTrackerRecord(ctx, webtracker)
	if err != nil {
		span.TraceError(err)
		return nil, err
	}

	// Use transaction
	err = s.leadsWriteDB.Transaction(func(tx *gorm.DB) error {
		// Create webtracker
		err := s.repositories.WebTracker.CreateWithTxn(ctx, tx, webTrackerRecord)
		if err != nil {
			span.TraceError(err)
			return err
		}

		// Create outbox event
		eventPayload, err := s.buildOutboxEventPayload(ctx, webTrackerRecord)
		if err != nil {
			span.TraceError(err)
			return err
		}

		event := &models.OutboxEvent{
			ID:        utils.GenerateEventID(),
			EntityID:  webtracker.ID,
			EventType: enums.EventWebtrackerCreated,
			Tenant:    utils.GetTenantFromContext(ctx),
			Payload:   eventPayload,
			Publisher: enum.WebtrackerService,
			Status:    enum.OutboxPending,
			CreatedAt: utils.Now(),
		}

		err = s.repositories.Outbox.CreateWithTxn(ctx, tx, event)
		if err != nil {
			span.TraceError(err)
			return err
		}

		return nil
	})
	if err != nil {
		span.TraceError(err)
		return nil, err
	}

	return webTrackerRecord, nil
}

func (s *webtrackerService) validateWebtrackerDoesNotExist(ctx context.Context, domain string) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.validateWebtrackerDoesNotExist")
	defer span.Finish()

	record, err := s.repositories.WebTracker.GetByDomain(ctx, domain)
	if err != nil {
		span.TraceError(err)
		return err
	}
	if record != nil {
		err = leads_errors.ErrWebtrackerExists
		span.TraceError(err)
		return err
	}
	return nil
}

func (s *webtrackerService) buildOutboxEventPayload(ctx context.Context, webtracker *models.WebTracker) ([]byte, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.buildOutboxEventPayload")
	defer span.Finish()

	data, err := proto.Marshal(&pb.WebTrackerCreated{
		Id:          webtracker.ID,
		Domain:      webtracker.Domain,
		CnameHost:   webtracker.CNAMEHost,
		CnameTarget: webtracker.CNAMETarget,
	})
	if err != nil {
		span.TraceError(err)
		return nil, fmt.Errorf("failed to marchal webtracker event: %w", err)
	}
	return data, nil
}

func (s *webtrackerService) buildWebTrackerRecord(ctx context.Context, webtracker *models.WebTracker) (*models.WebTracker, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.buildWebTrackerRecord")
	defer span.Finish()

	var cnameHost string

	if webtracker.CNAMEHost == "" {
		host, err := s.generateCNAMEHost(ctx, webtracker.Domain)
		if err != nil {
			span.TraceError(err)
			return nil, err
		}
		cnameHost = host
	} else {
		cnameHost = webtracker.CNAMEHost
	}

	id := webtracker.ID
	if id == "" {
		utils.GenerateNanoIDWithPrefix("trkr", 16)
	}

	return &models.WebTracker{
		ID:                id,
		Domain:            webtracker.Domain,
		CNAMEHost:         cnameHost,
		CNAMETarget:       fmt.Sprintf("%s.%s", utils.GenerateNanoID(9), CNAME_TARGET_DOMAIN),
		IsCNAMEConfigured: false,
		CheckCNAMEAfter:   utils.NowPtr(),
		IsProxyActive:     false,
		IsArchived:        false,
		CreatedAt:         utils.Now(),
	}, nil
}

func validateCreateWebtrackerRequest(webtracker *models.WebTracker) error {
	switch {
	case webtracker == nil:
		err := errors.New("Webtracker is empty")
		return err
	case webtracker.Domain == "":
		err := errors.New("Domain not set")
		return err
	default:
		return nil
	}
}

func (s *webtrackerService) generateCNAMEHost(ctx context.Context, domain string) (string, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.generateCNAMEHost")
	defer span.Finish()
	span.LogKV("domain", domain)

	defaultDomain := DEFAULT_CNAME_HOST + "." + domain

	_, err := net.LookupCNAME(defaultDomain)
	if err != nil {
		return DEFAULT_CNAME_HOST, nil
	}

	return DEFAULT_CNAME_HOST + "-" + utils.GenerateNanoID(4), nil
}
