package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type TemplateSource interface {
	ListActivityTemplates(context.Context) ([]domain.ActivityTemplate, error)
}
type PlannedActivitySource interface {
	ListPlannedActivities(context.Context, time.Time) ([]domain.PlannedActivity, error)
}

type DashboardSources struct {
	TemplateSource
	PlannedActivitySource
	Sessions studentpresence.SessionRecords
	Pickups  careplan.BulkPickupTimes
}

func (s DashboardSources) ListActiveSessions(ctx context.Context) ([]domain.ActiveSession, error) {
	groups, err := s.Sessions.FindActiveGroups(ctx)
	if err != nil {
		return nil, err
	}
	sessions := make([]domain.ActiveSession, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		session := domain.ActiveSession{ID: group.ID, RoomID: group.RoomID, Running: group.IsOpen()}
		if templateID, ok := group.TemplateID(); ok {
			session.TemplateID = &templateID
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (s DashboardSources) ListPickupTimes(ctx context.Context, studentIDs []int64, today time.Time) ([]domain.PickupTime, error) {
	times, err := s.Pickups.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, calendar.DateFromTime(today))
	if err != nil {
		return nil, err
	}
	result := make([]domain.PickupTime, 0, len(times))
	for studentID, effective := range times {
		entry := domain.PickupTime{StudentID: studentID}
		if effective != nil && effective.PickupTime != nil {
			normalized := calendar.NormalizeWallClock(*effective.PickupTime)
			entry.PickupWallTime = &normalized
		}
		result = append(result, entry)
	}
	return result, nil
}
