package config

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	configRepository "github.com/moto-nrw/project-phoenix/modules/settings/internal/adapters/configpostgres"
	_ "github.com/moto-nrw/project-phoenix/modules/settings/internal/defaults"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func attendanceScopeSettings(t *testing.T) configModel.SettingsService {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	return NewSettingsService(
		configRepository.NewSettingValueRepository(testpkg.ConfigRuntime(db)),
		configRepository.NewSettingAuditRepository(testpkg.ConfigRuntime(db)),
		nil, testpkg.SettingsRuntime(t, db), slog.Default(), configModel.DefaultRegistry(),
	)
}

func TestAttendanceScopeRequiresSchoolWideVisibility(t *testing.T) {
	t.Parallel()
	settings := attendanceScopeSettings(t)
	ctx := testpkg.Ctx(t)
	const editKey = configModel.KeyAttendanceEditScope
	read := func(key, want string) {
		t.Helper()
		got, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.ResolveString, key)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	read(editKey, "own")
	read(configModel.KeyOperationalOverviewScope, "all_staff")
	require.NoError(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, configModel.KeyOperationalOverviewScope, "own", nil, nil))
	require.ErrorIs(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, editKey, "all_staff", nil, nil), configModel.ErrInvalidValue)
	read(editKey, "own")

	// Expand visibility first, then attendance editing. Reverse that order
	// when restricting, including resets to the registry default.
	require.NoError(t, testpkg.TenantCommand3(t, ctx, testpkg.SetupTestDB(t), settings.ResetValue, configModel.KeyOperationalOverviewScope, nil, nil))
	require.NoError(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, editKey, "all_staff", nil, nil))
	require.ErrorIs(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, configModel.KeyOperationalOverviewScope, "own", nil, nil), configModel.ErrInvalidValue)
	read(configModel.KeyOperationalOverviewScope, "all_staff")
	require.NoError(t, testpkg.TenantCommand3(t, ctx, testpkg.SetupTestDB(t), settings.ResetValue, editKey, nil, nil))
	require.NoError(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, configModel.KeyOperationalOverviewScope, "own", nil, nil))
	read(editKey, "own")
	read(configModel.KeyOperationalOverviewScope, "own")
}

func TestAttendanceScopeConcurrentWritersRejectStalePair(t *testing.T) {
	t.Parallel()
	for _, firstKey := range []string{configModel.KeyAttendanceEditScope, configModel.KeyOperationalOverviewScope} {
		t.Run(firstKey, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(testpkg.OwnCtx(t), 10*time.Second)
			defer cancel()
			settings := attendanceScopeSettings(t)
			batch, ok := settings.(configModel.BatchSettingsService)
			require.True(t, ok)
			runtime := testpkg.SettingsRuntime(t, testpkg.SetupTestDB(t))
			firstValue, secondKey, secondValue := "all_staff", configModel.KeyOperationalOverviewScope, "own"
			if firstKey == configModel.KeyOperationalOverviewScope {
				firstValue, secondKey, secondValue = "own", configModel.KeyAttendanceEditScope, "all_staff"
			}
			firstWritten := make(chan struct{})
			secondReading := make(chan struct{})
			secondWriting := make(chan struct{})
			releaseFirst := make(chan struct{})
			firstResult, secondResult := make(chan error, 1), make(chan error, 1)
			go func() {
				firstResult <- runtime.WithinTenant(ctx, testpkg.Tenant(t), func(txCtx context.Context) error {
					select {
					case <-secondReading:
					case <-ctx.Done():
						return ctx.Err()
					}
					if err := testpkg.TenantCommand4(t, txCtx, testpkg.SetupTestDB(t), settings.SetValue, firstKey, any(firstValue), nil, nil); err != nil {
						return err
					}
					close(firstWritten)
					select {
					case <-releaseFirst:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
			}()
			go func() {
				secondResult <- runtime.WithinTenant(ctx, testpkg.Tenant(t), func(txCtx context.Context) error {
					txCtx = configModel.WithSettingsRequestCache(txCtx)
					snapshot, err := batch.ResolveMany(txCtx, []string{configModel.KeyOperationalOverviewScope, configModel.KeyAttendanceEditScope})
					if err != nil {
						return err
					}
					txCtx = configModel.WithSettingsSnapshot(txCtx, snapshot)
					close(secondReading)
					select {
					case <-firstWritten:
					case <-ctx.Done():
						return ctx.Err()
					}
					close(secondWriting)
					return testpkg.TenantCommand4(t, txCtx, testpkg.SetupTestDB(t), settings.SetValue, secondKey, any(secondValue), nil, nil)
				})
			}()
			select {
			case <-secondWriting:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case err := <-secondResult:
				close(releaseFirst)
				t.Fatalf("second writer finished before first transaction committed: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			close(releaseFirst)
			require.NoError(t, <-firstResult)
			require.ErrorIs(t, <-secondResult, configModel.ErrInvalidValue)
			visibility, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.ResolveString, configModel.KeyOperationalOverviewScope)
			require.NoError(t, err)
			editing, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.ResolveString, configModel.KeyAttendanceEditScope)
			require.NoError(t, err)
			require.False(t, editing == "all_staff" && visibility != "all_staff")
		})
	}
}

func TestAttendanceScopeRollbackPreservesPair(t *testing.T) {
	t.Parallel()
	settings := attendanceScopeSettings(t)
	ctx := testpkg.Ctx(t)
	runtime := testpkg.SettingsRuntime(t, testpkg.SetupTestDB(t))
	abort := errors.New("abort setting transaction")
	err := runtime.WithinTenant(ctx, testpkg.Tenant(t), func(txCtx context.Context) error {
		if err := testpkg.TenantCommand4(t, txCtx, testpkg.SetupTestDB(t), settings.SetValue, configModel.KeyAttendanceEditScope, "all_staff", nil, nil); err != nil {
			return err
		}
		return abort
	})
	require.ErrorIs(t, err, abort)
	got, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.ResolveString, configModel.KeyAttendanceEditScope)
	require.NoError(t, err)
	require.Equal(t, "own", got)
	require.NoError(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, configModel.KeyOperationalOverviewScope, "own", nil, nil))
}

