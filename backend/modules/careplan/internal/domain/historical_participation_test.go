package domain

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestHistoricalParticipationUsesDatedAlumniBoundaries(t *testing.T) {
	t.Parallel()
	lastDay := calendar.Date("2026-08-25")
	students := map[int64]CareStudent{
		1: {Status: careplan.StudentStatusAlumnus, EnrolledUntil: &lastDay},
		2: {Status: careplan.StudentStatusAlumnus},
		3: {Status: careplan.StudentStatusAlumnus, EnrolledUntil: &lastDay},
		4: {Status: careplan.StudentStatusActive},
	}
	ids := []int64{1, 2, 3, 4}
	boundaries := ParticipationBoundaries(students, map[int64]calendar.Date{3: "2026-08-25"})
	before := HistoricalParticipationOn(ids, students, boundaries, lastDay.AddDays(-1))
	assert.True(t, before[1])
	assert.False(t, before[2], "an undated alumnus has no reconstructable departure")
	assert.True(t, before[3])
	assert.True(t, before[4])
	on := HistoricalParticipationOn(ids, students, boundaries, lastDay)
	assert.True(t, on[1], "the last care day is inclusive")
	assert.False(t, on[3], "an earlier pending withdrawal boundary still wins")
	after := HistoricalParticipationOn(ids, students, boundaries, lastDay.AddDays(1))
	assert.False(t, after[1])
	assert.False(t, after[3])
	assert.True(t, after[4])
	assert.False(t, ParticipatingOn(ids, students, boundaries, lastDay.AddDays(-1), nil)[1],
		"current visibility keeps its existing alumni exclusion")
}
