package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Registration and secrecy of these keys are pinned against the Settings
// Platform registry by api.TestSchedulerSettingKeysPassTheRegistryGuard; the
// Worker refuses to start on a key that fails the same guard.
func TestSchedulerPollingSettingKeysAreUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, len(schedulerPollingSettingKeys))
	for _, key := range schedulerPollingSettingKeys {
		require.NotEmpty(t, key)
		_, duplicate := seen[key]
		assert.False(t, duplicate, "duplicate scheduler snapshot key %q", key)
		seen[key] = struct{}{}
	}
}

func TestSchedulerPollingSettingKeysIncludeAppointmentReminderSettings(t *testing.T) {
	t.Parallel()

	assert.Contains(t, schedulerPollingSettingKeys, settingCalendarAppointmentReminderEnabled)
	assert.Contains(t, schedulerPollingSettingKeys, settingCalendarAppointmentReminderLeadHours)
}

func TestSchedulerPollingSettingKeysIncludeAutoEndSettings(t *testing.T) {
	t.Parallel()

	assert.Contains(t, schedulerPollingSettingKeys, settingTimetableAutoEndEnabled)
	assert.Contains(t, schedulerPollingSettingKeys, settingTimetableAutoEndGraceMinutes)
}

// batchSettings is a settings resolver that also offers the batch read.
// It records every batch call and binds its snapshots by tenant ID.
type batchSettings struct {
	stubSettingsResolver
	mu    sync.Mutex
	calls [][]int64
	keys  [][]string
}

type boundSnapshotKey struct{}

func (b *batchSettings) ResolveSettingsSnapshots(_ context.Context, tenantIDs []int64, keys []string) (map[int64]SettingsSnapshot, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, append([]int64(nil), tenantIDs...))
	b.keys = append(b.keys, append([]string(nil), keys...))
	snapshots := make(map[int64]SettingsSnapshot, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		snapshots[tenantID] = tenantID
	}
	return snapshots, nil
}

func (b *batchSettings) BindSettingsSnapshot(ctx context.Context, snapshot SettingsSnapshot) (context.Context, error) {
	if _, ok := snapshot.(string); !ok {
		return ctx, errors.New("snapshot was not resolved by this source")
	}
	return context.WithValue(ctx, boundSnapshotKey{}, snapshot), nil
}

// One scheduler minute lists the active schools and resolves their polling
// settings in one batch call. The Settings Platform serves that call with one
// config.setting_values SELECT (modules/settings/compose).
func TestLoadMinuteSnapshotResolvesEverySchoolInOneBatch(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	settings := &batchSettings{}
	scheduler := unitScheduler(&Scheduler{
		schoolRepo:    dbTenantDirectory{db: db},
		settings:      settings,
		tenantRuntime: dbTenantRuntime(t, db),
		done:          make(chan struct{}),
		logger:        slog.Default()})

	snapshot, err := scheduler.loadMinuteSnapshot(context.Background())
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	assert.Contains(t, snapshot.tenantIDs, tenantA)
	assert.Contains(t, snapshot.tenantIDs, tenantB)
	require.Len(t, settings.calls, 1, "one batch read for all active schools")
	assert.Equal(t, snapshot.tenantIDs, settings.calls[0])
	assert.Equal(t, schedulerPollingSettingKeys, settings.keys[0])
	assert.Equal(t, SettingsSnapshot(tenantA), snapshot.settings[tenantA])
}

func TestForEachTenantSettingsBindsEachSchoolsSnapshot(t *testing.T) {
	t.Parallel()

	settings := &batchSettings{}
	scheduler := unitScheduler(&Scheduler{
		settings: settings,
		done:     make(chan struct{}),
		logger:   slog.Default(),
		minuteSnapshotLoader: func(context.Context) (*schedulerMinuteSnapshot, error) {
			return &schedulerMinuteSnapshot{
				tenantIDs: []int64{31, 32},
				settings:  map[int64]SettingsSnapshot{31: "snapshot-31", 32: "snapshot-32"},
			}, nil
		}})

	bound := map[int64]any{}
	scheduler.forEachTenantSettings(context.Background(), "bind-settings", func(ctx context.Context, tenantID int64) error {
		bound[tenantID] = ctx.Value(boundSnapshotKey{})
		return nil
	})

	assert.Equal(t, map[int64]any{31: "snapshot-31", 32: "snapshot-32"}, bound)
}

func TestForEachTenantSettingsFailsASchoolWhoseSnapshotCannotBeBound(t *testing.T) {
	t.Parallel()

	settings := &batchSettings{}
	scheduler := unitScheduler(&Scheduler{
		settings: settings,
		done:     make(chan struct{}),
		logger:   slog.Default(),
		minuteSnapshotLoader: func(context.Context) (*schedulerMinuteSnapshot, error) {
			return &schedulerMinuteSnapshot{
				tenantIDs: []int64{33, 34},
				settings:  map[int64]SettingsSnapshot{33: "snapshot-33", 34: 34},
			}, nil
		}})

	var called []int64
	completed := scheduler.forEachTenantSettings(context.Background(), "bind-settings", func(_ context.Context, tenantID int64) error {
		called = append(called, tenantID)
		return nil
	})

	assert.Equal(t, []int64{33}, called, "a school is never served without its snapshot")
	assert.Equal(t, []int64{33}, completed)
}

