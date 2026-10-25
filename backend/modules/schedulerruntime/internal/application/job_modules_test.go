// Worker jobs follow the booked modules (#4895): a job of an additional
// module skips each school that has not booked it, Basis and retention jobs
// run for every school.
package application

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// moduleSwitches answers the module switches per school. The snapshot path
// binds the school ID as its snapshot; the other paths read the school from
// the tenant unit of work.
type moduleSwitches struct {
	stubSettingsResolver
	off     map[int64][]string
	readErr error
}

func (m *moduleSwitches) ResolveSettingsSnapshots(_ context.Context, tenantIDs []int64, _ []string) (map[int64]ports.SettingsSnapshot, error) {
	snapshots := make(map[int64]ports.SettingsSnapshot, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		snapshots[tenantID] = tenantID
	}
	return snapshots, nil
}

func (m *moduleSwitches) BindSettingsSnapshot(ctx context.Context, snapshot ports.SettingsSnapshot) (context.Context, error) {
	return context.WithValue(ctx, boundSnapshotKey{}, snapshot), nil
}

func (m *moduleSwitches) ResolveBool(ctx context.Context, key string) (bool, error) {
	if !strings.HasPrefix(key, "module.") {
		return m.stubSettingsResolver.ResolveBool(ctx, key)
	}
	if m.readErr != nil {
		return false, m.readErr
	}
	tenantID, _ := ctx.Value(boundSnapshotKey{}).(int64)
	if tenantID == 0 {
		tenantID = testpkg.TenantIDFromContext(ctx)
	}
	return !slices.Contains(m.off[tenantID], key), nil
}

// bookedModules is a settings fixture for module jobs whose test is not
// about the booking: every module switch reads as booked, the registry
// default (#4895).
func bookedModules() ports.SettingsResolver {
	return &moduleSwitches{}
}

func withWorkerJob(id ports.JobID) context.Context {
	return context.WithValue(context.Background(), workerJobIDKey{}, id)
}

func moduleScheduler(settings ports.SettingsResolver, tenantIDs ...int64) *Scheduler {
	return unitScheduler(&unitSchedulerDependencies{
		settings: settings,
		done:     make(chan struct{}),
		logger:   slog.Default(),
		minuteSnapshotLoader: func(context.Context) (*schedulerMinuteSnapshot, error) {
			snapshots := make(map[int64]ports.SettingsSnapshot, len(tenantIDs))
			for _, tenantID := range tenantIDs {
				snapshots[tenantID] = tenantID
			}
			return &schedulerMinuteSnapshot{tenantIDs: tenantIDs, settings: snapshots}, nil
		}})
}

func TestEveryWorkerJobHasAScope(t *testing.T) {
	t.Parallel()

	for _, id := range requiredWorkerJobIDs() {
		_, ok := workerJobScopes[id]
		assert.Truef(t, ok, "worker job %q belongs to no module, Basis or retention: add it to workerJobScopes", id)
	}
	assert.Len(t, workerJobScopes, len(requiredWorkerJobIDs()), "workerJobScopes names a job the worker does not run")
	for id, scope := range workerJobScopes {
		if scope.retention {
			assert.Emptyf(t, scope.moduleSettingKey, "retention job %q runs regardless of the booked modules", id)
		}
		if scope.moduleSettingKey != "" {
			assert.Containsf(t, schedulerPollingSettingKeys, scope.moduleSettingKey,
				"the minute snapshot must carry the switch of %q", id)
		}
	}
}

func TestModuleJobsNameTheModulesOfTheIssue(t *testing.T) {
	t.Parallel()

	assert.Equal(t, settingModuleZeiterfassung, workerJobScopes["auto-checkout"].moduleSettingKey)
	assert.Equal(t, settingModuleZeiterfassung, workerJobScopes["break-auto-end"].moduleSettingKey)
	assert.Equal(t, settingModuleElternApp, workerJobScopes["appointment-reminders"].moduleSettingKey)
	assert.Equal(t, settingModuleElternApp, workerJobScopes["announcement-reminders"].moduleSettingKey)
	assert.Equal(t, settingModuleOnlineAnmeldung, workerJobScopes["rollover-deadline"].moduleSettingKey)
	for _, id := range []ports.JobID{"visit-cleanup", "time-tracking-cleanup", "staff-message-cleanup", "file-store-cleanup"} {
		assert.Truef(t, workerJobScopes[id].retention, "%q keeps the retention windows running (#4812)", id)
	}
}

