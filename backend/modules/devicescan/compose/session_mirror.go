package compose

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type MirroredSession = application.MirroredSession

// NewSessionMirror binds the optional mirror to the retained timetable data
// service. The root supplies the post-commit event publisher.
func NewSessionMirror(data *scheduleSvc.TimetableDataService, activities activitiesSvc.ActivityService, publish application.MirrorPublisher, logger *slog.Logger) application.SessionMirror {
	if data == nil {
		return nil
	}
	return application.NewSessionMirror(mirrorTimetable{data: data, activities: activities}, publish, logger)
}

type mirrorTimetable struct {
	data       *scheduleSvc.TimetableDataService
	activities activitiesSvc.ActivityService
}

func (m mirrorTimetable) HasInstance(ctx context.Context, sessionID int64) (bool, error) {
	instance, err := m.data.GetInstanceByActiveGroupID(ctx, sessionID)
	return instance != nil, err
}

func (m mirrorTimetable) CalendarParts(instant time.Time) (string, int) {
	local := instant.In(timezone.Berlin)
	return local.Format("2006-01-02"), local.Hour()*60 + local.Minute()
}

func (m mirrorTimetable) ActivityTitle(ctx context.Context, activityID int64) (string, error) {
	if m.activities == nil {
		return "", nil
	}
	group, err := m.activities.GetGroup(ctx, activityID)
	if group == nil {
		return "", err
	}
	return group.Name, err
}

func (m mirrorTimetable) CreateInstance(ctx context.Context, input MirroredSession) (MirroredSession, error) {
	date, err := scheduleModel.ParseDate(input.Date)
	if err != nil {
		return MirroredSession{}, err
	}
	start, err := time.Parse("15:04:05", input.StartTime)
	if err != nil {
		return MirroredSession{}, err
	}
	end, err := time.Parse("15:04:05", input.EndTime)
	if err != nil {
		return MirroredSession{}, err
	}
	instance := &scheduleModel.ActivityInstance{
		Date: date, ActivityGroupID: input.ActivityGroupID, Title: input.Title,
		StartTime: timezone.NormalizeWallClock(start), EndTime: timezone.NormalizeWallClock(end), RoomID: input.RoomID,
		Status: scheduleModel.InstanceStatusActive, ActiveGroupID: input.ActiveGroupID, IsSpontaneous: true,
		CreatedBy: input.CreatedBy, StartedBy: input.StartedBy, StartedAt: input.StartedAt,
	}
	instance.SetTenantID(tenant.FromContext(ctx))
	if err := m.data.CreateActivityInstance(ctx, instance); err != nil {
		return MirroredSession{}, err
	}
	input.ID = instance.ID
	return input, nil
}

func (m mirrorTimetable) AddStaff(ctx context.Context, instanceID, staffID int64) error {
	row := &scheduleModel.InstanceStaff{InstanceID: instanceID, StaffID: staffID}
	row.SetTenantID(tenant.FromContext(ctx))
	return m.data.CreateInstanceStaff(ctx, row)
}
