package config

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	configRepository "github.com/moto-nrw/project-phoenix/database/repositories/config"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	_ "github.com/moto-nrw/project-phoenix/services/config/defaults"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func attendanceScopeSettings(t *testing.T) SettingsService {
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
		got, err := settings.ResolveString(ctx, key)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	read(editKey, "own")
	read(configModel.KeyOperationalOverviewScope, "all_staff")
	require.NoError(t, settings.SetValue(ctx, configModel.KeyOperationalOverviewScope, "own", nil, nil))
	require.ErrorIs(t, settings.SetValue(ctx, editKey, "all_staff", nil, nil), ErrInvalidValue)
	read(editKey, "own")

	// Expand visibility first, then attendance editing. Reverse that order
	// when restricting, including resets to the registry default.
	require.NoError(t, settings.ResetValue(ctx, configModel.KeyOperationalOverviewScope, nil, nil))
	require.NoError(t, settings.SetValue(ctx, editKey, "all_staff", nil, nil))
	require.ErrorIs(t, settings.SetValue(ctx, configModel.KeyOperationalOverviewScope, "own", nil, nil), ErrInvalidValue)
	read(configModel.KeyOperationalOverviewScope, "all_staff")
	require.NoError(t, settings.ResetValue(ctx, editKey, nil, nil))
	require.NoError(t, settings.SetValue(ctx, configModel.KeyOperationalOverviewScope, "own", nil, nil))
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
			batch, ok := settings.(BatchSettingsService)
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
					if err := settings.SetValue(txCtx, firstKey, firstValue, nil, nil); err != nil {
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
					txCtx = WithSettingsRequestCache(txCtx)
					snapshot, err := batch.ResolveMany(txCtx, []string{configModel.KeyOperationalOverviewScope, configModel.KeyAttendanceEditScope})
					if err != nil {
						return err
					}
					txCtx = WithSettingsSnapshot(txCtx, snapshot)
					close(secondReading)
					select {
					case <-firstWritten:
					case <-ctx.Done():
						return ctx.Err()
					}
					close(secondWriting)
					return settings.SetValue(txCtx, secondKey, secondValue, nil, nil)
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
			require.ErrorIs(t, <-secondResult, ErrInvalidValue)
			visibility, err := settings.ResolveString(ctx, configModel.KeyOperationalOverviewScope)
			require.NoError(t, err)
			editing, err := settings.ResolveString(ctx, configModel.KeyAttendanceEditScope)
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
		if err := settings.SetValue(txCtx, configModel.KeyAttendanceEditScope, "all_staff", nil, nil); err != nil {
			return err
		}
		return abort
	})
	require.ErrorIs(t, err, abort)
	got, err := settings.ResolveString(ctx, configModel.KeyAttendanceEditScope)
	require.NoError(t, err)
	require.Equal(t, "own", got)
	require.NoError(t, settings.SetValue(ctx, configModel.KeyOperationalOverviewScope, "own", nil, nil))
}

func TestAttendanceScopeKeepsTenantAndPermissionBoundaries(t *testing.T) {
	t.Parallel()
	settings := attendanceScopeSettings(t)
	ctx := testpkg.Ctx(t)
	key := configModel.KeyAttendanceEditScope
	require.ErrorIs(t, settings.SetValue(ctx, key, "all_staff", nil, []string{"config:read"}), ErrPermissionDenied)
	require.ErrorIs(t, settings.SetValue(ctx, key, "unknown", nil, []string{"config:update"}), ErrInvalidValue)
	require.NoError(t, settings.SetValue(ctx, key, "all_staff", nil, []string{"config:update"}))
	require.ErrorIs(t, settings.ResetValue(ctx, key, nil, []string{"config:read"}), ErrPermissionDenied)
	got, err := settings.ResolveString(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "all_staff", got)

	t.Run("another school retains the registry default", func(t *testing.T) {
		otherCtx := testpkg.OwnCtx(t)
		got, err := settings.ResolveString(otherCtx, key)
		require.NoError(t, err)
		require.Equal(t, "own", got)
		require.NoError(t, settings.SetValue(otherCtx, configModel.KeyOperationalOverviewScope, "own", nil, []string{"config:update"}))
	})
	got, err = settings.ResolveString(ctx, configModel.KeyOperationalOverviewScope)
	require.NoError(t, err)
	require.Equal(t, "all_staff", got)
}
