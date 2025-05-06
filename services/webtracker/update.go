package webtracker

import (
	"context"
	"errors"

	"github.com/customeros/leads/dto"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
)

func (s *webtrackerService) UpdateCNAMEHost(ctx context.Context, webtrackerID, cnameHost string) (*models.WebTracker, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.UpdateCNAMEHost")
	defer span.Finish()

	configured := false

	// check if tracker exists
	trackerRecord, err := s.repositories.WebTracker.GetByID(ctx, webtrackerID)
	if err != nil {
		span.TraceError(err)
		return nil, err
	}
	if trackerRecord == nil {
		err := errors.New("Cannot find webtracker")
		span.TraceError(err)
		return nil, err
	}

	// update tracker
	updateRecord := dto.WebTrackerUpdate{
		ID:                webtrackerID,
		CNAMEHost:         &cnameHost,
		IsCNAMEConfigured: &configured,
	}

	err = s.repositories.WebTracker.Update(ctx, updateRecord)
	if err != nil {
		span.TraceError(err)
		return nil, err
	}

	trackerRecord.CNAMEHost = cnameHost
	trackerRecord.IsCNAMEConfigured = configured
	trackerRecord.UpdatedAt = utils.NowPtr()

	return trackerRecord, nil
}

func (s *webtrackerService) IsCNAMEConfigured(ctx context.Context, webtrackerID string) (bool, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.IsCNAMEConfigured")
	defer span.Finish()

	trackerRecord, err := s.repositories.WebTracker.GetByID(ctx, webtrackerID)
	if err != nil {
		span.TraceError(err)
		return false, err
	}
	if trackerRecord == nil {
		err := errors.New("Cannot find webtracker")
		span.TraceError(err)
		return false, err
	}

	return trackerRecord.IsCNAMEConfigured, nil
}

func (s *webtrackerService) ArchiveWebtracker(ctx context.Context, webtrackerID string) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.ArchiveWebtracker")
	defer span.Finish()

	trackerRecord, err := s.repositories.WebTracker.GetByID(ctx, webtrackerID)
	if err != nil {
		span.TraceError(err)
		return err
	}
	if trackerRecord == nil {
		err := errors.New("Cannot find webtracker")
		span.TraceError(err)
		return err
	}

	return s.repositories.WebTracker.Archive(ctx, webtrackerID)
}

func (s *webtrackerService) GetWebtracker(ctx context.Context, webtrackerID string) (*models.WebTracker, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.GetWebtracker")
	defer span.Finish()

	trackerRecord, err := s.repositories.WebTracker.GetByID(ctx, webtrackerID)
	if err != nil {
		span.TraceError(err)
		return nil, err
	}
	if trackerRecord == nil {
		err := errors.New("Cannot find webtracker")
		span.TraceError(err)
		return nil, err
	}
	return trackerRecord, nil
}

func (s *webtrackerService) GetActiveWebtrackers(ctx context.Context) ([]models.WebTracker, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "webtrackerService.GetActiveWebtrackers")
	defer span.Finish()

	return s.repositories.WebTracker.GetActiveTrackers(ctx)
}
