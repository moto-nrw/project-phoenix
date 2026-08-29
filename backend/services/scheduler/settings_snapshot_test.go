package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	configRepository "github.com/moto-nrw/project-phoenix/database/repositories/config"
	platformRepository "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type schedulerSettingQueryCounter struct {
	count atomic.Int32
}

func (c *schedulerSettingQueryCounter) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	query := strings.ToLower(event.Query)
	if strings.HasPrefix(strings.TrimSpace(query), "select") &&
		strings.Contains(query, "config.setting_values") {
		c.count.Add(1)
	}
	return ctx
}

func (*schedulerSettingQueryCounter) AfterQuery(context.Context, *bun.QueryEvent) {}

func TestSchedulerPollingSettingKeysAreRegisteredUniqueAndNonSecret(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, len(schedulerPollingSettingKeys))
	for _, key := range schedulerPollingSettingKeys {
		require.NotEmpty(t, key)
		_, duplicate := seen[key]
		assert.False(t, duplicate, "duplicate scheduler snapshot key %q", key)
		seen[key] = struct{}{}

		definition := configModel.GetDefinition(key)
		require.NotNil(t, definition, "scheduler snapshot key %q must be registered", key)
		assert.NotEqual(t, configModel.FieldPassword, definition.Type,
			"scheduler snapshots must not preload secret settings")
	}
}

func TestSchedulerPollingSettingKeysIncludeAppointmentReminderSettings(t *testing.T) {
	t.Parallel()

	assert.Contains(t, schedulerPollingSettingKeys, configModel.KeyCalendarAppointmentReminderEnabled)
	assert.Contains(t, schedulerPollingSettingKeys, configModel.KeyCalendarAppointmentReminderLeadHours)
}

func TestSchedulerPollingSettingKeysIncludeAutoEndSettings(t *testing.T) {
	t.Parallel()

	assert.Contains(t, schedulerPollingSettingKeys, configModel.KeyTimetableAutoEndEnabled)
	assert.Contains(t, schedulerPollingSettingKeys, configModel.KeyTimetableAutoEndGraceMinutes)
}

// Deliberately NOT parallel: the test installs a query hook on the SHARED
// package pool and asserts a query budget, so any test running beside it is
// counted too.
func TestLoadMinuteSnapshotUsesOneSettingsQuery(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	valueRepository := configRepository.NewSettingValueRepository(db)
	settings := configService.NewSettingsService(valueRepository, nil, nil, db, slog.Default())
	scheduler := unitScheduler(&Scheduler{
		db:         db,
		schoolRepo: platformRepository.NewSchoolRepository(db),
		settings:   settings,
		done:       make(chan struct{}),
		logger:     slog.Default()})

	counter := &schedulerSettingQueryCounter{}
	db.AddQueryHook(counter)

	snapshot, err := scheduler.loadMinuteSnapshot(context.Background())
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	assert.Contains(t, snapshot.tenantIDs, tenantA)
	assert.Contains(t, snapshot.tenantIDs, tenantB)
	assert.Equal(t, int32(1), counter.count.Load(),
		"one scheduler minute must issue one config.setting_values SELECT for all active tenants")
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
			err := tenant.WithinAdmin(ctx, func(context.Context) error {
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
