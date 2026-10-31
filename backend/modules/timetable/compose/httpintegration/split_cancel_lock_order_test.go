package httpintegration_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Pause the real split after Care Plan's predecessor resync has deleted a
// purely planned participant. Start the real cancellation and wait for
// PostgreSQL to report it blocked before the split continues.
// Without the fix this closes the participant -> day -> participant cycle.
func TestFutureSplitAndBulkCancellationDoNotDeadlock(t *testing.T) {
	t.Parallel()
	for _, offset := range []int{0, -7, 7} {
		t.Run(fmt.Sprintf("participant_%+d_days_from_cut", offset), func(t *testing.T) {
			testFutureSplitAndBulkCancellation(t, offset)
		})
	}
}

func testFutureSplitAndBulkCancellation(t *testing.T, offset int) {
	t.Helper()
	testpkg.SetupIsolatedTestDB(t)
	f := setupInactiveSourceTemplate(t)
	cut := futureMonday(2)
	day := cut.AddDays(offset)
	from, to := cut, day
	if day.Before(cut) {
		from, to = day, cut
	}
	require.NoError(t, testpkg.WithTenantTx(t, f.s.ctx, f.s.db, f.s.tenantID, func(ctx context.Context, _ bun.Tx) error {
		_, err := f.s.materialization.MaterializeForTenant(ctx, day, day, timetable.MaterializationSourceManual)
		return err
	}))
	instances := testpkg.ActivityInstancesWhere(t, f.s.ctx, f.s.db, `"activity_instance".activity_group_id = ? AND "activity_instance".date = ?`, f.update.TemplateID, day)
	require.Len(t, instances, 1)
	oldID := instances[0].ID
	participant := f.participant(t, oldID, f.s.students[0])
	f.deactivate(t, f.a)
	ctx, cancel := context.WithCancel(f.s.ctx)
	resynced := make(chan context.Context, 1)
	releaseSplit := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(releaseSplit) }) }
	var running sync.WaitGroup
	t.Cleanup(func() {
		cancel()
		release()
		running.Wait()
	})
	module, err := api.NewTimetableHTTPTestModule(testpkg.NewDatabase(t, f.s.db), api.TimetableHTTPTestOptions{
		Instances:                  f.s.instance,
		Materialization:            f.s.materialization,
		Clock:                      func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) },
		ValidateCareOfferingSeries: f.s.careOffering.ValidateTemplateSeries,
		ValidateOfferingSource:     f.s.careOffering.ValidateTemplateOfferingSource,
		ResyncOfferingRoster: func(ctx context.Context, in timetable.OfferingRosterResyncInput) error {
			err := f.s.careOffering.ResyncTemplateOfferingRoster(ctx, careplan.OfferingRosterResync{
				TemplateID: in.TemplateID, OfferingIDs: in.OfferingIDs, GradeLevels: in.GradeLevels,
				SchoolClasses: in.SchoolClasses, CalendarPeriodID: in.CalendarPeriodID, EffectiveFrom: in.EffectiveFrom,
				ScopeRequestChildIDs: in.ScopeRequestChildIDs, TolerateDriftedSources: in.TolerateDriftedSources,
				StoredOfferingIDs: in.StoredOfferingIDs,
			})
			if err != nil || in.TemplateID != f.update.TemplateID {
				return err
			}
			select {
			case resynced <- ctx:
			case <-ctx.Done():
				return ctx.Err()
			}
			select {
			case <-releaseSplit:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	require.NoError(t, err)
	in := baseSplitInput(f.s, cut, "Split with concurrent cancellation")
	in.TemplateID = f.update.TemplateID
	in.TargetGroupType = f.update.Fields.TargetGroupType
	in.MaterializeFrom, in.MaterializeTo = &cut, &cut
	splitDone := make(chan error, 1)
	running.Go(func() {
		splitDone <- testpkg.WithTenantTx(t, ctx, f.s.db, f.s.tenantID, func(splitCtx context.Context, _ bun.Tx) error {
			_, err := module.Templates.SplitTemplate(splitCtx, in)
			return err
		})
	})
	var splitCtx context.Context
	select {
	case splitCtx = <-resynced:
	case err := <-splitDone:
		t.Fatalf("split did not reach its resync barrier: %v", err)
	case <-ctx.Done():
		t.Fatal("split did not reach its resync barrier")
	}
	txValue, ok := tenant.TransactionFromContext(splitCtx)
	require.True(t, ok)
	tx := txValue.(bun.Tx)
	var deleted bool
	require.NoError(t, tx.NewRaw("SELECT NOT EXISTS (SELECT 1 FROM schedule.instance_students WHERE id = ?)", participant.ID).Scan(splitCtx, &deleted))
	require.True(t, deleted, "the real predecessor resync must delete the participant in the uncommitted split")
	var splitPID int
	require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(splitCtx, &splitPID))
	bulkDone := make(chan error, 1)
	running.Go(func() {
		bulkDone <- testpkg.WithTenantTx(t, ctx, f.s.db, f.s.tenantID, func(bulkCtx context.Context, _ bun.Tx) error {
			_, err := f.s.instance.BulkCancelPlanned(bulkCtx, from, to, timetable.BulkCancelOptions{}, nil)
			return err
		})
	})
	awaitSplitCancelWaiter(t, splitCtx, tx, splitPID, bulkDone)

	release()
	splitErr := awaitSplitCancel(t, ctx, splitDone)
	bulkErr := awaitSplitCancel(t, ctx, bulkDone)
	require.NoError(t, splitErr, "split must commit without 40P01")
	require.NoError(t, bulkErr, "bulk cancellation must commit without 40P01")
	require.Empty(t, testpkg.ActivityInstancesWhere(t, f.s.ctx, f.s.db,
		`"activity_instance".activity_group_id = ? AND "activity_instance".date = ?`, f.update.TemplateID, day))
	var remaining int
	require.NoError(t, f.s.db.NewRaw("SELECT count(*) FROM schedule.activity_instances WHERE tenant_id = ? AND date BETWEEN ? AND ? AND status = 'planned'", f.s.tenantID, from, to).Scan(ctx, &remaining))
	require.Zero(t, remaining, "cancellation must see and remove the committed successor too")
}

func awaitSplitCancel(t *testing.T, ctx context.Context, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		t.Fatal("split/cancellation did not finish: ", ctx.Err())
		return ctx.Err()
	}
}

// PostgreSQL decides when the competing transaction is blocked. The ticker
// only paces observation; the go test timeout, not a local load-sensitive
// deadline, bounds a command that never queues (#5221/#5271).
func awaitSplitCancelWaiter(t *testing.T, ctx context.Context, tx bun.Tx, holderPID int, done <-chan error) {
	t.Helper()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		var waiting bool
		require.NoError(t, tx.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_locks l
   WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))`, holderPID).Scan(ctx, &waiting))
		if waiting {
			return
		}
		select {
		case err := <-done:
			require.NoError(t, err)
			t.Fatal("cancellation bypassed the uncommitted split")
		case <-poll.C:
		}
	}
}
