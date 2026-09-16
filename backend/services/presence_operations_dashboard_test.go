package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
)

type dashboardActiveFake struct {
	active.Service
	analytics *active.DashboardAnalytics
}

func (f dashboardActiveFake) GetDashboardAnalytics(context.Context) (*active.DashboardAnalytics, error) {
	return f.analytics, nil
}

type atSchoolCounterFake struct {
	count int
	err   error
	seen  []int64
}

func (f *atSchoolCounterFake) CountAtSchoolToday(_ context.Context, ids []int64) (int, error) {
	f.seen = ids
	return f.count, f.err
}

// TestDashboardAnalyticsSplitsSchoolFromHome pins #3260: the children still in
// class leave the "Zuhause" figure for their own one.
func TestDashboardAnalyticsSplitsSchoolFromHome(t *testing.T) {
	t.Parallel()

	counter := &atSchoolCounterFake{count: 3}
	ops := NewPresenceOperations(dashboardActiveFake{analytics: &active.DashboardAnalytics{
		StudentsHome: 5, HomeCandidateIDs: []int64{1, 2, 3, 4, 5},
	}}, counter)

	got, err := ops.DashboardAnalytics(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2, 3, 4, 5}, counter.seen)
	assert.Equal(t, 3, got.StudentsAtSchool)
	assert.Equal(t, 2, got.StudentsHome)
}

func TestDashboardAnalyticsClampsHomeAtZero(t *testing.T) {
	t.Parallel()

	// StudentsHome is clamped upstream and can be lower than the candidates.
	ops := NewPresenceOperations(dashboardActiveFake{analytics: &active.DashboardAnalytics{
		StudentsHome: 1, HomeCandidateIDs: []int64{1, 2},
	}}, &atSchoolCounterFake{count: 2})

	got, err := ops.DashboardAnalytics(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, got.StudentsAtSchool)
	assert.Equal(t, 0, got.StudentsHome)
}

func TestDashboardAnalyticsSkipsTheCounterWithoutCandidates(t *testing.T) {
	t.Parallel()

	counter := &atSchoolCounterFake{count: 9}
	ops := NewPresenceOperations(dashboardActiveFake{analytics: &active.DashboardAnalytics{StudentsHome: 0}}, counter)

	got, err := ops.DashboardAnalytics(context.Background())
	require.NoError(t, err)
	assert.Nil(t, counter.seen)
	assert.Equal(t, 0, got.StudentsAtSchool)
}

func TestDashboardAnalyticsFailsWhenTheCounterFails(t *testing.T) {
	t.Parallel()

	boom := errors.New("planning down")
	ops := NewPresenceOperations(dashboardActiveFake{analytics: &active.DashboardAnalytics{
		StudentsHome: 1, HomeCandidateIDs: []int64{1},
	}}, &atSchoolCounterFake{err: boom})

	_, err := ops.DashboardAnalytics(context.Background())
	require.ErrorIs(t, err, boom)
}
