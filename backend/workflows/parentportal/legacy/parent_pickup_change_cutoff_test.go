package legacy_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal/legacy"
)

// #3163 fixtures: the service clock stands on 24 August 2026 (a Monday) at a
// chosen Berlin wall-clock time, and the request service shares that day.
var cutoffToday = timezone.NewDate(2026, 8, 24)

func berlinClock(hour, minute int) func() time.Time {
	return func() time.Time { return time.Date(2026, 8, 24, hour, minute, 0, 0, timezone.Berlin) }
}

func cutoffStub(enabled bool, clock string) parentSettingsStub {
	return parentSettingsStub{
		boolValues: map[string]bool{configModels.KeyParentPickupChangeEnabled: enabled},
		stringValues: map[string]string{
			configModels.KeyGuardianParentInviteMode:     configModels.ParentInviteModeDisabled,
			configModels.KeyParentPickupChangeCutoffTime: clock,
		},
	}
}

type cutoffServiceDeps struct {
	db    *bun.DB
	repos *repositories.Factory
	base  parentService.ServiceConfig
}

// newCutoffServiceDeps reuses the pickup-change wiring with the real request
// service, whose today is pinned to the fixture day.
func newCutoffServiceDeps(t *testing.T) cutoffServiceDeps {
	t.Helper()
	var base parentService.ServiceConfig
	pinToday := services.WithCareRequestToday(func() timezone.Date { return cutoffToday })
	_, db, repos := buildPickupChangeServiceWithRequestOptions(t, []services.CareRequestOption{pinToday}, func(cfg *parentService.ServiceConfig) {
		base = *cfg
	})
	return cutoffServiceDeps{db: db, repos: repos, base: base}
}

func (d cutoffServiceDeps) service(t *testing.T, settings configService.SettingsService, now func() time.Time) parentService.Service {
	t.Helper()
	cfg := d.base
	cfg.CareExceptions = d.repos.CarePlan()
	cfg.Settings = settings
	cfg.MealPlan = availableMealPlan(false)
	cfg.Broadcaster = testpkg.NewRecordingBroadcaster()
	cfg.Now = now
	return parentService.NewService(cfg)
}

func cutoffPickupTime() time.Time {
	return timezone.NormalizeWallClock(time.Date(2026, 1, 1, 14, 30, 0, 0, time.UTC))
}

// seedGuardianPickupException stores an already effective guardian pickup
// exception, the state DELETE .../care-exception removes.
func seedGuardianPickupException(t *testing.T, d cutoffServiceDeps, chain testpkg.ParentChain, date timezone.Date) {
	t.Helper()
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID)
	guardianID := chain.AccountID
	exception := &scheduleModels.StudentPickupException{
		StudentID:         chain.StudentID,
		ExceptionDate:     scheduleModels.Date(date),
		PickupTime:        wallClock(14, 30),
		Source:            scheduleModels.ExceptionSourceGuardian,
		CreatedByGuardian: &guardianID,
	}
	exception.SetTenantID(chain.TenantID)
	require.NoError(t, d.repos.StudentPickupException.Create(ctx, exception))
}

func guardianPickupExists(t *testing.T, d cutoffServiceDeps, chain testpkg.ParentChain, date timezone.Date) bool {
	t.Helper()
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID)
	row, err := d.repos.StudentPickupException.FindByStudentIDAndDate(ctx, chain.StudentID, scheduleModels.Date(date))
	require.NoError(t, err)
	return row != nil
}

