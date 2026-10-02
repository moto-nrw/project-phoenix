package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	pwaSvc "github.com/moto-nrw/project-phoenix/modules/delivery/application/pwa"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pwaUsageDeletes records the retention sweeps the Delivery usage service
// asks its repository for. The repository's SQL is covered in
// database/repositories/pwausage.
type pwaUsageDeletes struct {
	mu      sync.Mutex
	tenants []int64
	cutoffs []time.Time
}

func (r *pwaUsageDeletes) RecordSeen(context.Context, int64, int64, string) error { return nil }

func (r *pwaUsageDeletes) DeleteLastSeenBefore(_ context.Context, tenantID int64, cutoff time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenants = append(r.tenants, tenantID)
	r.cutoffs = append(r.cutoffs, cutoff)
	return 1, nil
}

// TestPWAUsageCleanup_SweepsStaleRows drives the scheduler hook through the
// real Delivery usage service: inside the school's transaction it sweeps the
// standalone-usage rows past the retention window (#2189).
func TestPWAUsageCleanup_SweepsStaleRows(t *testing.T) {
	t.Parallel()

	scheduleNow := time.Now()
	if scheduleNow.Second() >= 58 {
		t.Skip("skipping to avoid minute-boundary race on timeMatchesNow")
	}

	deletes := &pwaUsageDeletes{}
	cleanup := pwaSvc.NewUsageService(
		nil,
		deletes,
		nil,
		nil, // Cleanup does not resolve guardian schools.
		&configtest.Mock{
			ResolveIntFn: func(_ context.Context, key string) (int, error) {
				if key != "gdpr.pwa_usage_retention_days" {
					return 0, fmt.Errorf("unexpected setting key %q", key)
				}
				return 90, nil
			},
		},
		slog.Default(),
	)
	s := unitScheduler(&Scheduler{
		pwaUsageCleanup: cleanup,
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

	deletes.mu.Lock()
	defer deletes.mu.Unlock()
	require.Equal(t, []int64{schedulerUnitTenantID}, deletes.tenants, "one sweep for the school in its transaction")
	assert.WithinDuration(t, time.Now().AddDate(0, 0, -90), deletes.cutoffs[0], time.Minute,
		"rows older than the 90-day retention fall to the sweep")
	assert.True(t, wasRunToday(&s.lastPWAUsageCleanup, schedulerUnitTenantID), "the committed sweep marks the day")
}
