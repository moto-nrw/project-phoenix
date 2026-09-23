package compose

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestNativeClassArrivalPlansUpsertClearAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	commands, err := NewClassArrivals(db, func(Observation) {})
	require.NoError(t, err)
	input := timetable.ClassArrivalPlanInput{SchoolClass: " 3A ", ArrivalTimes: map[string]string{"mon": "12:15", "fri": "11:45"}}
	first, err := commands.UpsertClassArrivalPlan(ctx, input)
	require.NoError(t, err)
	require.Positive(t, first.ID)
	require.Equal(t, testpkg.Tenant(t), first.TenantID)
	require.Equal(t, input.ArrivalTimes, first.ArrivalTimes)
	abort := errors.New("rollback class timetable update")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if lockErr := commands.LockClassArrivalPlan(txCtx, "3a"); lockErr != nil {
			return lockErr
		}
		_, writeErr := commands.UpsertClassArrivalPlan(txCtx, timetable.ClassArrivalPlanInput{SchoolClass: "3a"})
		if writeErr != nil {
			return writeErr
		}
		return abort
	})
	require.ErrorIs(t, err, abort)
	rows, err := commands.ListClassArrivalPlans(ctx, []string{"3a"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, first.ArrivalTimes, rows[0].ArrivalTimes)
	cleared, err := commands.UpsertClassArrivalPlan(ctx, timetable.ClassArrivalPlanInput{SchoolClass: "3a"})
	require.NoError(t, err)
	require.Equal(t, first.ID, cleared.ID)
	require.Empty(t, cleared.ArrivalTimes)
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	other, err := commands.UpsertClassArrivalPlan(tenant.WithTenantID(ctx, otherTenant), input)
	require.NoError(t, err)
	require.Equal(t, otherTenant, other.TenantID)
	require.NotEqual(t, first.ID, other.ID)
	input.ArrivalTimes = map[string]string{"sat": "12:15"}
	_, err = commands.UpsertClassArrivalPlan(ctx, input)
	require.ErrorIs(t, err, timetable.ErrInvalidClassArrivalPlan)
}

func TestNativeClassArrivalPlanLockUsesRetainedKey(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	commands, err := NewClassArrivals(db, func(Observation) {})
	require.NoError(t, err)
	key := fmt.Sprintf("class-arrival:%d:3a", testpkg.Tenant(t))
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if lockErr := commands.LockClassArrivalPlan(txCtx, " 3A "); lockErr != nil {
			return lockErr
		}
		var acquired bool
		readErr := db.NewRaw("SELECT pg_try_advisory_xact_lock(hashtext(?))", key).Scan(ctx, &acquired)
		require.NoError(t, readErr)
		require.False(t, acquired, "another connection must not acquire the retained class key")
		return nil
	})
	require.NoError(t, err)
	var acquired bool
	require.NoError(t, db.NewRaw("SELECT pg_try_advisory_xact_lock(hashtext(?))", key).Scan(ctx, &acquired))
	require.True(t, acquired, "the class lock must release with the transaction")
}

func TestNativeClassArrivalCommandsUpsertAndDeleteWithinTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	commands, err := NewClassArrivals(db, func(Observation) {})
	require.NoError(t, err)
	input := timetable.ClassArrivalExceptionInput{SchoolClass: " 3A ", Date: "2026-09-21",
		ArrivalTime: time.Date(2026, 9, 21, 12, 15, 0, 0, time.FixedZone("UTC+2", 2*60*60))}
	first, err := commands.UpsertClassArrivalException(ctx, input)
	require.NoError(t, err)
	require.Positive(t, first.ID)
	require.Equal(t, testpkg.Tenant(t), first.TenantID)
	require.Equal(t, "ogs", first.Origin)
	require.Equal(t, "12:15", first.ArrivalTime.Format("15:04"), "wall-clock time must not shift through UTC")
	input.SchoolClass, input.Origin = "3a", "school"
	input.ArrivalTime = input.ArrivalTime.Add(time.Hour)
	updated, err := commands.UpsertClassArrivalException(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first.ID, updated.ID, "normalized class and date identify one row")
	require.Equal(t, "school", updated.Origin)
	require.Equal(t, "13:15", updated.ArrivalTime.Format("15:04"))
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	otherCtx := tenant.WithTenantID(ctx, otherTenant)
	other, err := commands.UpsertClassArrivalException(otherCtx, input)
	require.NoError(t, err)
	require.Equal(t, otherTenant, other.TenantID)
	require.NotEqual(t, first.ID, other.ID)
	deleted, err := commands.DeleteClassArrivalException(otherCtx, " 3A ", input.Date)
	require.NoError(t, err)
	require.True(t, deleted)
	rows, err := commands.ListClassArrivalExceptions(ctx, []string{"3a"}, input.Date, input.Date)
	require.NoError(t, err)
	require.Len(t, rows, 1, "deleting another tenant's row must not affect this tenant")
	require.Equal(t, first.ID, rows[0].ID)
	deleted, err = commands.DeleteClassArrivalException(ctx, "3a", input.Date)
	require.NoError(t, err)
	require.True(t, deleted)
	deleted, err = commands.DeleteClassArrivalException(ctx, "3a", input.Date)
	require.NoError(t, err)
	require.False(t, deleted)
	input.Date = "invalid"
	_, err = commands.UpsertClassArrivalException(ctx, input)
	require.ErrorIs(t, err, timetable.ErrInvalidClassArrivalException)
	_, err = commands.DeleteClassArrivalException(ctx, "3a", input.Date)
	require.ErrorIs(t, err, timetable.ErrInvalidClassArrivalException)
}

