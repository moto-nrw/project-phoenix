package httpintegration_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstance_Create_RejectsExternalCaregiver(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	external := testpkg.CreateTestGuest(t, s.db, "Musik")

	created, err := s.svc.CreateInstance(s.ctx, timetable.CreateInstanceInput{
		Date:            calendar.TodayDate().AddDays(7),
		StartTime:       time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		Title:           "Geplante Musik-AG",
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
		StaffIDs:        []int64{external.StaffID},
	})

	require.Error(t, err)
	assert.Nil(t, created)
	assert.ErrorIs(t, err, timetable.ErrInvalidInstanceReference)
	assert.Zero(t, countRowsWhere(t, s, "schedule.activity_instances", "room_id", s.roomID))
}