func TestGetMinuteSnapshotCoalescesConcurrentLoads(t *testing.T) {
	t.Parallel()

	fixedNow := time.Date(2026, time.July, 30, 12, 15, 10, 0, time.UTC)
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	scheduler := unitScheduler(&Scheduler{
		done:              make(chan struct{}),
		minuteSnapshotNow: func() time.Time { return fixedNow },
		minuteSnapshotLoader: func(context.Context) (*schedulerMinuteSnapshot, error) {
			if calls.Add(1) == 1 {
				close(started)
			}
			<-release
			return &schedulerMinuteSnapshot{tenantIDs: []int64{1, 2}}, nil
		}})

	const callers = 32
	results := make(chan *schedulerMinuteSnapshot, callers)
	errs := make(chan error, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	start := make(chan struct{})
	for range callers {
		go func() {
			ready.Done()
			<-start
			result, err := scheduler.getMinuteSnapshot(context.Background())
			results <- result
			errs <- err
		}()
	}
	ready.Wait()
	close(start)
	<-started
	close(release)

	for range callers {
		require.NoError(t, <-errs)
		require.Equal(t, []int64{1, 2}, (<-results).tenantIDs)
	}
	assert.Equal(t, int32(1), calls.Load(), "all jobs in one minute must share one loader call")
}

func TestGetMinuteSnapshotRetriesOnlyAfterMinuteChanges(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, time.July, 30, 12, 15, 10, 0, time.UTC)
	loadErr := errors.New("settings unavailable")
	var calls atomic.Int32
	scheduler := unitScheduler(&Scheduler{
		done:              make(chan struct{}),
		minuteSnapshotNow: func() time.Time { return current },
		minuteSnapshotLoader: func(context.Context) (*schedulerMinuteSnapshot, error) {
			calls.Add(1)
			return nil, loadErr
		}})

	_, err := scheduler.getMinuteSnapshot(context.Background())
	require.ErrorIs(t, err, loadErr)
	_, err = scheduler.getMinuteSnapshot(context.Background())
	require.ErrorIs(t, err, loadErr)
	assert.Equal(t, int32(1), calls.Load(), "an outage must not trigger a retry storm within the same minute")

	current = current.Add(time.Minute)
	_, err = scheduler.getMinuteSnapshot(context.Background())
	require.ErrorIs(t, err, loadErr)
	assert.Equal(t, int32(2), calls.Load(), "the next minute must retry so settings recover within the freshness bound")
}

func TestGetMinuteSnapshotBindsTenantRuntimeBeforeLoading(t *testing.T) {
	t.Parallel()
	var adminCalled bool
	scheduler := unitScheduler(&Scheduler{
		done: make(chan struct{}),
		minuteSnapshotLoader: func(ctx context.Context) (*schedulerMinuteSnapshot, error) {
			err := testpkg.WithinAdminTransaction(ctx, func(context.Context) error {
				adminCalled = true
				return nil
			})
			return &schedulerMinuteSnapshot{}, err
		},
	})

	_, err := scheduler.getMinuteSnapshot(context.Background())

	require.NoError(t, err)
	assert.True(t, adminCalled)
}

func TestGetMinuteSnapshotSlowPriorMinuteCannotOverwriteCurrent(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, time.July, 30, 12, 15, 59, 0, time.UTC)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var calls atomic.Int32
	scheduler := unitScheduler(&Scheduler{
		done:              make(chan struct{}),
		minuteSnapshotNow: func() time.Time { return current },
		minuteSnapshotLoader: func(context.Context) (*schedulerMinuteSnapshot, error) {
			switch calls.Add(1) {
			case 1:
				close(firstStarted)
				<-releaseFirst
				return &schedulerMinuteSnapshot{tenantIDs: []int64{1}}, nil
			case 2:
				return &schedulerMinuteSnapshot{tenantIDs: []int64{2}}, nil
			default:
				return nil, errors.New("unexpected extra load")
			}
		}})

	firstResult := make(chan *schedulerMinuteSnapshot, 1)
	go func() {
		result, _ := scheduler.getMinuteSnapshot(context.Background())
		firstResult <- result
	}()
	<-firstStarted

	current = current.Add(time.Second)
	second, err := scheduler.getMinuteSnapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, []int64{2}, second.tenantIDs)

	close(releaseFirst)
	require.Equal(t, []int64{1}, (<-firstResult).tenantIDs)

	cached, err := scheduler.getMinuteSnapshot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int64{2}, cached.tenantIDs)
	assert.Equal(t, int32(2), calls.Load())
}
