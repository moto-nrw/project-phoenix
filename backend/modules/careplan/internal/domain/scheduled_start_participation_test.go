package domain

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

// A child approved in November for care from 3 February waits as pending with
// that start (#6041). It takes part in care from the start day on, also on a
// planning day after it; before, only actual presence keeps it.
func TestParticipationWaitsForScheduledStart(t *testing.T) {
	t.Parallel()
	start := calendar.Date("2027-02-03")
	pastStart := calendar.Date("2026-09-01")
	renewedUntil := calendar.Date("2027-07-31")
	const (
		scheduled = iota + 1
		immediate
		renewed
		pendingOverdue
		pendingUndated
	)
	students := map[int64]CareStudent{
		scheduled:      {Status: careplan.StudentStatusPending, EnrolledFrom: &start},
		immediate:      {Status: careplan.StudentStatusActive, EnrolledFrom: &start},
		renewed:        {Status: careplan.StudentStatusActive, EnrolledFrom: &pastStart, EnrolledUntil: &renewedUntil},
		pendingOverdue: {Status: careplan.StudentStatusPending, EnrolledFrom: &pastStart},
		pendingUndated: {Status: careplan.StudentStatusPending},
	}
	ids := []int64{scheduled, immediate, renewed, pendingOverdue, pendingUndated}
	boundaries := ParticipationBoundaries(students, nil)

	november := ParticipatingOn(ids, students, boundaries, "2026-11-07", nil)
	assert.False(t, november[scheduled], "a scheduled child does not count before its care start")
	assert.True(t, november[immediate], "an immediately activated child keeps counting")
	assert.True(t, november[renewed], "a running renewal keeps counting")
	assert.True(t, november[pendingOverdue], "a start already reached counts before the hourly activation")
	assert.True(t, november[pendingUndated], "a pending child without a start date is not hidden")

	assert.False(t, ParticipatingOn(ids, students, boundaries, start.AddDays(-1), nil)[scheduled],
		"the day before the start does not count")
	assert.True(t, ParticipatingOn(ids, students, boundaries, start, nil)[scheduled],
		"the start day counts")
	assert.True(t, ParticipatingOn(ids, students, boundaries, start.AddDays(30), nil)[scheduled],
		"a planning day after the start counts")

	present := ParticipatingOn(ids, students, boundaries, "2026-11-07", map[int64]bool{scheduled: true})
	assert.True(t, present[scheduled], "actual presence always wins")
}

// The administrative child list keeps a waiting child findable, so the school
// can prepare it; the end boundary and the alumni rule still apply there.
func TestAdministrativeVisibilityKeepsAScheduledChild(t *testing.T) {
	t.Parallel()
	start := calendar.Date("2027-02-03")
	ended := calendar.Date("2026-10-31")
	const (
		scheduled = iota + 1
		endedChild
		alumnus
		retainedEnded
	)
	students := map[int64]CareStudent{
		scheduled:     {Status: careplan.StudentStatusPending, EnrolledFrom: &start},
		endedChild:    {Status: careplan.StudentStatusInactive, EnrolledUntil: &ended},
		alumnus:       {Status: careplan.StudentStatusAlumnus},
		retainedEnded: {Status: careplan.StudentStatusInactive, EnrolledUntil: &ended},
	}
	ids := []int64{scheduled, endedChild, alumnus, retainedEnded}
	boundaries := ParticipationBoundaries(students, nil)
	visible := AdministrativelyVisibleOn(ids, students, boundaries, "2026-11-07", map[int64]bool{retainedEnded: true})
	assert.True(t, visible[scheduled], "the administrative list keeps a child before its care start")
	assert.False(t, visible[endedChild], "an ended care stays out")
	assert.False(t, visible[alumnus])
	assert.True(t, visible[retainedEnded], "a pending withdrawal task still retains the child")
}

// The scheduled start also holds for retained reports: a child that only
// starts in February was not cared for in November, even with a dated end.
func TestHistoricalParticipationWaitsForScheduledStart(t *testing.T) {
	t.Parallel()
	start := calendar.Date("2027-02-03")
	until := calendar.Date("2027-07-31")
	const scheduled int64 = 7
	students := map[int64]CareStudent{
		scheduled: {Status: careplan.StudentStatusPending, EnrolledFrom: &start, EnrolledUntil: &until},
	}
	ids := []int64{scheduled}
	boundaries := ParticipationBoundaries(students, nil)
	assert.False(t, HistoricalParticipationOn(ids, students, boundaries, "2026-11-07")[scheduled],
		"a dated end does not pull a child into care before its start")
	assert.True(t, HistoricalParticipationOn(ids, students, boundaries, start)[scheduled])
	assert.False(t, HistoricalParticipationOn(ids, students, boundaries, until.AddDays(1))[scheduled],
		"the end boundary still applies")
}
