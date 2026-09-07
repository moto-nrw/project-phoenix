// Package legacy adapts the collaborators the info-point dashboard reads and
// that do not expose a public capability yet. It maps their values into the
// Device Fleet owner's compose types; nothing here reads iot.devices or
// display.displays.
package legacy

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/services/schedule"
)

// DashboardDependencies are the legacy readers the dashboard aggregate needs.
type DashboardDependencies struct {
	ActiveGroups   activeModels.GroupRepository
	Templates      activitiesModels.GroupRepository
	Instances      scheduleModels.ActivityInstanceRepository
	PickupSchedule schedule.PickupScheduleService
}

type dashboardSources struct{ deps DashboardDependencies }

// NewDashboardSources adapts the legacy readers to the owner's port.
func NewDashboardSources(deps DashboardDependencies) compose.DashboardSources {
	if deps.ActiveGroups == nil || deps.Templates == nil || deps.Instances == nil || deps.PickupSchedule == nil {
		panic("devicefleet dashboard sources: all dependencies are required")
	}
	return dashboardSources{deps: deps}
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
		session := compose.ActiveSession{ID: group.ID, RoomID: group.RoomID, Running: group.IsActive()}
		if templateID, ok := group.TemplateID(); ok {
			session.TemplateID = &templateID
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (s dashboardSources) ListActivityTemplates(ctx context.Context) ([]compose.ActivityTemplate, error) {
	groups, err := s.deps.Templates.ListWithCategory(ctx, nil)
	if err != nil {
		return nil, err
	}
	templates := make([]compose.ActivityTemplate, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		template := compose.ActivityTemplate{
			ID: group.ID, Name: group.Name, ParticipantLimit: group.ParticipantLimit(),
		}
		if group.Category != nil {
			name := group.Category.Name
			template.CategoryName = &name
		}
		templates = append(templates, template)
	}
	return templates, nil
}

func (s dashboardSources) ListPlannedActivities(ctx context.Context, today time.Time) ([]compose.PlannedActivity, error) {
	date := timezone.DateFromTime(today)
	instances, err := s.deps.Instances.FindByTenantAndDate(ctx, scheduleModels.Date(date))
	if err != nil {
		return nil, err
	}
	planned := make([]compose.PlannedActivity, 0, len(instances))
	for _, instance := range instances {
		if instance == nil {
			continue
		}
		planned = append(planned, compose.PlannedActivity{
			ID: instance.ID, Title: instance.Title, RoomID: instance.RoomID,
			ActivityGroupID: instance.ActivityGroupID, ActiveGroupID: instance.ActiveGroupID,
			Planned:        instance.Status == scheduleModels.InstanceStatusPlanned,
			StartWallClock: timezone.NormalizeWallClock(instance.StartTime),
		})
	}
	return planned, nil
}

func (s dashboardSources) ListPickupTimes(ctx context.Context, studentIDs []int64, today time.Time) ([]compose.PickupTime, error) {
	times, err := s.deps.PickupSchedule.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, timezone.DateFromTime(today))
	if err != nil {
		return nil, err
	}
	result := make([]compose.PickupTime, 0, len(times))
	for studentID, effective := range times {
		entry := compose.PickupTime{StudentID: studentID}
		if effective != nil && effective.PickupTime != nil {
			normalized := timezone.NormalizeWallClock(*effective.PickupTime)
			entry.PickupWallTime = &normalized
		}
		result = append(result, entry)
	}
	return result, nil
}