func TestAttendanceScopeKeepsTenantAndPermissionBoundaries(t *testing.T) {
	t.Parallel()
	settings := attendanceScopeSettings(t)
	ctx := testpkg.Ctx(t)
	key := configModel.KeyAttendanceEditScope
	require.ErrorIs(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, key, "all_staff", nil, []string{"config:read"}), configModel.ErrPermissionDenied)
	require.ErrorIs(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, key, "unknown", nil, []string{"config:update"}), configModel.ErrInvalidValue)
	require.NoError(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, key, "all_staff", nil, []string{"config:update"}))
	require.ErrorIs(t, testpkg.TenantCommand3(t, ctx, testpkg.SetupTestDB(t), settings.ResetValue, key, nil, []string{"config:read"}), configModel.ErrPermissionDenied)
	got, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.ResolveString, key)
	require.NoError(t, err)
	require.Equal(t, "all_staff", got)

	t.Run("another school retains the registry default", func(t *testing.T) {
		otherCtx := testpkg.OwnCtx(t)
		got, err := testpkg.TenantQuery1(t, otherCtx, testpkg.SetupTestDB(t), settings.ResolveString, key)
		require.NoError(t, err)
		require.Equal(t, "own", got)
		require.NoError(t, testpkg.TenantCommand4(t, otherCtx, testpkg.SetupTestDB(t), settings.SetValue, configModel.KeyOperationalOverviewScope, "own", nil, []string{"config:update"}))
	})
	got, err = testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.ResolveString, configModel.KeyOperationalOverviewScope)
	require.NoError(t, err)
	require.Equal(t, "all_staff", got)
}

func TestBlockStartScopeRequiresSchoolWideVisibility(t *testing.T) {
	t.Parallel()
	settings := attendanceScopeSettings(t)
	ctx := testpkg.Ctx(t)
	const startKey, visibilityKey = configModel.KeyBlockStartScope, configModel.KeyOperationalOverviewScope
	read := func(key, want string) {
		t.Helper()
		got, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.ResolveString, key)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	read(startKey, "own")
	require.NoError(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, visibilityKey, "own", nil, nil))
	err := testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, startKey, "all_staff", nil, nil)
	require.ErrorIs(t, err, configModel.ErrInvalidValue)
	require.ErrorContains(t, err, "Das ganze Team darf nur starten")
	read(startKey, "own")

	// Expand visibility first, then starting. Restricting runs the other way,
	// including resets to the registry default.
	require.NoError(t, testpkg.TenantCommand3(t, ctx, testpkg.SetupTestDB(t), settings.ResetValue, visibilityKey, nil, nil))
	require.NoError(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, startKey, "all_staff", nil, nil))
	require.ErrorIs(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, visibilityKey, "own", nil, nil), configModel.ErrInvalidValue)
	require.ErrorIs(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, visibilityKey, configModel.OverviewScopeAdmins, nil, nil), configModel.ErrInvalidValue)
	read(visibilityKey, "all_staff")

	// Each widened action blocks a restriction until it is back to own.
	require.NoError(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, configModel.KeyAttendanceEditScope, "all_staff", nil, nil))
	require.NoError(t, testpkg.TenantCommand3(t, ctx, testpkg.SetupTestDB(t), settings.ResetValue, startKey, nil, nil))
	require.ErrorIs(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, visibilityKey, "own", nil, nil), configModel.ErrInvalidValue)
	require.NoError(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, configModel.KeyAttendanceEditScope, "own", nil, nil))
	require.NoError(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, visibilityKey, "own", nil, nil))
	read(startKey, "own")
	read(visibilityKey, "own")
}