// The parent submit resolves the school's cutoff for the child and hands it to
// the request service; after the cutoff today is refused with the portal's
// own sentinel, later days go through.
func TestSubmitPickupChangeRequestHonoursSameDayCutoff(t *testing.T) {
	t.Parallel()

	d := newCutoffServiceDeps(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	for _, tc := range []struct {
		name    string
		now     func() time.Time
		date    timezone.Date
		clock   string
		enabled bool
		wantErr error
	}{
		{"heute 10:59", berlinClock(10, 59), cutoffToday, "11:00", true, nil},
		{"heute genau 11:00", berlinClock(11, 0), cutoffToday, "11:00", true, nil},
		{"heute 11:01", berlinClock(11, 1), cutoffToday, "11:00", true, parentService.ErrPickupChangeCutoffPassed},
		{"morgen 11:01", berlinClock(11, 1), cutoffToday.AddDays(1), "11:00", true, nil},
		{"ohne Frist heute 23:00", berlinClock(23, 0), cutoffToday, "", true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chain := testpkg.CreateTestParentGuardianChain(t, d.db)
			svc := d.service(t, cutoffStub(tc.enabled, tc.clock), tc.now)
			_, err := svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, tc.date, cutoffPickupTime(), "Arzttermin", nil)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

// The cutoff is read after the transaction has acquired its locks. A request
// that was open when it began must still be refused if the deadline passes
// before the pickup-change row is validated.
func TestSubmitPickupChangeRequestRechecksCutoffAtWriteBoundary(t *testing.T) {
	t.Parallel()

	d := newCutoffServiceDeps(t)
	chain := testpkg.CreateTestParentGuardianChain(t, d.db)
	now := berlinClock(10, 59)
	settings := cutoffStub(true, "11:00")
	settings.stringInTxFn = func(key string) (string, error) {
		if key == configModels.KeyParentPickupChangeEnabled {
			now = berlinClock(11, 1)
			return "true", nil
		}
		return settings.stringValues[key], nil
	}

	_, err := d.service(t, settings, func() time.Time { return now() }).SubmitPickupChangeRequest(
		testpkg.WithPackageTenantRuntime(context.Background()),
		chain.AccountID,
		chain.StudentID,
		cutoffToday,
		cutoffPickupTime(),
		"Arzttermin",
		nil,
	)
	require.ErrorIs(t, err, parentService.ErrPickupChangeCutoffPassed)
}

// A switch-off that commits after the initial feature gate but before the
// parent write reaches its transaction must still reject the new request.
func TestSubmitPickupChangeRequestRechecksEnablementAtWriteBoundary(t *testing.T) {
	t.Parallel()

	d := newCutoffServiceDeps(t)
	chain := testpkg.CreateTestParentGuardianChain(t, d.db)
	lockTaken := false
	settings := cutoffStub(true, "11:00")
	settings.stringInTxFn = func(key string) (string, error) {
		if key == configModels.KeyParentPickupChangeEnabled {
			return "false", nil
		}
		return settings.stringValues[key], nil
	}
	settings.lockPickupChangePolicyFn = func(ctx context.Context, tenantID int64) error {
		_, inTx := tenant.TransactionFromContext(ctx)
		require.True(t, inTx, "the policy lock must be held by the parent write transaction")
		require.Equal(t, chain.TenantID, tenantID)
		lockTaken = true
		return nil
	}

	_, err := d.service(t, settings, berlinClock(10, 59)).SubmitPickupChangeRequest(
		testpkg.WithPackageTenantRuntime(context.Background()),
		chain.AccountID,
		chain.StudentID,
		cutoffToday,
		cutoffPickupTime(),
		"Arzttermin",
		nil,
	)
	require.ErrorIs(t, err, parentService.ErrPickupChangeDisabled)
	assert.True(t, lockTaken)
}

// The unrouted direct exception path is a guardian write too, so it must not
// commit after the feature switch changed between its first check and write.
func TestSubmitCareExceptionRechecksEnablementAtWriteBoundary(t *testing.T) {
	t.Parallel()

	d := newCutoffServiceDeps(t)
	chain := testpkg.CreateTestParentGuardianChain(t, d.db)
	settings := cutoffStub(true, "11:00")
	settings.stringInTxFn = func(key string) (string, error) {
		if key == configModels.KeyParentPickupChangeEnabled {
			return "false", nil
		}
		return settings.stringValues[key], nil
	}
	pickupTime := cutoffPickupTime()

	_, err := d.service(t, settings, berlinClock(10, 59)).SubmitCareExceptionWithReason(
		testpkg.WithPackageTenantRuntime(context.Background()),
		chain.AccountID,
		chain.StudentID,
		cutoffToday,
		&pickupTime,
		"Arzttermin",
	)
	require.ErrorIs(t, err, parentService.ErrPickupChangeDisabled)
}

// A request that came in before the cutoff cannot be edited afterwards; the
// guardian's today is closed completely.
func TestEditPickupChangeRequestHonoursSameDayCutoff(t *testing.T) {
	t.Parallel()

	d := newCutoffServiceDeps(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, d.db)
	settings := cutoffStub(true, "11:00")

	created, err := d.service(t, settings, berlinClock(9, 30)).
		SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, cutoffToday, cutoffPickupTime(), "Arzttermin", nil)
	require.NoError(t, err)

	later := d.service(t, settings, berlinClock(11, 1))
	_, err = later.EditPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, created.ID,
		cutoffToday, cutoffPickupTime().Add(time.Hour), "Termin verschoben", "")
	require.ErrorIs(t, err, parentService.ErrPickupChangeCutoffPassed)

	_, err = later.EditPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, created.ID,
		cutoffToday.AddDays(1), cutoffPickupTime(), "Termin verschoben", "")
	require.ErrorIs(t, err, parentService.ErrPickupChangeCutoffPassed, "moving today's request away is closed too")
}

