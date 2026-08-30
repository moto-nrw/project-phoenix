package e2e_timetable

import (
	"context"

	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

func (s *scenario) createActiveVisit(ctx context.Context, visit *activeModel.Visit) error {
	return s.createVisitFn.(func(context.Context, *activeModel.Visit) error)(ctx, visit)
}

func (s *scenario) endActiveVisit(ctx context.Context, visitID int64) error {
	return s.endVisitFn.(func(context.Context, int64) error)(ctx, visitID)
}

func (s *scenario) previewTimetableCleanup(ctx context.Context) (*scheduleSvc.TimetableCleanupPreview, error) {
	result, err := s.previewCleanupFn(ctx)
	if err != nil {
		return nil, err
	}
	return result.(*scheduleSvc.TimetableCleanupPreview), nil
}

func (s *scenario) cleanupTimetable(ctx context.Context) (*scheduleSvc.TimetableCleanupResult, error) {
	result, err := s.cleanupFn(ctx)
	if err != nil {
		return nil, err
	}
	return result.(*scheduleSvc.TimetableCleanupResult), nil
}
