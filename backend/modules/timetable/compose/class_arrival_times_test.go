package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestClassArrivalQueriesNormalizeScopeAndPropagateFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	row := testpkg.CreateTestClassArrivalTime(t, db, " 1A ", map[string]string{"mon": "12:15", "fri": "11:45"})
	testpkg.CreateTestClassArrivalTime(t, db, "2b", map[string]string{"mon": "13:00"})
	var observations []Observation
	query, err := NewClassArrivalQueries(db, func(value Observation) { observations = append(observations, value) })
	require.NoError(t, err)
	times, err := query.ListClassArrivalTimes(testpkg.Ctx(t), []string{"1a", " 1A ", "", "missing"})
	require.NoError(t, err)
	require.Equal(t, map[string]map[string]string{"1a": row.ArrivalTimes}, times)
	require.Len(t, observations, 1)
	require.EqualValues(t, 1, observations[0].Stats.Queries)
	require.EqualValues(t, 1, observations[0].Stats.Rows)
	times, err = query.ListClassArrivalTimes(testpkg.Ctx(t), []string{" "})
	require.NoError(t, err)
	require.Empty(t, times)
	require.Len(t, observations, 1, "empty reads must not touch the database")
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	times, err = query.ListClassArrivalTimes(tenant.WithTenantID(testpkg.Ctx(t), otherTenant), []string{"1a"})
	require.NoError(t, err)
	require.Empty(t, times)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()
	_, err = query.ListClassArrivalTimes(ctx, []string{"1a"})
	require.ErrorIs(t, err, context.Canceled)
	_, err = NewClassArrivalQueries(nil, func(Observation) {})
	require.Error(t, err)
}