// Removing an effective exception needs no staff decision, so it is the path
// that must not slip past the cutoff.
func TestDeleteCareExceptionHonoursSameDayCutoff(t *testing.T) {
	t.Parallel()

	d := newCutoffServiceDeps(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	for _, tc := range []struct {
		name        string
		now         func() time.Time
		date        timezone.Date
		settings    parentSettingsStub
		wantRefused bool
	}{
		{"heute 10:59", berlinClock(10, 59), cutoffToday, cutoffStub(true, "11:00"), false},
		{"heute genau 11:00", berlinClock(11, 0), cutoffToday, cutoffStub(true, "11:00"), false},
		{"heute 11:01", berlinClock(11, 1), cutoffToday, cutoffStub(true, "11:00"), true},
		{"morgen 11:01", berlinClock(11, 1), cutoffToday.AddDays(1), cutoffStub(true, "11:00"), false},
		{"ohne Frist heute 23:00", berlinClock(23, 0), cutoffToday, cutoffStub(true, ""), false},
		// The cutoff belongs to the one-day pickup change; with that switched
		// off the setting is hidden and does not apply.
		{"Abholänderung aus, heute 11:01", berlinClock(11, 1), cutoffToday, cutoffStub(false, "11:00"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chain := testpkg.CreateTestParentGuardianChain(t, d.db)
			seedGuardianPickupException(t, d, chain, tc.date)
			svc := d.service(t, tc.settings, tc.now)

			err := svc.DeleteCareException(ctx, chain.AccountID, chain.StudentID, tc.date)
			if tc.wantRefused {
				require.ErrorIs(t, err, parentService.ErrPickupChangeCutoffPassed)
				assert.True(t, guardianPickupExists(t, d, chain, tc.date), "the refused delete leaves the exception in place")
				return
			}
			require.NoError(t, err)
			assert.False(t, guardianPickupExists(t, d, chain, tc.date))
		})
	}
}

// A repeated or stale delete has no effective guardian exception to change,
// so it remains a successful no-op even after today's cutoff.
func TestDeleteCareExceptionWithoutGuardianPickupIsNoopAfterCutoff(t *testing.T) {
	t.Parallel()

	d := newCutoffServiceDeps(t)
	chain := testpkg.CreateTestParentGuardianChain(t, d.db)

	err := d.service(t, cutoffStub(true, "11:00"), berlinClock(11, 1)).DeleteCareException(
		testpkg.WithPackageTenantRuntime(context.Background()),
		chain.AccountID,
		chain.StudentID,
		cutoffToday,
	)
	require.NoError(t, err)
	assert.False(t, guardianPickupExists(t, d, chain, cutoffToday))
}

// The portal learns the cutoff and whether today is already closed from the
// features response, before anyone types.
func TestChildFeaturesReportPickupChangeCutoff(t *testing.T) {
	t.Parallel()

	d := newCutoffServiceDeps(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, d.db)

	for _, tc := range []struct {
		name       string
		settings   parentSettingsStub
		now        func() time.Time
		wantClock  string
		wantClosed bool
	}{
		{"vor der Frist", cutoffStub(true, "11:00"), berlinClock(10, 59), "11:00", false},
		{"genau zur Frist", cutoffStub(true, "11:00"), berlinClock(11, 0), "11:00", false},
		{"nach der Frist", cutoffStub(true, "11:00"), berlinClock(11, 1), "11:00", true},
		{"ohne Frist", cutoffStub(true, ""), berlinClock(23, 0), "", false},
		{"Abholänderung aus", cutoffStub(false, "11:00"), berlinClock(11, 1), "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags, err := d.service(t, tc.settings, tc.now).ChildFeatures(ctx, chain.AccountID, chain.StudentID)
			require.NoError(t, err)
			assert.Equal(t, tc.wantClock, flags.PickupChangeCutoffTime)
			assert.Equal(t, tc.wantClosed, flags.PickupChangeTodayClosed)
		})
	}
}

