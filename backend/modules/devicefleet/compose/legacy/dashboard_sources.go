// Package legacy adapts the collaborators the info-point dashboard reads and
// that do not expose a public capability yet. It maps their values into the
// Device Fleet owner's compose types; nothing here reads iot.devices or
// display.displays.
package legacy

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type templateSource interface {
	ListActivityTemplates(context.Context) ([]compose.ActivityTemplate, error)
}
type plannedActivitySource interface {
	ListPlannedActivities(context.Context, time.Time) ([]compose.PlannedActivity, error)
}

// DashboardDependencies are the legacy readers the dashboard aggregate needs.
type DashboardDependencies struct {
	ActiveGroups   studentpresence.SessionRecords
	Templates      templateSource
	Instances      plannedActivitySource
	PickupSchedule careplan.BulkPickupTimes
}

type dashboardSources struct {
	templateSource
	plannedActivitySource
	deps DashboardDependencies
}

// NewDashboardSources adapts the legacy readers to the owner's port.
func NewDashboardSources(deps DashboardDependencies) compose.DashboardSources {
	if deps.ActiveGroups == nil || deps.Templates == nil || deps.Instances == nil || deps.PickupSchedule == nil {
		panic("devicefleet dashboard sources: all dependencies are required")
	}
	return dashboardSources{templateSource: deps.Templates, plannedActivitySource: deps.Instances, deps: deps}
}

func (s dashboardSources) ListActiveSessions(ctx context.Context) ([]compose.ActiveSession, error) {
	groups, err := s.deps.ActiveGroups.FindActiveGroups(ctx)
	if err != nil {
		return nil, err
	}
	sessions := make([]compose.ActiveSession, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		session := compose.ActiveSession{ID: group.ID, RoomID: group.RoomID, Running: group.IsOpen()}
		if templateID, ok := group.TemplateID(); ok {
			session.TemplateID = &templateID
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (s dashboardSources) ListPickupTimes(ctx context.Context, studentIDs []int64, today time.Time) ([]compose.PickupTime, error) {
	times, err := s.deps.PickupSchedule.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, calendar.DateFromTime(today))
	if err != nil {
		return nil, err
	}
	result := make([]compose.PickupTime, 0, len(times))
	for studentID, effective := range times {
		entry := compose.PickupTime{StudentID: studentID}
		if effective != nil && effective.PickupTime != nil {
			normalized := calendar.NormalizeWallClock(*effective.PickupTime)
			entry.PickupWallTime = &normalized
		}
		result = append(result, entry)
	}
	return result, nil
}