func TestModuleJobSkipsSchoolWithoutTheModule(t *testing.T) {
	t.Parallel()

	settings := &moduleSwitches{off: map[int64][]string{42: {settingModuleElternApp}}}
	scheduler := moduleScheduler(settings, 41, 42)

	var ran []int64
	completed := scheduler.forEachTenantSettings(withWorkerJob("announcement-reminders"), "announcement-reminders",
		func(_ context.Context, tenantID int64) error {
			ran = append(ran, tenantID)
			return nil
		})

	assert.Equal(t, []int64{41}, ran, "only the school with Eltern-App gets its parent reminders")
	assert.ElementsMatch(t, []int64{41, 42}, completed, "a skipped school is done for this tick, not failed")
}

func TestModuleJobRunsForEverySchoolWithTheModule(t *testing.T) {
	t.Parallel()

	settings := &moduleSwitches{off: map[int64][]string{41: {settingModuleZeiterfassung}}}
	scheduler := moduleScheduler(settings, 41, 42)

	var ran []int64
	scheduler.forEachTenantSettings(withWorkerJob("appointment-reminders"), "appointment-reminders",
		func(_ context.Context, tenantID int64) error {
			ran = append(ran, tenantID)
			return nil
		})

	assert.Equal(t, []int64{41, 42}, ran, "a switch of another module does not stop the job")
}

func TestTwoSchoolsDoNotShareTheirBooking(t *testing.T) {
	t.Parallel()

	settings := &moduleSwitches{off: map[int64][]string{
		41: {settingModuleZeiterfassung},
		42: {settingModuleElternApp},
	}}
	scheduler := moduleScheduler(settings, 41, 42)

	ranFor := func(job ports.JobID) []int64 {
		var ran []int64
		scheduler.forEachTenantSettings(withWorkerJob(job), string(job), func(_ context.Context, tenantID int64) error {
			ran = append(ran, tenantID)
			return nil
		})
		return ran
	}

	assert.Equal(t, []int64{42}, ranFor("auto-checkout"))
	assert.Equal(t, []int64{41}, ranFor("appointment-reminders"))
}

func TestForEachTenantModuleJobSkipsSchoolWithoutTheModule(t *testing.T) {
	t.Parallel()

	settings := &moduleSwitches{off: map[int64][]string{42: {settingModuleZeiterfassung}}}
	scheduler := moduleScheduler(settings, 41, 42)

	var ran []int64
	err := scheduler.forEachTenant(withWorkerJob("break-auto-end"), "break-auto-end", func(ctx context.Context) error {
		ran = append(ran, testpkg.TenantIDFromContext(ctx))
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []int64{41}, ran)
}

func TestRetentionAndBasisJobsIgnoreTheBooking(t *testing.T) {
	t.Parallel()

	allOff := []string{settingModuleElternApp, settingModuleOnlineAnmeldung, settingModuleZeiterfassung}
	settings := &moduleSwitches{off: map[int64][]string{41: allOff}}
	scheduler := moduleScheduler(settings, 41)

	for _, job := range []ports.JobID{"time-tracking-cleanup", "staff-message-cleanup", "session-end", "reminder-notifications"} {
		var ran []int64
		scheduler.forEachTenantSettings(withWorkerJob(job), string(job), func(_ context.Context, tenantID int64) error {
			ran = append(ran, tenantID)
			return nil
		})
		assert.Equalf(t, []int64{41}, ran, "%q runs without any additional module", job)
	}
}

func TestUnreadableModuleSwitchFailsTheSchool(t *testing.T) {
	t.Parallel()

	settings := &moduleSwitches{readErr: errors.New("settings unavailable")}
	scheduler := moduleScheduler(settings, 41)

	called := false
	completed := scheduler.forEachTenantSettings(withWorkerJob("rollover-deadline"), "rollover-deadline",
		func(context.Context, int64) error {
			called = true
			return nil
		})

	assert.False(t, called, "an unreadable switch never counts as booked")
	assert.Empty(t, completed, "the school is retried on the next tick")
}
