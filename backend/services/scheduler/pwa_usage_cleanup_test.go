package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	repoFactory "github.com/moto-nrw/project-phoenix/database/repositories"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	pwaSvc "github.com/moto-nrw/project-phoenix/modules/delivery/application/pwa"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPWAUsageCleanup_SweepsStaleRows drives the scheduler hook end to end:
// a stale standalone-usage row falls to the retention sweep while a fresh
// one survives (#2189).
func TestPWAUsageCleanup_SweepsStaleRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pwa-cleanup-%d@example.com", time.Now().UnixNano()))
	defer func() {
		_, _ = db.NewRaw(`DELETE FROM iot.pwa_standalone_usage WHERE account_id = ?`, account.ID).Exec(context.Background())
	}()

	insertUsage := func(portal string, lastSeen time.Time) {
		_, err := db.ExecContext(context.Background(),
			`INSERT INTO iot.pwa_standalone_usage (tenant_id, account_id, portal, first_seen_at, last_seen_at)
			 VALUES (?, ?, ?, ?, ?)`,
			testpkg.Tenant(t), account.ID, portal, lastSeen, lastSeen)
		require.NoError(t, err)
	}
	insertUsage("staff", time.Now().AddDate(0, 0, -200))
	insertUsage("parent", time.Now())

	scheduleNow := time.Now()
	if scheduleNow.Second() >= 58 {
		t.Skip("skipping to avoid minute-boundary race on timeMatchesNow")
	}

	repos := repoFactory.NewFactory(db, repoFactory.NewUnobservedTimetableDependencies(db))
	cleanup := pwaSvc.NewUsageService(
		db,
		repos.PWAStandaloneUsage,
		repos.OperatorSummaries,
		repos.AccountTenant,
		&configtest.Mock{
			ResolveIntFn: func(_ context.Context, key string) (int, error) {
				if key != configModel.KeyGDPRPWAUsageRetentionDays {
					return 0, fmt.Errorf("unexpected setting key %q", key)
				}
				return 90, nil
			},
		},
		slog.Default(),
	)
	s := unitScheduler(&Scheduler{
		db:              db,
		schoolRepo:      platformRepo.NewSchoolRepository(db),
		pwaUsageCleanup: cleanup,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyDataCleanupEnabled: true,
			},
			stringValues: map[string]string{
				configModel.KeyDataCleanupTime: scheduleNow.Format("15:04"),
			},
			intValues: map[string]int{
				configModel.KeyDataCleanupTimeoutMinutes: 30,
			},
		},
		logger: slog.Default()})

	s.checkAndRunPWAUsageCleanup(context.Background(), &ScheduledTask{Name: "pwa-usage-cleanup"})

	var portals []string
	require.NoError(t, db.NewSelect().
		ColumnExpr("portal").
		TableExpr("iot.pwa_standalone_usage").
		Where("account_id = ?", account.ID).
		Scan(context.Background(), &portals))
	assert.Equal(t, []string{"parent"}, portals, "stale staff row must be swept, fresh parent row must survive")
}
