package domain

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
	const today calendar.Date = "2026-11-07"

	november := ParticipatingOn(ids, students, boundaries, today, today, nil)
	assert.False(t, november[scheduled], "a scheduled child does not count before its care start")
	assert.True(t, november[immediate], "an immediately activated child keeps counting")
	assert.True(t, november[renewed], "a running renewal keeps counting")
	assert.True(t, november[pendingOverdue], "a start already reached counts before the hourly activation")
	assert.True(t, november[pendingUndated], "a pending child without a start date is not hidden")

	assert.False(t, ParticipatingOn(ids, students, boundaries, start.AddDays(-1), today, nil)[scheduled],
		"the day before the start does not count")
	assert.True(t, ParticipatingOn(ids, students, boundaries, start, today, nil)[scheduled],
		"the start day counts")
	assert.True(t, ParticipatingOn(ids, students, boundaries, start.AddDays(30), today, nil)[scheduled],
		"a planning day after the start counts")

	present := ParticipatingOn(ids, students, boundaries, today, today, map[int64]bool{scheduled: true})
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
		alumnus:       {Status: careplan.StudentStatusAlumnus, FirstCareFrom: &start},
		retainedEnded: {Status: careplan.StudentStatusInactive, EnrolledUntil: &ended},
	}
	ids := []int64{scheduled, endedChild, alumnus, retainedEnded}
	boundaries := ParticipationBoundaries(students, nil)
	visible := AdministrativelyVisibleOn(ids, students, boundaries, "2026-11-07", "2026-11-07", map[int64]bool{retainedEnded: true})
	assert.True(t, visible[scheduled], "the administrative list keeps a child before its care start")
	assert.False(t, visible[endedChild], "an ended care stays out")
	assert.False(t, visible[alumnus])
	assert.True(t, visible[retainedEnded], "a pending withdrawal task still retains the child")
}

func TestAdministrativeVisibilityPreservesScheduledDepartureBoundaries(t *testing.T) {
	t.Parallel()
	const studentID int64 = 1
	today := calendar.Date("2026-11-07")
	start := calendar.Date("2027-02-03")
	yesterday, tomorrow := today.AddDays(-1), today.AddDays(1)
	for _, kind := range []struct {
		name    string
		student CareStudent
	}{
		{"pending_start", CareStudent{Status: careplan.StudentStatusPending, EnrolledFrom: &start}},
		{"first_care_bound", CareStudent{Status: careplan.StudentStatusInactive, FirstCareFrom: &start}},
	} {
		for _, tc := range []struct {
			name     string
			until    *calendar.Date
			pending  *calendar.Date
			retained bool
			visible  bool
		}{
			{name: "without_departure", visible: true},
			{name: "last_enrollment_day", until: &today, visible: true},
			{name: "expired_enrollment", until: &yesterday},
			{name: "future_completion", pending: &tomorrow, visible: true},
			{name: "completion_day", pending: &today},
			{name: "expired_completion", pending: &yesterday},
			{name: "enrollment_ends_first", until: &yesterday, pending: &tomorrow},
			{name: "completion_ends_first", until: &tomorrow, pending: &today},
			{name: "retained_after_enrollment", until: &yesterday, retained: true, visible: true},
			{name: "retained_after_completion", pending: &today, retained: true, visible: true},
		} {
			t.Run(kind.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				student := kind.student
				student.EnrolledUntil = tc.until
				students := map[int64]CareStudent{studentID: student}
				pending := make(map[int64]calendar.Date)
				if tc.pending != nil {
					pending[studentID] = *tc.pending
				}
				boundaries := ParticipationBoundaries(students, pending)
				visible := AdministrativelyVisibleOn([]int64{studentID}, students, boundaries, today, today,
					map[int64]bool{studentID: tc.retained})
				assert.Equal(t, tc.visible, visible[studentID])
			})
		}
	}
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
	const today calendar.Date = "2026-11-07"
	assert.False(t, HistoricalParticipationOn(ids, students, boundaries, today, today, nil)[scheduled],
		"a dated end does not pull a child into care before its start")
	assert.True(t, HistoricalParticipationOn(ids, students, boundaries, start, today, nil)[scheduled])
	assert.False(t, HistoricalParticipationOn(ids, students, boundaries, until.AddDays(1), today, nil)[scheduled],
		"the end boundary still applies")
}

