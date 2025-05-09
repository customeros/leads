package proxy_manager

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/miekg/dns"
	"go.uber.org/multierr"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"

	"github.com/customeros/leads/dto"
	"github.com/customeros/leads/internal/enum"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
	"github.com/customeros/leads/proto/pb"
)

func (s *proxyManagerService) CheckCNAME(ctx context.Context) {
	span, ctx := telemetry.StartServiceSpan(ctx, "proxyManagerService.CheckCNAME")
	defer span.Finish()

	trackers, err := s.repositories.WebTracker.GetCNAMEChecks(ctx)
	if err != nil {
		span.TraceError(err)
		return
	}
	if trackers == nil {
		return
	}

	var errs error
	for _, tracker := range trackers {
		isActive, err := s.isCNAMEActive(ctx, tracker.CNAMEHost, tracker.Domain, tracker.CNAMETarget)
		if err != nil {
			span.TraceError(err)
			errs = multierr.Append(errs, err)
			continue
		}

		if isActive {
			err := s.handleCNAMEConfigured(ctx, &tracker)
			if err != nil {
				span.TraceError(err)
				errs = multierr.Append(errs, err)
			}
			continue
		}

		err = s.handleCNAMENotConfigured(ctx, &tracker)
		if err != nil {
			span.TraceError(err)
			errs = multierr.Append(errs, err)
		}
	}
}

func (s *proxyManagerService) isCNAMEActive(ctx context.Context, cnameHost, domain, cnameTarget string) (bool, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "proxyManagerService.isCNAMEActive")
	defer span.Finish()
	span.LogKV("cnameHost", cnameHost, "domain", domain, "cnameTarget", cnameTarget)

	dnsRecord, err := s.getDNSRecord(ctx, cnameHost, domain)
	if err != nil {
		return false, nil
	}

	// Look for CNAME records in the answer
	for _, ans := range dnsRecord {
		if cname, ok := ans.(*dns.CNAME); ok {
			// Ensure cnameTarget ends with a dot for comparison
			expectedTarget := cnameTarget
			if !strings.HasSuffix(expectedTarget, ".") {
				expectedTarget = expectedTarget + "."
			}
			matches := (cname.Target == expectedTarget)
			return matches, nil
		}
	}

	// No CNAME record found
	return false, nil
}

func (s *proxyManagerService) handleCNAMEConfigured(ctx context.Context, tracker *models.WebTracker) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "proxyManagerService.handleCNAMEConfigured")
	defer span.Finish()

	// create outbox event
	event := &pb.ProxyWebtrackerCnameConfigured{
		Tenant:      tracker.Tenant,
		TrackerId:   tracker.ID,
		CnameHost:   tracker.CNAMEHost,
		CnameTarget: tracker.CNAMETarget,
	}

	payload, err := proto.Marshal(event)
	if err != nil {
		span.TraceError(err)
		return err
	}

	// write to db in single txn
	err = s.leadsDB.WriteDB.Transaction(func(tx *gorm.DB) error {
		err := s.repositories.WebTracker.CNAMEConfiguredWithTxn(ctx, tx, tracker.ID)
		if err != nil {
			span.TraceError(err)
			return err
		}

		outboxEvent := &models.OutboxEvent{
			ID:        utils.GenerateEventID(),
			EntityID:  tracker.ID,
			EventType: enum.EventProxyWebtrackerCnameConfigured,
			Tenant:    tracker.Tenant,
			Payload:   payload,
			Publisher: enum.ProxyManager,
			Status:    enum.OutboxPending,
			CreatedAt: utils.Now(),
		}

		err = s.repositories.Outbox.CreateWithTxn(ctx, tx, outboxEvent)
		if err != nil {
			span.TraceError(err)
			return err
		}

		return nil
	})
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}

func (s *proxyManagerService) handleCNAMENotConfigured(ctx context.Context, tracker *models.WebTracker) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "proxyManagerService.handleCNAMENotConfigured")
	defer span.Finish()

	checkCount := tracker.CNAMECheckCount + 1
	isConfigured := false

	updateRecord := dto.WebTrackerUpdate{
		ID:                tracker.ID,
		IsCNAMEConfigured: &isConfigured,
		CNAMECheckCount:   &checkCount,
		CheckCNAMEAfter:   calculateNextCNAMECheck(checkCount),
	}

	return s.repositories.WebTracker.Update(ctx, updateRecord)
}

func (s *proxyManagerService) getDNSRecord(ctx context.Context, cnameHost, domain string) ([]dns.RR, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "proxyManagerService.getCNAMERecord")
	defer span.Finish()
	span.LogKV("cnameHost", cnameHost, "domain", domain)

	c := dns.Client{}
	m := dns.Msg{}

	// Ensure subdomain ends with a dot
	host := fmt.Sprintf("%s.%s.", cnameHost, domain)
	span.LogKV("host", host)

	// Set up the DNS query for CNAME
	m.SetQuestion(host, dns.TypeCNAME)
	m.RecursionDesired = true

	// Use Cloudflare's DNS server
	r, _, err := c.Exchange(&m, "1.1.1.1:53")
	if err != nil {
		span.TraceError(err)
		return nil, fmt.Errorf("DNS query failed: %w", err)
	}

	span.LogKV("result.Rcode", r.Rcode)
	if r.Rcode != dns.RcodeSuccess {
		err = fmt.Errorf("DNS query returned non-success code: %d", r.Rcode)
		return nil, err
	}

	return r.Answer, nil
}

func calculateNextCNAMECheck(tryCount uint) *time.Time {
	baseDelay := time.Minute
	maxDelay := 24 * time.Hour

	var delay time.Duration
	if tryCount > 10 {
		delay = maxDelay
	} else {
		// 2^tryCount * baseDelay
		multiplier := math.Pow(2, float64(tryCount))
		delay = time.Duration(multiplier) * baseDelay

		// Cap at max delay
		if delay > maxDelay {
			delay = maxDelay
		}
	}

	// Calculate the future time
	backoffTime := time.Now().Add(delay)

	return &backoffTime
}
