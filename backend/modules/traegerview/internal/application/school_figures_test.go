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

	assert.Equal(t, 3, countSick(roster, []int64{bothMarks, dayOnly, endedCare}, []int64{bothMarksM, flagOnlyM, endedCareM}))
	assert.Zero(t, countSick(nil, []int64{dayOnly}, []int64{flagOnlyM}))
}

func TestCountMembersKeepsOnlyTheRoster(t *testing.T) {
	t.Parallel()
	const onSite, away, leaver int64 = 30, 31, 32

	assert.Equal(t, 1, countMembers([]int64{onSite, away}, []int64{onSite, leaver}))
}
