package session_analyzer

import (
	"context"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"github.com/customeros/leads/enum"
	"github.com/customeros/leads/internal/proto/pb"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
)

func (s *sessionAnalyzer) AnalyzeSession(ctx context.Context, msg *nats.Msg) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "sessionAnalyzer.AnalyzeSession")
	defer span.Finish()

	message := s.getNatsMessage(ctx, msg)

	return nil
}

func (s *sessionAnalyzer) getNatsMessage(ctx context.Context, msg *nats.Msg) (*pb.WebTrackerSessionClosed, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "sessionAnalyzer.getNatsMessage")
	defer span.Finish()

	message := &pb.WebTrackerSessionClosed{}
	err := proto.Unmarshal(msg.Data, message)
	if err != nil {
		span.TraceError(err)
		return nil, err
	}
	return message, nil
}

func (s *sessionAnalyzer) determineLeadSource(ctx context.Context, href, referrer string) (LeadSource, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "sessionManager.determineLeadSource")
	defer span.Finish()

	leadSource := LeadSource{}

	if referrer == "" {
		leadSource.Channel = enum.ChannelDirect
		return leadSource, nil
	}

	parsedReferrer, err := utils.ParseURL(referrer)
	if err != nil {
		span.TraceError(err)
		return leadSource, nil
	}

	isSearch, searchEngine := isSearchEngine(parsedReferrer.Host)
	isSocial, socialPlatform := isSocialPlatform(parsedReferrer)
	isEmail := isEmail(parsedReferrer.Host)

	parsedHref, err := utils.ParseURL(href)
	if err != nil {
		span.TraceError(err)
		return leadSource, nil
	}

	campaignMetadata := parseCampaignMetadata(parsedHref)

	isPaid := campaignMetadata != nil && campaignMetadata.IsPaid

	switch {
	case isSearch && isPaid:
		leadSource.AdPlatform = campaignMetadata.AdPlatform
		leadSource.Channel = enum.ChannelPaidSearch

	case isSearch && !isPaid:
		leadSource.SearchEngine = searchEngine
		leadSource.Channel = enum.ChannelOrganicSearch

	case isSocial && isPaid:
		leadSource.AdPlatform = campaignMetadata.AdPlatform
		leadSource.Channel = enum.ChannelPaidSocial

	case isSocial && !isPaid:
		leadSource.SocialPlatform = socialPlatform
		leadSource.Channel = enum.ChannelOrganicSocial

	case isEmail && !isPaid:
		leadSource.Channel = enum.ChannelEmail

	case isPaid:
		leadSource.Channel = enum.ChannelPaidSocial
		leadSource.AdPlatform = campaignMetadata.AdPlatform

	default:
		leadSource.ReferrerHost = parsedReferrer.Host
		leadSource.ReferrerPath = parsedReferrer.Path
		leadSource.Channel = enum.ChannelReferral
	}

	leadSource.Campaign = campaignMetadata

	return leadSource, nil
}
