package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pwaUsageSweeps records the schools the Worker asks the Delivery usage
// sweep for. The sweep's retention cutoff is covered in
// modules/delivery/application/pwa.
type pwaUsageSweeps struct {
	mu      sync.Mutex
	tenants []int64
}

func (r *pwaUsageSweeps) cleanup(ctx context.Context) (ports.PWAUsageCleanupResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenants = append(r.tenants, testpkg.TenantIDFromContext(ctx))
	return ports.PWAUsageCleanupResult{RowsDeleted: 1, RetentionDays: 90}, nil
}

// TestPWAUsageCleanup_SweepsStaleRows drives the scheduler hook: inside the
// school's transaction it sweeps the standalone-usage rows past the retention
// window (#2189).
func TestPWAUsageCleanup_SweepsStaleRows(t *testing.T) {
	t.Parallel()

	scheduleNow := berlin(14, 2, 0)

	sweeps := &pwaUsageSweeps{}
	cleanup := sweeps.cleanup
	s := unitScheduler(&unitSchedulerDependencies{
		pwaUsageCleanup: cleanup,
		cleanupClock:    func() time.Time { return scheduleNow },
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				settingDataCleanupEnabled: true,
			},
			stringValues: map[string]string{
				settingDataCleanupTime: scheduleNow.Format("15:04"),
			},
			intValues: map[string]int{
				settingDataCleanupTimeoutMinutes: 30,
			},
		},
		logger: slog.Default()})

	s.checkAndRunPWAUsageCleanup(context.Background(), &ScheduledTask{Name: "pwa-usage-cleanup"})

	sweeps.mu.Lock()
	defer sweeps.mu.Unlock()
	require.Equal(t, []int64{schedulerUnitTenantID}, sweeps.tenants, "one sweep for the school in its transaction")
	assert.True(t, wasRunAt(&s.lastPWAUsageCleanup, schedulerUnitTenantID, scheduleNow), "the committed sweep marks the day")
}