func TestNativeClassArrivalExceptionsKeepDatesAndTenantScope(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	clock := time.Date(1, 1, 1, 12, 15, 0, 0, time.UTC)
	row := &schedule.ClassArrivalException{SchoolClass: " 3A ", Date: "2026-09-21", ArrivalTime: clock, Origin: "school"}
	require.NoError(t, NewClassArrivalExceptionRepository(db).Upsert(ctx, row))
	var observations []Observation
	query, err := NewClassArrivalQueries(db, func(value Observation) { observations = append(observations, value) })
	require.NoError(t, err)
	rows, err := query.ListClassArrivalExceptions(ctx, []string{"3a", " 3A ", "missing"}, "2026-09-21", "2026-09-21")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, row.ID, rows[0].ID)
	require.Equal(t, row.TenantID, rows[0].TenantID)
	require.Equal(t, row.SchoolClass, rows[0].SchoolClass)
	require.Equal(t, "2026-09-21", rows[0].Date)
	require.Equal(t, "12:15", rows[0].ArrivalTime.Format("15:04"))
	require.Equal(t, "school", rows[0].Origin)
	require.Len(t, observations, 1)
	require.EqualValues(t, 1, observations[0].Stats.Queries)
	require.EqualValues(t, 1, observations[0].Stats.Rows)
	rows, err = query.ListClassArrivalExceptions(ctx, nil, "2026-09-21", "2026-09-21")
	require.NoError(t, err)
	require.Empty(t, rows)
	rows, err = query.ListClassArrivalExceptions(ctx, []string{"3a"}, "2026-09-22", "2026-09-21")
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = query.ListClassArrivalExceptions(ctx, []string{"3a"}, "invalid", "2026-09-21")
	require.ErrorIs(t, err, timetable.ErrInvalidClassArrivalExceptionRange)
	require.Len(t, observations, 1, "empty and invalid ranges must not read storage")
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	rows, err = query.ListClassArrivalExceptions(tenant.WithTenantID(ctx, otherTenant), []string{"3a"}, "2026-09-21", "2026-09-21")
	require.NoError(t, err)
	require.Empty(t, rows)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = query.ListClassArrivalExceptions(canceled, []string{"3a"}, "2026-09-21", "2026-09-21")
	require.ErrorIs(t, err, context.Canceled)
}

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

func TestClassArrivalPlansPreserveDisplayLabelsWithinTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	row := testpkg.CreateTestClassArrivalTime(t, db, " 3A ", map[string]string{"mon": "12:15"})
	testpkg.CreateTestClassArrivalTime(t, db, "4b", map[string]string{"fri": "13:00"})
	var observations []Observation
	query, err := NewClassArrivalQueries(db, func(value Observation) { observations = append(observations, value) })
	require.NoError(t, err)
	plans, err := query.ListClassArrivalPlans(testpkg.Ctx(t), []string{"3a", " 3A ", "", "missing"})
	require.NoError(t, err)
	require.Len(t, plans, 1)
	require.Equal(t, row.ID, plans[0].ID)
	require.Equal(t, row.TenantID, plans[0].TenantID)
	require.Equal(t, row.SchoolClass, plans[0].SchoolClass)
	require.Equal(t, row.ArrivalTimes, plans[0].ArrivalTimes)
	require.True(t, row.CreatedAt.Equal(plans[0].CreatedAt))
	require.True(t, row.UpdatedAt.Equal(plans[0].UpdatedAt))
	require.Equal(t, row.UpdatedBy, plans[0].UpdatedBy)
	require.Len(t, observations, 1)
	require.EqualValues(t, 1, observations[0].Stats.Queries)
	plans, err = query.ListClassArrivalPlans(testpkg.Ctx(t), []string{" "})
	require.NoError(t, err)
	require.Empty(t, plans)
	require.Len(t, observations, 1)
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	plans, err = query.ListClassArrivalPlans(tenant.WithTenantID(testpkg.Ctx(t), otherTenant), []string{"3a"})
	require.NoError(t, err)
	require.Empty(t, plans)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()
	_, err = query.ListClassArrivalPlans(ctx, []string{"3a"})
	require.ErrorIs(t, err, context.Canceled)
}
