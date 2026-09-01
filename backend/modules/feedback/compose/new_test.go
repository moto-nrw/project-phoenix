package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var fixedToday = feedback.Date("2026-08-31")

type settingsStub struct {
	enabled      bool
	retention    int
	enabledErr   error
	retentionErr error
}

func (s settingsStub) FeedbackEnabled(context.Context) (bool, error) { return s.enabled, s.enabledErr }
func (s settingsStub) FeedbackRetentionDays(context.Context) (int, error) {
	return s.retention, s.retentionErr
}

func buildModule(t *testing.T, db *bun.DB, settings settingsStub, observations ...func(Observation)) *feedback.Module {
	t.Helper()
	observe := func(Observation) {}
	if len(observations) > 0 {
		observe = observations[0]
	}
	module, err := New(Dependencies{DB: db, Settings: settings, Today: func() feedback.Date { return fixedToday }, Observe: observe})
	require.NoError(t, err)
	return module
}

func createInput(studentID int64, day feedback.Date, value string) feedback.CreateEntry {
	return feedback.CreateEntry{Value: value, Day: day, Time: "12:34:56", StudentID: studentID}
}

func TestModulePersistsReadsCountsAndDeletesOneTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Feedback", "Module", "1a")
	module := buildModule(t, db, settingsStub{enabled: true, retention: 90})
	ctx := testpkg.Ctx(t)

	created, err := module.Submit(ctx, createInput(student.ID, "2026-08-31", feedback.ValuePositive))
	require.NoError(t, err)
	assert.Positive(t, created.ID)

	entries, err := module.FindEntries(ctx, feedback.Filter{StudentID: &student.ID})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, feedback.Date("2026-08-31"), entries[0].Day)
	assert.Equal(t, "12:34:56", entries[0].Time)

	count, err := module.CountForStudent(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	require.NoError(t, module.EraseEntry(ctx, created.ID))
	_, err = module.LookupEntry(ctx, created.ID)
	require.ErrorIs(t, err, feedback.ErrEntryNotFound)
}

func TestModuleRLSHidesAnotherTenantsFeedback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, settingsStub{enabled: true, retention: 90})
	student := testpkg.CreateTestStudent(t, db, "Feedback", "TenantA", "1a")
	entry, err := module.Submit(testpkg.Ctx(t), createInput(student.ID, "2026-08-31", feedback.ValuePositive))
	require.NoError(t, err)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), otherTenantID)
	_, err = module.LookupEntry(otherCtx, entry.ID)
	require.ErrorIs(t, err, feedback.ErrEntryNotFound)
	entries, err := module.FindEntries(otherCtx, feedback.Filter{})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestModuleCreateRollsBackWithOuterTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Feedback", "Rollback", "1a")
	module := buildModule(t, db, settingsStub{enabled: true, retention: 90})
	wantErr := errors.New("abort outer transaction")

	err := tenant.WithinCurrentTenant(testpkg.Ctx(t), func(txCtx context.Context) error {
		_, createErr := module.Submit(txCtx, createInput(student.ID, "2026-08-31", feedback.ValueNeutral))
		require.NoError(t, createErr)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	entries, err := module.FindEntries(testpkg.Ctx(t), feedback.Filter{})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestModuleBatchFailureRollsBackEveryEntry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Feedback", "BatchRollback", "1a")
	module := buildModule(t, db, settingsStub{enabled: true, retention: 90})

	_, err := module.SubmitBatch(testpkg.Ctx(t), []feedback.CreateEntry{
		createInput(student.ID, "2026-08-31", feedback.ValuePositive),
		createInput(9_223_372_036_854_775_000, "2026-08-31", feedback.ValueNegative),
	})
	require.Error(t, err)
	var batchErr *feedback.BatchOperationError
	assert.NotErrorAs(t, err, &batchErr)

	entries, listErr := module.FindEntries(testpkg.Ctx(t), feedback.Filter{StudentID: &student.ID})
	require.NoError(t, listErr)
	assert.Empty(t, entries)
}

func TestModuleKeepsSettingsAndPersistenceErrorsVisible(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	settingsErr := errors.New("settings unavailable")
	module := buildModule(t, db, settingsStub{enabledErr: settingsErr, retention: 90})

	_, err := module.Available(testpkg.Ctx(t))
	require.ErrorIs(t, err, settingsErr)

	missingTenantID := testpkg.UniqueTestTenantID(t)
	missingTenantCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), missingTenantID)
	_, err = module.Submit(missingTenantCtx, createInput(123, "2026-08-31", feedback.ValueNegative))
	require.ErrorContains(t, err, "feedback postgres: insert entry")
	assert.NotErrorIs(t, err, feedback.ErrInvalidEntryData)
}

func TestModuleRetentionUsesSettingAndRollsBackAtomically(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Feedback", "Retention", "1a")
	module := buildModule(t, db, settingsStub{enabled: true, retention: 30})
	ctx := testpkg.Ctx(t)
	_, err := module.Submit(ctx, createInput(student.ID, "2026-07-01", feedback.ValueNegative))
	require.NoError(t, err)
	_, err = module.Submit(ctx, createInput(student.ID, "2026-08-15", feedback.ValueNeutral))
	require.NoError(t, err)

	wantErr := errors.New("abort cleanup")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		rows, cleanupErr := module.DeleteExpired(txCtx)
		require.NoError(t, cleanupErr)
		assert.Equal(t, 1, rows)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	entries, err := module.FindEntries(ctx, feedback.Filter{})
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestModuleRetentionDoesNotFallbackWhenSettingFails(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	wantErr := errors.New("retention setting unavailable")
	module := buildModule(t, db, settingsStub{enabled: true, retentionErr: wantErr})

	_, err := module.DeleteExpired(testpkg.Ctx(t))

	require.ErrorIs(t, err, wantErr)
}

func TestModuleReportsRetentionRowsAndStatementDuration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Feedback", "Metrics", "1a")
	var observations []Observation
	module := buildModule(t, db, settingsStub{enabled: true, retention: 30}, func(observation Observation) {
		observations = append(observations, observation)
	})
	_, err := module.Submit(testpkg.Ctx(t), createInput(student.ID, "2026-07-01", feedback.ValuePositive))
	require.NoError(t, err)

	rows, err := module.DeleteExpired(testpkg.Ctx(t))
	require.NoError(t, err)
	assert.Equal(t, 1, rows)
	require.Len(t, observations, 2)
	assert.Equal(t, "retention_cleanup", observations[1].Operation)
	assert.EqualValues(t, 1, observations[1].Stats.Rows)
	assert.Positive(t, observations[1].Stats.StatementDuration)
}

func TestModuleObservesMappedStableErrorCodeAndStatusClass(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var observations []Observation
	module := buildModule(t, db, settingsStub{enabled: true, retention: 30}, func(observation Observation) {
		observations = append(observations, observation)
	})

	_, err := module.LookupEntry(testpkg.Ctx(t), 9_223_372_036_854_775_000)
	require.ErrorIs(t, err, feedback.ErrEntryNotFound)
	require.Len(t, observations, 1)
	assert.ErrorIs(t, observations[0].Err, feedback.ErrEntryNotFound)
	assert.Equal(t, "entry_not_found", feedback.ErrorCode(observations[0].Err))
}
