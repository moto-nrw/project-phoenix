package behaviortest_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-ogs/project-phoenix/modules/studentpresence/compose"
	active "github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/ports"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

type validationStaffDirectory struct {
	read func(context.Context, []int64) (map[int64]*ports.SessionStaff, error)
}

func (d validationStaffDirectory) SupervisionStaff(ctx context.Context, ids []int64) (map[int64]*ports.SessionStaff, error) {
	return d.read(ctx, ids)
}

func TestTransferredSupervisorConflictPrecedesLaterEnrichmentFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	module := testSchoolPresence(t, db)
	first := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	later := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	firstStaff := testpkg.CreateTestStaff(t, db, "Unlocked", "Supervisor")
	laterStaff := testpkg.CreateTestStaff(t, db, "Later", "Supervisor")
	for _, pair := range [][2]int64{{first.ID, firstStaff.ID}, {later.ID, laterStaff.ID}} {
		_, err := module.RecordSupervision(ctx, studentpresence.GroupSupervision{
			GroupID: pair[0], StaffID: pair[1], Role: "supervisor", StartDate: "2026-01-01",
		})
		require.NoError(t, err)
	}
	enrichmentFailure := errors.New("later supervision staff directory unavailable")
	var enriched [][]int64
	records := owners.ComposePresenceSessionRecords(presenceCompose.SessionRecordDependencies{DB: db, Staff: validationStaffDirectory{read: func(_ context.Context, ids []int64) (map[int64]*ports.SessionStaff, error) {
		enriched = append(enriched, slices.Clone(ids))
		if slices.Contains(ids, laterStaff.ID) {
			return nil, enrichmentFailure
		}
		return nil, nil
	}}, Now: func() time.Time { return calendar.NewDate(2026, 11, 5).BerlinMidnight() }})
	repo := sessionSupervisors(records)
	svc := active.NewService(active.ServiceDependencies{SupervisorRepo: repo})
	err := testpkg.TenantCommand0(t, ctx, db, func(txCtx context.Context) error {
		return active.ValidateTransferredSupervisorLocksForTest(svc, txCtx, []int64{first.ID, later.ID}, nil)
	})
	require.ErrorIs(t, err, studentpresence.ErrSessionConflict)
	require.NotErrorIs(t, err, enrichmentFailure)
	require.Equal(t, [][]int64{{firstStaff.ID}}, enriched, "later enrichment must not displace an earlier group conflict")
}

// TestTransferredSupervisorValidationKeepsGroupOrder pins the guard's order
// (#5783): every group is read on its own, in order, without a lock, up to the
// first group with an unlocked supervisor; the staff of the groups read so
// far resolve in one directory read, and its failure answers before any
// conflict.
func TestTransferredSupervisorValidationKeepsGroupOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	module := testSchoolPresence(t, db)
	type source struct{ group, staff int64 }
	sources := make([]source, 3)
	for i := range sources {
		group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
		staff := testpkg.CreateTestStaff(t, db, "Order", fmt.Sprintf("Supervisor%d", i))
		_, err := module.RecordSupervision(ctx, studentpresence.GroupSupervision{
			GroupID: group.ID, StaffID: staff.ID, Role: "supervisor", StartDate: "2026-01-01",
		})
		require.NoError(t, err)
		sources[i] = source{group: group.ID, staff: staff.ID}
	}
	groupIDs := []int64{sources[0].group, sources[1].group, sources[2].group}
	failure := errors.New("supervision staff directory unavailable")
	counter := testpkg.CaptureQueries(t, db)
	validate := func(t *testing.T, locked map[int64]bool, failOn int64) ([][]int64, []string, error) {
		t.Helper()
		var enriched [][]int64
		records := owners.ComposePresenceSessionRecords(presenceCompose.SessionRecordDependencies{DB: db, Staff: validationStaffDirectory{read: func(_ context.Context, ids []int64) (map[int64]*ports.SessionStaff, error) {
			enriched = append(enriched, slices.Clone(ids))
			if slices.Contains(ids, failOn) {
				return nil, failure
			}
			return nil, nil
		}}, Now: func() time.Time { return calendar.NewDate(2026, 11, 5).BerlinMidnight() }})
		svc := active.NewService(active.ServiceDependencies{SupervisorRepo: sessionSupervisors(records)})
		counter.Reset()
		err := testpkg.TenantCommand0(t, ctx, db, func(txCtx context.Context) error {
			return active.ValidateTransferredSupervisorLocksForTest(svc, txCtx, groupIDs, locked)
		})
		reads := counter.Matching(func(sql string) bool {
			return strings.HasPrefix(strings.TrimSpace(sql), "select") && strings.Contains(sql, "group_supervisors")
		})
		for _, read := range reads {
			require.NotContains(t, strings.ToLower(read), "for update", "the guard adds no lock")
			require.NotContains(t, strings.ToLower(read), "for share", "the guard adds no lock")
		}
		return enriched, reads, err
	}
	all := map[int64]bool{sources[0].staff: true, sources[1].staff: true, sources[2].staff: true}
	firstTwo := map[int64]bool{sources[0].staff: true, sources[1].staff: true}

	t.Run("stops at the first conflicting group", func(t *testing.T) {
		enriched, reads, err := validate(t, map[int64]bool{sources[0].staff: true}, sources[2].staff)
		require.ErrorIs(t, err, studentpresence.ErrSessionConflict)
		require.Len(t, reads, 2, "one read per group up to the conflict")
		require.Equal(t, [][]int64{{sources[0].staff, sources[1].staff}}, enriched)
	})
	t.Run("earlier enrichment failure precedes a later conflict", func(t *testing.T) {
		enriched, _, err := validate(t, firstTwo, sources[0].staff)
		require.ErrorIs(t, err, failure)
		require.NotErrorIs(t, err, studentpresence.ErrSessionConflict)
		require.Equal(t, [][]int64{{sources[0].staff, sources[1].staff, sources[2].staff}}, enriched)
	})
	t.Run("conflicting group's own enrichment failure precedes its conflict", func(t *testing.T) {
		_, _, err := validate(t, firstTwo, sources[2].staff)
		require.ErrorIs(t, err, failure)
	})
	t.Run("enrichment failure without conflict", func(t *testing.T) {
		_, reads, err := validate(t, all, sources[2].staff)
		require.ErrorIs(t, err, failure)
		require.Len(t, reads, 3)
	})
	t.Run("all supervisors locked", func(t *testing.T) {
		enriched, reads, err := validate(t, all, 0)
		require.NoError(t, err)
		require.Len(t, reads, 3, "one read per group")
		require.Equal(t, [][]int64{{sources[0].staff, sources[1].staff, sources[2].staff}}, enriched, "one directory read for every group")
	})
}

func TestForceStartSupervisorValidationGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, count := range []int{3, 8} {
		t.Run(fmt.Sprintf("N%d", count), func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			ctx := testpkg.Ctx(t)
			module := testSchoolPresence(t, db)
			svc, _ := newServiceWithBroadcaster(t, db, module)
			active.ConfigureForTest(svc, active.WithTenantRuntime(testpkg.TenantRuntime(t, db)))
			activity := testpkg.CreateTestActivityGroup(t, db, "Validation Growth")
			target := testpkg.CreateTestRoom(t, db, "Validation Target")
			device := testpkg.CreateTestDevice(t, db, "validation-growth")
			requested := testpkg.CreateTestStaff(t, db, "Requested", "Supervisor")
			groupIDs, visitIDs, supervisorIDs := make([]int64, 0, count), make([]int64, 0, count), []int64{requested.ID}
			for i := range count {
				room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Source %d", i))
				group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
				staff := testpkg.CreateTestStaff(t, db, "Source", fmt.Sprintf("Supervisor%d", i))
				testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
				student := testpkg.CreateTestStudent(t, db, "Source", fmt.Sprintf("Child%d", i), "3a")
				visit := testpkg.CreateTestVisit(t, db, student.ID, group.ID, time.Now().Add(-time.Hour), nil)
				groupIDs = append(groupIDs, group.ID)
				visitIDs = append(visitIDs, visit.ID)
				supervisorIDs = append(supervisorIDs, staff.ID)
			}
			counter := testpkg.CaptureQueries(t, db)
			created, err := svc.ForceStartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{requested.ID}, &target.ID)
			queries := counter.Queries()
			counter.Stop()
			require.NoError(t, err)
			require.NotNil(t, created)
			require.Positive(t, created.ID)
			require.Equal(t, testpkg.Tenant(t), created.TenantID)
			require.Equal(t, &activity.ID, created.GroupID)
			require.Equal(t, &device.ID, created.DeviceID)
			require.Equal(t, target.ID, created.RoomID)
			require.Nil(t, created.EndTime)
			for _, id := range groupIDs {
				old, readErr := svc.GetActiveGroup(ctx, id)
				require.NoError(t, readErr)
				require.NotNil(t, old.EndTime)
			}
			for _, id := range visitIDs {
				visit, readErr := module.FindVisit(ctx, id)
				require.NoError(t, readErr)
				require.Equal(t, created.ID, visit.ActiveGroupID)
				require.Nil(t, visit.ExitTime)
			}
			rows, err := module.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{GroupIDs: []int64{created.ID}, OpenOnly: true})
			require.NoError(t, err)
			gotStaff := make([]int64, 0, len(rows))
			for _, row := range rows {
				gotStaff = append(gotStaff, row.StaffID)
			}
			require.ElementsMatch(t, supervisorIDs, gotStaff)
			t.Logf("complete force-start service N%d: %d statements", count, len(queries))
			if count == 3 {
				querybudgets.AssertQueryBudget(t, "modules.studentpresence.force_start.sources_3", counter.Measurement(queries))
			} else {
				querybudgets.AssertQueryBudget(t, "modules.studentpresence.force_start.sources_8", counter.Measurement(queries))
			}
		})
	}
}
