package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleOwnsRecurrenceRuleLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	endDate := time.Date(2027, time.June, 30, 0, 0, 0, 0, time.UTC)

	created := createOwnedRecurrenceRule(t, module, ctx, timetable.RecurrenceRuleInput{
		Frequency: "weekly", IntervalCount: 2, Weekdays: []string{"MON", "WED"}, EndDate: &endDate,
	})
	found, err := module.FindRecurrenceRule(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"MON", "WED"}, found.Weekdays)

	byFrequency, err := module.ListRecurrenceRules(ctx, timetable.RecurrenceRuleFilter{Frequency: "WEEKLY"})
	require.NoError(t, err)
	assert.Equal(t, []int64{created.ID}, recurrenceRuleIDs(byFrequency))
	byWeekday, err := module.ListRecurrenceRules(ctx, timetable.RecurrenceRuleFilter{Weekday: "WED"})
	require.NoError(t, err)
	assert.Equal(t, []int64{created.ID}, recurrenceRuleIDs(byWeekday))
	activeAt := endDate.Add(-24 * time.Hour)
	expiredAt := activeAt.Add(-24 * time.Hour)
	createOwnedRecurrenceRule(t, module, ctx, timetable.RecurrenceRuleInput{
		Frequency: "daily", IntervalCount: 1, EndDate: &expiredAt,
	})
	active, err := module.ListRecurrenceRules(ctx, timetable.RecurrenceRuleFilter{ActiveAt: &activeAt})
	require.NoError(t, err)
	assert.Equal(t, []int64{created.ID}, recurrenceRuleIDs(active))

	count := 4
	updated, err := module.UpdateRecurrenceRule(ctx, created.ID, timetable.RecurrenceRuleInput{
		Frequency: "monthly", IntervalCount: 1, MonthDays: []int{1, 15}, Count: &count,
	})
	require.NoError(t, err)
	assert.Equal(t, []int{1, 15}, updated.MonthDays)
	assert.Nil(t, updated.EndDate)
	assert.True(t, updated.UpdatedAt.After(created.UpdatedAt))

	require.NoError(t, module.DeleteRecurrenceRule(ctx, created.ID))
	_, err = module.FindRecurrenceRule(ctx, created.ID)
	require.ErrorIs(t, err, timetable.ErrRecurrenceRuleNotFound)
}

func TestModuleRecurrenceRulesAreTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	owned := createOwnedRecurrenceRule(t, module, ctx, dailyRecurrenceRuleInput())

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)
	foreign := createOwnedRecurrenceRule(t, module, foreignCtx, dailyRecurrenceRuleInput())

	_, err := module.FindRecurrenceRule(foreignCtx, owned.ID)
	require.ErrorIs(t, err, timetable.ErrRecurrenceRuleNotFound)
	listed, err := module.ListRecurrenceRules(ctx, timetable.RecurrenceRuleFilter{})
	require.NoError(t, err)
	assert.Contains(t, recurrenceRuleIDs(listed), owned.ID)
	assert.NotContains(t, recurrenceRuleIDs(listed), foreign.ID)
	_, err = module.UpdateRecurrenceRule(foreignCtx, owned.ID, dailyRecurrenceRuleInput())
	require.ErrorIs(t, err, timetable.ErrRecurrenceRuleNotFound)
	require.NoError(t, module.DeleteRecurrenceRule(foreignCtx, owned.ID))
	_, err = module.FindRecurrenceRule(ctx, owned.ID)
	require.NoError(t, err)
}

func TestModuleRecurrenceRuleReadFailuresAreNotSwallowed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, err := module.FindRecurrenceRule(ctx, 1)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.ListRecurrenceRules(ctx, timetable.RecurrenceRuleFilter{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestModuleRecurrenceRuleWritesParticipateInCallerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	wantErr := errors.New("abort recurrence rule write")
	var rolledBackID int64

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		created, createErr := module.CreateRecurrenceRule(txCtx, dailyRecurrenceRuleInput())
		rolledBackID = created.ID
		if createErr != nil {
			return createErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	_, err = module.FindRecurrenceRule(ctx, rolledBackID)
	require.ErrorIs(t, err, timetable.ErrRecurrenceRuleNotFound)

	rule := createOwnedRecurrenceRule(t, module, ctx, dailyRecurrenceRuleInput())
	requireRecurrenceRuleRollback(t, ctx, func(txCtx context.Context) error {
		_, updateErr := module.UpdateRecurrenceRule(txCtx, rule.ID, timetable.RecurrenceRuleInput{
			Frequency: "yearly", IntervalCount: 1,
		})
		return updateErr
	})
	stored, err := module.FindRecurrenceRule(ctx, rule.ID)
	require.NoError(t, err)
	assert.Equal(t, "daily", stored.Frequency)
	_, err = module.UpdateRecurrenceRule(ctx, rule.ID, timetable.RecurrenceRuleInput{
		Frequency: "yearly", IntervalCount: 1,
	})
	require.NoError(t, err)

	requireRecurrenceRuleRollback(t, ctx, func(txCtx context.Context) error {
		return module.DeleteRecurrenceRule(txCtx, rule.ID)
	})
	_, err = module.FindRecurrenceRule(ctx, rule.ID)
	require.NoError(t, err)
	require.NoError(t, module.DeleteRecurrenceRule(ctx, rule.ID))
	_, err = module.FindRecurrenceRule(ctx, rule.ID)
	require.ErrorIs(t, err, timetable.ErrRecurrenceRuleNotFound)
}

func TestRecurrenceRuleListQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	for range 8 {
		createOwnedRecurrenceRule(t, module, ctx, dailyRecurrenceRuleInput())
	}
	counter := testpkg.CaptureQueries(t, db)
	_, err := module.ListRecurrenceRules(ctx, timetable.RecurrenceRuleFilter{})
	require.NoError(t, err)
	testpkg.AssertQueryBudget(t, "modules.timetable.recurrence_rules.list", counter.Queries())
}

func createOwnedRecurrenceRule(t *testing.T, module *timetable.Module, ctx context.Context, input timetable.RecurrenceRuleInput) timetable.RecurrenceRule {
	t.Helper()
	value, err := module.CreateRecurrenceRule(ctx, input)
	require.NoError(t, err)
	return value
}

func dailyRecurrenceRuleInput() timetable.RecurrenceRuleInput {
	return timetable.RecurrenceRuleInput{Frequency: "daily", IntervalCount: 1}
}

func recurrenceRuleIDs(values []timetable.RecurrenceRule) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}

func requireRecurrenceRuleRollback(t *testing.T, ctx context.Context, write func(context.Context) error) {
	t.Helper()
	wantErr := errors.New("abort recurrence rule write")
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if err := write(txCtx); err != nil {
			return err
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
}