// The school schedules a return (#6041 review): care ran until 6 November and
// restarts on 3 February. The resumption stores the child as pending with the
// new start and no end, like a new child, so a past day cannot tell the two
// apart. Past days keep their participation; from today on the child waits.
func TestParticipationKeepsThePastOfAScheduledReturn(t *testing.T) {
	t.Parallel()
	restart := calendar.Date("2027-02-03")
	const returning int64 = 9
	students := map[int64]CareStudent{
		returning: {Status: careplan.StudentStatusPending, EnrolledFrom: &restart},
	}
	ids := []int64{returning}
	boundaries := ParticipationBoundaries(students, nil)
	const today calendar.Date = "2026-11-07"
	const cared calendar.Date = "2026-11-05"
	assert.True(t, HistoricalParticipationOn(ids, students, boundaries, cared, today, nil)[returning],
		"a report keeps the care before the scheduled return")
	assert.True(t, ParticipatingOn(ids, students, boundaries, cared, today, nil)[returning],
		"a past day of a running reader keeps the child too")
	assert.False(t, HistoricalParticipationOn(ids, students, boundaries, today, today, nil)[returning],
		"from today on the child waits for its new start")
	assert.False(t, ParticipatingOn(ids, students, boundaries, today, today, nil)[returning])
	assert.False(t, ParticipatingOn(ids, students, boundaries, restart.AddDays(-1), today, nil)[returning])
	assert.True(t, ParticipatingOn(ids, students, boundaries, restart, today, nil)[returning])
}

// A new child approved in December for care from 3 February (#6041 review 2):
// School Membership bounds its first care by the start (FirstCareFrom), so a past day
// of the running half-year does not pull it in either. A read on 8 December
// of 7 December must agree with the read on 7 December.
func TestParticipationHoldsBackTheWholePastOfAKnownNewChild(t *testing.T) {
	t.Parallel()
	start := calendar.Date("2027-02-03")
	until := calendar.Date("2027-07-31")
	const (
		newChild int64 = iota + 1
		returning
	)
	students := map[int64]CareStudent{
		newChild:  {Status: careplan.StudentStatusPending, EnrolledFrom: &start, EnrolledUntil: &until, FirstCareFrom: &start},
		returning: {Status: careplan.StudentStatusPending, EnrolledFrom: &start},
	}
	ids := []int64{newChild, returning}
	boundaries := ParticipationBoundaries(students, nil)
	const yesterday, today calendar.Date = "2026-12-07", "2026-12-08"
	for _, day := range []calendar.Date{"2026-08-24", yesterday} {
		assert.False(t, HistoricalParticipationOn(ids, students, boundaries, day, today, nil)[newChild],
			"%s: a known new child was never cared for before its start", day)
		assert.False(t, ParticipatingOn(ids, students, boundaries, day, today, nil)[newChild], day)
		assert.True(t, HistoricalParticipationOn(ids, students, boundaries, day, today, nil)[returning],
			"%s: a returning child keeps its past", day)
	}
	assert.Equal(t,
		HistoricalParticipationOn(ids, students, boundaries, yesterday, yesterday, nil)[newChild],
		HistoricalParticipationOn(ids, students, boundaries, yesterday, today, nil)[newChild],
		"the answer for a day does not change with the day it is read")
	assert.True(t, ParticipatingOn(ids, students, boundaries, yesterday, today, map[int64]bool{newChild: true})[newChild],
		"actual presence still wins")
	assert.True(t, HistoricalParticipationOn(ids, students, boundaries, start, today, nil)[newChild], "the start day counts")
}

// The hourly run activates a new child on its start day, 3 February (#6041
// review 3). The first-care bound survives the activation: read on
// 4 February, January and the days before the start stay out, for the
// running lists and the reports alike, while an immediately activated child
// without a bound keeps taking part before its regular start.
func TestParticipationKeepsTheFirstCareBoundAfterActivation(t *testing.T) {
	t.Parallel()
	start := calendar.Date("2099-02-03")
	const (
		activated int64 = iota + 1
		immediate
	)
	students := map[int64]CareStudent{
		activated: {Status: careplan.StudentStatusActive, EnrolledFrom: &start, FirstCareFrom: &start},
		immediate: {Status: careplan.StudentStatusActive, EnrolledFrom: &start},
	}
	ids := []int64{activated, immediate}
	boundaries := ParticipationBoundaries(students, nil)
	const today calendar.Date = "2099-02-04"
	for _, day := range []calendar.Date{"2099-01-15", "2099-02-02"} {
		assert.False(t, HistoricalParticipationOn(ids, students, boundaries, day, today, nil)[activated],
			"%s: the activation is no evidence of care before the start", day)
		assert.False(t, ParticipatingOn(ids, students, boundaries, day, today, nil)[activated], day)
		assert.True(t, HistoricalParticipationOn(ids, students, boundaries, day, today, nil)[immediate],
			"%s: an immediately activated child keeps taking part before its start", day)
	}
	assert.True(t, HistoricalParticipationOn(ids, students, boundaries, start, today, nil)[activated], "the start day counts")
	assert.True(t, ParticipatingOn(ids, students, boundaries, today, today, nil)[activated])
}

