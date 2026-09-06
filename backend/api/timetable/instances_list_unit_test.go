package timetable

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
)

// The per-occurrence pin wins over the template override; a NULL pin inherits
// the template value; both NULL means "derive from the Betreuungsschlüssel"
// downstream (#1839).
func TestInstanceRequiredStaffOverride(t *testing.T) {
	t.Parallel()

	pin := 5
	tmpl := 3

	tests := []struct {
		name             string
		pin              *int
		templateOverride *int
		want             *int
	}{
		{"pin wins over template override", &pin, &tmpl, &pin},
		{"nil pin inherits template override", nil, &tmpl, &tmpl},
		{"pin without template override", &pin, nil, &pin},
		{"both nil derives downstream", nil, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, instanceRequiredStaffOverride(tt.pin, tt.templateOverride))
		})
	}
}

// A timed auto-excusal pickup exception can coexist with a timeless "Kommt
// heute nicht" exception on the same day: the care-day verdict is cancelled
// while the pre-cutoff attendance row stays expected. That row must not carry
// the early-pickup marker — the child is not leaving early, they are not
// coming at all (#2360 review round 5).
func TestSummarizeInstanceStudentsEarlyPickupRequiresExpectedCareDay(t *testing.T) {
	t.Parallel()

	const studentID = int64(4711)
	day := timezone.NewDate(2030, time.March, 4)
	inst := &scheduleModel.ActivityInstance{
		Date:      scheduleModel.Date(day),
		StartTime: timezone.NormalizeWallClock(time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC)),
		EndTime:   timezone.NormalizeWallClock(time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC)),
		Status:    scheduleModel.InstanceStatusPlanned,
	}
	rows := []*scheduleModel.InstanceStudent{
		{StudentID: studentID, Status: scheduleModel.AttendanceStatusExpected},
	}
	cutoffs := map[int64]time.Time{
		studentID: timezone.NormalizeWallClock(time.Date(1, 1, 1, 14, 45, 0, 0, time.UTC)),
	}
	verdicts := func(v scheduleSvc.CareDayStatus) map[int64]map[timezone.Date]scheduleSvc.CareDayStatus {
		return map[int64]map[timezone.Date]scheduleSvc.CareDayStatus{studentID: {day: v}}
	}

	scheduled := summarizeInstanceStudents(inst, rows, verdicts(scheduleSvc.CareDayScheduled), cutoffs)
	if assert.Len(t, scheduled.students, 1) && assert.NotNil(t, scheduled.students[0].EarlyPickupTime) {
		assert.Equal(t, "14:45", *scheduled.students[0].EarlyPickupTime)
	}

	cancelled := summarizeInstanceStudents(inst, rows, verdicts(scheduleSvc.CareDayCancelled), cutoffs)
	if assert.Len(t, cancelled.students, 1) {
		assert.Nil(t, cancelled.students[0].EarlyPickupTime)
	}
}
