package application

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

func TestMaterializedInstanceValidatesMidnightClock(t *testing.T) {
	t.Parallel()
	midnight := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	instance := materializedInstance(&activityContract.Group{Name: "Night care"}, calendar.NewDate(2026, 11, 2), &schedule.CalendarPeriod{}, materialParams{StartTime: midnight, EndTime: midnight.Add(time.Hour), RoomID: 5})
	require.NoError(t, instance.Validate())
}

func TestPlannedEditDoesNotInheritClockPresence(t *testing.T) {
	t.Parallel()
	midnight := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	instance := &schedule.ActivityInstance{Status: schedule.InstanceStatusPlanned}
	instance.SetWallClockTimes(midnight, midnight.Add(time.Hour))
	applyPlannedEdit(instance, timetable.UpdateInstanceInput{Date: calendar.NewDate(2026, 11, 2), Title: "Care", RoomID: 5, EndTime: midnight.Add(time.Hour)})
	require.EqualError(t, instance.Validate(), "start_time is required")
}