// A new child waiting for its start comes to the OGS on 27 February (#6041
// review 3). The recorded presence of that day is care: a report read later
// keeps it, while a day without presence and a day after an ended care stay
// out.
func TestHistoricalParticipationHonorsRecordedPresenceBeforeTheStart(t *testing.T) {
	t.Parallel()
	start := calendar.Date("2099-04-19")
	ended := calendar.Date("2099-02-20")
	const (
		newChild int64 = iota + 1
		endedNew
		alumnus
	)
	students := map[int64]CareStudent{
		newChild: {Status: careplan.StudentStatusPending, EnrolledFrom: &start, FirstCareFrom: &start},
		endedNew: {Status: careplan.StudentStatusInactive, EnrolledFrom: &start, EnrolledUntil: &ended, FirstCareFrom: &start},
		alumnus:  {Status: careplan.StudentStatusAlumnus, FirstCareFrom: &start},
	}
	ids := []int64{newChild, endedNew, alumnus}
	boundaries := ParticipationBoundaries(students, nil)
	const today calendar.Date = "2099-03-02"
	present := map[int64]bool{newChild: true, endedNew: true, alumnus: true}
	attended := HistoricalParticipationOn(ids, students, boundaries, "2099-02-27", today, present)
	assert.True(t, attended[newChild], "recorded presence before the start is care")
	assert.False(t, attended[endedNew], "presence does not reach past the end of care")
	assert.False(t, attended[alumnus], "an undated alumnus stays out")
	assert.False(t, HistoricalParticipationOn(ids, students, boundaries, "2099-02-26", today, nil)[newChild],
		"a day without presence stays out")
	assert.True(t, HistoricalParticipationOn(ids, students, boundaries, today, today, map[int64]bool{newChild: true})[newChild],
		"presence today counts as well")
}

// A child's care ended on 31 October; on 7 November the school resumes it
// for 1 February (#6073). School Membership keeps the gap, so the days in
// between are no care days on every later read, also after the activation,
// while the care before the end and recorded presence in the gap still
// count.
func TestParticipationLeavesOutTheCareGapOfAReturningChild(t *testing.T) {
	t.Parallel()
	ended, resumes := calendar.Date("2099-10-31"), calendar.Date("2100-02-01")
	const (
		pending int64 = iota + 1
		activated
		unknown
	)
	students := map[int64]CareStudent{
		pending:   {Status: careplan.StudentStatusPending, EnrolledFrom: &resumes, CarePausedAfter: &ended, CareResumedFrom: &resumes},
		activated: {Status: careplan.StudentStatusActive, EnrolledFrom: &resumes, CarePausedAfter: &ended, CareResumedFrom: &resumes},
		unknown:   {Status: careplan.StudentStatusPending, EnrolledFrom: &resumes},
	}
	ids := []int64{pending, activated, unknown}
	boundaries := ParticipationBoundaries(students, nil)
	for _, today := range []calendar.Date{"2099-11-08", "2100-02-02"} {
		for _, day := range []calendar.Date{"2099-11-01", "2099-11-06", "2100-01-31"} {
			if day.After(today) {
				continue
			}
			for _, id := range []int64{pending, activated} {
				assert.False(t, HistoricalParticipationOn(ids, students, boundaries, day, today, nil)[id],
					"read %s: %s lies in the gap of child %d", today, day, id)
				assert.False(t, ParticipatingOn(ids, students, boundaries, day, today, nil)[id], day)
			}
		}
		assert.True(t, HistoricalParticipationOn(ids, students, boundaries, ended, today, nil)[pending],
			"the last care day before the gap counts")
		assert.True(t, HistoricalParticipationOn(ids, students, boundaries, "2099-11-03", today, nil)[unknown],
			"without a known gap a returning child keeps its past days")
	}
	assert.True(t, HistoricalParticipationOn(ids, students, boundaries, resumes, "2100-02-02", nil)[activated],
		"the day care resumes counts")
	assert.True(t, HistoricalParticipationOn(ids, students, boundaries, "2099-11-04", "2099-11-08", map[int64]bool{pending: true})[pending],
		"recorded presence in the gap is care")
}