func TestBlockCompleteScopeRequiresSchoolWideVisibility(t *testing.T) {
	t.Parallel()
	settings := attendanceScopeSettings(t)
	ctx := testpkg.Ctx(t)
	const endKey, visibilityKey = configModel.KeyBlockCompleteScope, configModel.KeyOperationalOverviewScope
	got, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.ResolveString, endKey)
	require.NoError(t, err)
	require.Equal(t, "own", got)
	require.NoError(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, visibilityKey, "own", nil, nil))
	err = testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, endKey, "all_staff", nil, nil)
	require.ErrorIs(t, err, configModel.ErrInvalidValue)
	require.ErrorContains(t, err, "Das ganze Team darf nur beenden")

	require.NoError(t, testpkg.TenantCommand3(t, ctx, testpkg.SetupTestDB(t), settings.ResetValue, visibilityKey, nil, nil))
	require.NoError(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, endKey, "all_staff", nil, nil))
	require.ErrorIs(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, visibilityKey, "own", nil, nil), configModel.ErrInvalidValue)
	require.NoError(t, testpkg.TenantCommand3(t, ctx, testpkg.SetupTestDB(t), settings.ResetValue, endKey, nil, nil))
	require.NoError(t, testpkg.TenantCommand4(t, ctx, testpkg.SetupTestDB(t), settings.SetValue, visibilityKey, "own", nil, nil))
}

func TestBlockStartScopeConcurrentWritersRejectStaleValues(t *testing.T) {
	t.Parallel()
	for _, firstKey := range []string{configModel.KeyBlockStartScope, configModel.KeyOperationalOverviewScope} {
		t.Run(firstKey, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(testpkg.OwnCtx(t), 10*time.Second)
			defer cancel()
			settings := attendanceScopeSettings(t)
			batch, ok := settings.(configModel.BatchSettingsService)
			require.True(t, ok)
			runtime := testpkg.SettingsRuntime(t, testpkg.SetupTestDB(t))
			firstValue, secondKey, secondValue := "all_staff", configModel.KeyOperationalOverviewScope, "own"
			if firstKey == configModel.KeyOperationalOverviewScope {
				firstValue, secondKey, secondValue = "own", configModel.KeyBlockStartScope, "all_staff"
			}
			firstWritten, secondReading, releaseFirst := make(chan struct{}), make(chan struct{}), make(chan struct{})
			firstResult, secondResult := make(chan error, 1), make(chan error, 1)
			go func() {
				firstResult <- runtime.WithinTenant(ctx, testpkg.Tenant(t), func(txCtx context.Context) error {
					select {
					case <-secondReading:
					case <-ctx.Done():
						return ctx.Err()
					}
					if err := testpkg.TenantCommand4(t, txCtx, testpkg.SetupTestDB(t), settings.SetValue, firstKey, any(firstValue), nil, nil); err != nil {
						return err
					}
					close(firstWritten)
					select {
					case <-releaseFirst:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
			}()
			go func() {
				secondResult <- runtime.WithinTenant(ctx, testpkg.Tenant(t), func(txCtx context.Context) error {
					txCtx = configModel.WithSettingsRequestCache(txCtx)
					snapshot, err := batch.ResolveMany(txCtx, []string{configModel.KeyOperationalOverviewScope, configModel.KeyBlockStartScope})
					if err != nil {
						return err
					}
					txCtx = configModel.WithSettingsSnapshot(txCtx, snapshot)
					close(secondReading)
					select {
					case <-firstWritten:
					case <-ctx.Done():
						return ctx.Err()
					}
					return testpkg.TenantCommand4(t, txCtx, testpkg.SetupTestDB(t), settings.SetValue, secondKey, any(secondValue), nil, nil)
				})
			}()
			select {
			case <-firstWritten:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case err := <-secondResult:
				close(releaseFirst)
				t.Fatalf("second writer finished before first transaction committed: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			close(releaseFirst)
			require.NoError(t, <-firstResult)
			require.ErrorIs(t, <-secondResult, configModel.ErrInvalidValue)
			visibility, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.ResolveString, configModel.KeyOperationalOverviewScope)
			require.NoError(t, err)
			starting, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.ResolveString, configModel.KeyBlockStartScope)
			require.NoError(t, err)
			require.False(t, starting == "all_staff" && visibility != "all_staff")
		})
	}
}
