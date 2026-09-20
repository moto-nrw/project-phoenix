package compose_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLatestDemoVisitsReadsOnlyTodayAndYesterdayAttendance(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	device := testpkg.CreateTestDevice(t, db, "demo-snapshot-web")
	today := testpkg.TodayDate()
	students := []struct {
		studentID int64
		date      string
	}{
		{testpkg.CreateTestStudent(t, db, "Demo", "Today", "3a").ID, today.String()},
		{testpkg.CreateTestStudent(t, db, "Demo", "Yesterday", "3a").ID, today.AddDays(-1).String()},
		{testpkg.CreateTestStudent(t, db, "Demo", "Historical", "3a").ID, today.AddDays(-2).String()},
	}
	for _, input := range students {
		_, err := module.RecordAttendance(ctx, studentpresence.Attendance{
			StudentID:   input.studentID,
			Date:        input.date,
			CheckInTime: time.Now(),
			DeviceID:    device.ID,
		})
		require.NoError(t, err)
	}

	visits, err := module.LatestDemoVisits(ctx, device.ID, today.AddDays(-1).String(), today.String())
	require.NoError(t, err)
	byStudent := make(map[int64]studentpresence.DemoVisit, len(visits))
	for _, visit := range visits {
		byStudent[visit.StudentID] = visit
	}
	assert.Contains(t, byStudent, students[0].studentID)
	assert.Contains(t, byStudent, students[1].studentID, "yesterday remains available for the midnight grace period")
	assert.NotContains(t, byStudent, students[2].studentID)
}
