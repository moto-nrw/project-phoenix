package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCountSickCountsEachRosterChildOnce(t *testing.T) {
	t.Parallel()
	const (
		bothMarks, dayOnly, flagOnly, healthy     int64 = 10, 11, 12, 13
		bothMarksM, dayOnlyM, flagOnlyM, healthyM int64 = 20, 21, 22, 23
		endedCare, endedCareM                     int64 = 14, 24
	)
	roster := map[int64]int64{bothMarks: bothMarksM, dayOnly: dayOnlyM, flagOnly: flagOnlyM, healthy: healthyM}

	assert.Equal(t, 3, countSick(roster, []int64{bothMarks, dayOnly, endedCare}, []int64{bothMarksM, flagOnlyM, endedCareM}, nil))
	assert.Zero(t, countSick(nil, []int64{dayOnly}, []int64{flagOnlyM}, nil))
}

// Place before status (#6371): a checked-in child counts as present only. Its
// sick day or flag is a hint and leaves the sick figure and the sick-rate alert.
func TestSchoolFiguresCountACheckedInSickChildAsPresentOnly(t *testing.T) {
	t.Parallel()
	const (
		sickHome, sickHere, healthyHere    int64 = 10, 11, 12
		sickHomeM, sickHereM, healthyHereM int64 = 20, 21, 22
	)
	roster := map[int64]int64{sickHome: sickHomeM, sickHere: sickHereM, healthyHere: healthyHereM}
	present := []int64{sickHere, healthyHere}

	assert.Equal(t, 1, countSick(roster, []int64{sickHome, sickHere}, nil, present), "sick day")
	assert.Equal(t, 1, countSick(roster, nil, []int64{sickHomeM, sickHereM}, present), "sick flag")
	assert.Equal(t, len(roster), countStudents(roster, present)+countSick(roster, []int64{sickHome, sickHere}, []int64{sickHereM}, present),
		"present and sick name every child once")
}

func TestCountMembersKeepsOnlyTheRoster(t *testing.T) {
	t.Parallel()
	const onSite, away, leaver int64 = 30, 31, 32

	assert.Equal(t, 1, countMembers([]int64{onSite, away}, []int64{onSite, leaver}))
}
