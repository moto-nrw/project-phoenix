package api

import (
	"context"
	"log/slog"
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewKioskSessionMirror binds existing Timetable wiring, without a root graph field.
func NewKioskSessionMirror(rows timetableCompose.Repositories, activities activityContract.ActivityService, publish func(context.Context, devicescanCompose.MirroredSession), logger *slog.Logger) devicescanCompose.SessionMirror {
	if rows.ActivityInstance == nil || rows.InstanceStaff == nil {
		return nil
	}
	return devicescanCompose.NewSessionMirror(mirrorTimetable{records: rows.NativeRecords(), activities: timetableCompose.NewCatalog(activities)}, publish, logger)
}

type mirrorTimetable struct {
	records    timetable.SessionRecords
	activities timetable.Catalog
}

func (m mirrorTimetable) HasInstance(ctx context.Context, sessionID int64) (bool, error) {
	instance, err := m.records.FindSessionInstance(ctx, sessionID)
	return instance != nil, err
}

func (m mirrorTimetable) CalendarParts(instant time.Time) (string, int) {
	local := instant.In(calendar.Berlin)
	return local.Format("2006-01-02"), local.Hour()*60 + local.Minute()
}

func (m mirrorTimetable) ActivityTitle(ctx context.Context, activityID int64) (string, error) {
	if m.activities == nil {
		return "", nil
	}
	group, err := m.activities.FindCatalogGroup(ctx, activityID)
	if group == nil {
		return "", err
	}
	return group.Name, err
}

func (m mirrorTimetable) CreateInstance(ctx context.Context, input devicescanCompose.MirroredSession) (devicescanCompose.MirroredSession, error) {
	date, err := calendar.ParseDate(input.Date)
	if err != nil {
		return devicescanCompose.MirroredSession{}, err
	}
	start, err := time.Parse("15:04:05", input.StartTime)
	if err != nil {
		return devicescanCompose.MirroredSession{}, err
	}
	end, err := time.Parse("15:04:05", input.EndTime)
	if err != nil {
		return devicescanCompose.MirroredSession{}, err
	}
	id, err := m.records.CreateMirroredSessionInstance(ctx, timetable.ScheduledInstance{
		Date: date, ActivityGroupID: input.ActivityGroupID, Title: input.Title,
		StartTime: start, EndTime: end, RoomID: input.RoomID, CreatedBy: input.CreatedBy,
		ActiveGroupID: input.ActiveGroupID, StartedBy: input.StartedBy, StartedAt: input.StartedAt,
	})
	if err != nil {
		return devicescanCompose.MirroredSession{}, err
	}
	input.ID = id
	return input, nil
}

func (m mirrorTimetable) AddStaff(ctx context.Context, instanceID, staffID int64) error {
	return m.records.AddSessionInstanceStaff(ctx, instanceID, staffID)
}