// The cutoff is a per-school setting resolved through the child's tenant: a
// school that sets one does not lock the families of another school.
func TestPickupChangeCutoffIsTenantScoped(t *testing.T) {
	t.Parallel()

	d := newCutoffServiceDeps(t)
	sf, err := services.NewFactoryForTests(d.repos, d.db, slog.Default())
	require.NoError(t, err)
	settings := sf.Settings
	runtimeCtx := testpkg.WithPackageTenantRuntime(context.Background())

	chainA := testpkg.CreateTestParentGuardianChain(t, d.db)
	require.NoError(t, settings.SetValue(testpkg.Ctx(t), configModels.KeyParentPickupChangeCutoffTime, "11:00", nil, nil))
	seedGuardianPickupException(t, d, chainA, cutoffToday)
	svc := d.service(t, settings, berlinClock(11, 1))

	flagsA, err := svc.ChildFeatures(runtimeCtx, chainA.AccountID, chainA.StudentID)
	require.NoError(t, err)
	assert.Equal(t, "11:00", flagsA.PickupChangeCutoffTime)
	assert.True(t, flagsA.PickupChangeTodayClosed)
	require.ErrorIs(t, svc.DeleteCareException(runtimeCtx, chainA.AccountID, chainA.StudentID, cutoffToday),
		parentService.ErrPickupChangeCutoffPassed)

	t.Run("andere Schule ohne Frist", func(t *testing.T) {
		testpkg.OwnTenant(t)
		chainB := testpkg.CreateTestParentGuardianChain(t, d.db)
		require.NotEqual(t, chainA.TenantID, chainB.TenantID)
		seedGuardianPickupException(t, d, chainB, cutoffToday)

		flagsB, err := svc.ChildFeatures(runtimeCtx, chainB.AccountID, chainB.StudentID)
		require.NoError(t, err)
		assert.Empty(t, flagsB.PickupChangeCutoffTime)
		assert.False(t, flagsB.PickupChangeTodayClosed)
		require.NoError(t, svc.DeleteCareException(runtimeCtx, chainB.AccountID, chainB.StudentID, cutoffToday))
		assert.False(t, guardianPickupExists(t, d, chainB, cutoffToday))
	})
}
