package compose

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type MirroredSession = application.MirroredSession

// SessionMirror records a kiosk session start as a spontaneous timetable
// block.
type SessionMirror = application.SessionMirror

// MirrorInstances is the retained instance storage the kiosk mirror writes a
// spontaneous block into; the retained rows route its session state to
// Student Presence.
type MirrorInstances interface {
	FindByActiveGroupID(ctx context.Context, activeGroupID int64) (*scheduleModel.ActivityInstance, error)
	Create(ctx context.Context, instance *scheduleModel.ActivityInstance) error
}

// MirrorInstanceStaff stores the staff row of a mirrored block.
type MirrorInstanceStaff interface {
	Create(ctx context.Context, staff *scheduleModel.InstanceStaff) error
}

// NewSessionMirror binds the optional mirror to the retained timetable
// rows. The root supplies the post-commit event publisher.
func NewSessionMirror(instances MirrorInstances, staff MirrorInstanceStaff, activities activitiesSvc.ActivityService, publish application.MirrorPublisher, logger *slog.Logger) application.SessionMirror {
	if instances == nil || staff == nil {
		return nil
	}
	return application.NewSessionMirror(mirrorTimetable{instances: instances, staff: staff, activities: activities}, publish, logger)
}

type mirrorTimetable struct {
	instances  MirrorInstances
	staff      MirrorInstanceStaff
	activities activitiesSvc.ActivityService
}

func (m mirrorTimetable) HasInstance(ctx context.Context, sessionID int64) (bool, error) {
	instance, err := m.instances.FindByActiveGroupID(ctx, sessionID)
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
	if err := m.instances.Create(ctx, instance); err != nil {
		return MirroredSession{}, err
	}
	input.ID = instance.ID
	return input, nil
}

func (m mirrorTimetable) AddStaff(ctx context.Context, instanceID, staffID int64) error {
	row := &scheduleModel.InstanceStaff{InstanceID: instanceID, StaffID: staffID}
	row.SetTenantID(tenant.FromContext(ctx))
	return m.staff.Create(ctx, row)
}
