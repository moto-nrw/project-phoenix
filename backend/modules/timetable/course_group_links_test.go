package timetable

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type courseGroupList struct {
	filter CourseGroupFilter
	groups []CourseGroup
}

func (l *courseGroupList) ListCourseGroups(_ context.Context, filter CourseGroupFilter) ([]CourseGroup, error) {
	l.filter = filter
	return l.groups, nil
}

func TestCourseGroupsForOfferingsResolvesBothLinkShapes(t *testing.T) {
	t.Parallel()
	legacy := int64(70)
	list := &courseGroupList{groups: []CourseGroup{
		{ID: legacy, MaxParticipants: 12},
		{ID: 71, SourceCareOfferingIDs: []int64{5, 99}},
	}}
	result, err := CourseGroupsForOfferings(context.Background(), list, []CourseOfferingLink{
		{OfferingID: 4, ActivityGroupID: &legacy}, {OfferingID: 5}, {OfferingID: 0, ActivityGroupID: &legacy},
	}, calendar.Date("2026-10-01"))
	require.NoError(t, err)
	assert.Equal(t, CourseGroupFilter{LegacyGroupIDs: []int64{legacy}, SourceOfferingIDs: []int64{4, 5}, EffectiveOn: "2026-10-01"}, list.filter)
	require.Len(t, result[4], 1)
	assert.Equal(t, legacy, result[4][0].ID)
	require.Len(t, result[5], 1)
	assert.Equal(t, int64(71), result[5][0].ID)
	assert.NotContains(t, result, int64(99), "an offering nobody asked for is not linked")
}

func TestCourseGroupParticipantLimitTreatsZeroAsUnlimited(t *testing.T) {
	t.Parallel()
	assert.Nil(t, CourseGroup{}.ParticipantLimit())
	limit := CourseGroup{MaxParticipants: 12}.ParticipantLimit()
	require.NotNil(t, limit)
	assert.Equal(t, 12, *limit)
}
