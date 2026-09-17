package repositories_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestIdentityMembershipRuntimeEvidence records the runtime evidence for the
// #2721 cutover of the People Directory, Organisation & Tenancy and parent
// portal reads that filter by school mapping and roles: statement counts,
// latency, returned rows, pool waits, lock-wait events and deadlocks per
// operation on an isolated clone. The harness only uses constructors that
// exist before and after the cutover, so the same file measures both sides;
// the raw JSON is logged for docs/runtime-checkpoints.
func TestIdentityMembershipRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	factory, err := repositories.NewFactoryWithPeopleDirectory(db, repositories.NewUnobservedTimetableDependencies(db))
	require.NoError(t, err)
	staff, err := repositories.NewStaffMessagingTestRepositories(db)
	require.NoError(t, err)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	colleague, colleagueAccount := testpkg.CreateTestStaffWithAccount(t, db, "Evidence", "Colleague")
	require.NotNil(t, colleague)
	testpkg.AssignLehrkraftSystemRole(t, db, colleagueAccount.ID, chain.TenantID)
	// authorize.GuardianPermissionPortalAccess; this test scope may not import
	// the security runtime.
	perm := "parent_portal.access"

	counter := testpkg.CaptureQueriesForContext(t, db)
	testpkg.AttachLockWaitEvidence(db)
	ctx, events := testpkg.CaptureUnitOfWorkEvidence(counter.Context(testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)))
	var version string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &version))
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &count))
		return count
	}
	beforeDeadlocks := deadlocks()
	samples := map[string][]testpkg.RuntimeCheckpointSample{}
	var failures []string
	// measure times fn inside a fresh transaction of the given kind and
	// records the statements fn issued; the transaction scope statements are
	// part of the count on both sides.
	measure := func(operation string, iteration int, admin bool, fn func(context.Context) (int, error)) {
		counter.Reset()
		before := db.Stats()
		started := time.Now()
		var returned int
		run := func(txCtx context.Context, _ bun.Tx) error {
			var err error
			returned, err = fn(txCtx)
			return err
		}
		var err error
		if admin {
			err = testpkg.WithAdminTx(t, ctx, db, run)
		} else {
			err = testpkg.WithTenantTx(t, ctx, db, chain.TenantID, run)
		}
		elapsed := time.Since(started)
		after := db.Stats()
		if iteration < 5 {
			require.NoError(t, err, "warmup %s", operation)
			return
		}
		sample := testpkg.RuntimeCheckpointSample{
			DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(),
			PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond),
		}
		if err != nil {
			sample.Status = 1
			sample.ErrorBody = err.Error()
			failures = append(failures, operation+": "+err.Error())
		} else {
			writes := counter.WriteRows()
			sample.WriteRowsAffected = &writes
			sample.RowsAffected = int64(returned)
			_, sample.StatementsWithRows = counter.Rows()
		}
		samples[operation] = append(samples[operation], sample)
	}
	boolRows := func(ok bool) int {
		if ok {
			return 1
		}
		return 0
	}

	for iteration := range 35 {
		measure("guardian_account_permission", iteration, false, func(txCtx context.Context) (int, error) {
			ok, err := factory.StudentGuardian.AccountHasStudentPermission(txCtx, chain.AccountID, chain.StudentID, chain.TenantID, perm)
			return boolRows(ok), err
		})
		measure("guardian_email_permission", iteration, false, func(txCtx context.Context) (int, error) {
			ok, err := factory.StudentGuardian.GuardianEmailHasStudentPermission(txCtx, chain.Email, chain.StudentID, chain.TenantID, perm)
			return boolRows(ok), err
		})
		measure("guardian_access_filter", iteration, false, func(txCtx context.Context) (int, error) {
			ids, err := factory.StudentGuardian.FilterAccountsWithStudentAccess(txCtx, []int64{chain.AccountID}, []int64{chain.StudentID}, chain.TenantID, perm)
			return len(ids), err
		})
		measure("portal_profile_reachability", iteration, false, func(txCtx context.Context) (int, error) {
			profiles, err := factory.GuardianProfile.FindActivePortalProfilesByIDs(txCtx, []int64{chain.GuardianProfileID})
			return len(profiles), err
		})
		measure("messageable_guardians", iteration, false, func(txCtx context.Context) (int, error) {
			guardians, err := factory.ParentMessageThread.ListGuardiansForStudent(txCtx, chain.StudentID)
			return len(guardians), err
		})
		measure("messageable_staff", iteration, false, func(txCtx context.Context) (int, error) {
			rows, err := staff.Read.ListMessageableStaff(txCtx, chain.AccountID)
			return len(rows), err
		})
		measure("is_messageable_staff", iteration, false, func(txCtx context.Context) (int, error) {
			ok, err := staff.Read.IsMessageableStaff(txCtx, colleagueAccount.ID)
			return boolRows(ok), err
		})
		measure("staff_role_kinds", iteration, false, func(txCtx context.Context) (int, error) {
			kinds, err := staff.Read.StaffRoleKinds(txCtx, []int64{colleagueAccount.ID, chain.AccountID})
			return len(kinds), err
		})
		measure("parent_children", iteration, true, func(txCtx context.Context) (int, error) {
			children, err := factory.ParentChild.ListByAccount(txCtx, chain.AccountID)
			return len(children), err
		})
		measure("parent_submit_status", iteration, true, func(txCtx context.Context) (int, error) {
			status, err := factory.ParentEnrollablePhase.GuardianSubmitStatus(txCtx, chain.AccountID, chain.TenantID)
			if err != nil || status == nil {
				return 0, err
			}
			return boolRows(status.Linked), nil
		})
		measure("schools_of_account", iteration, true, func(txCtx context.Context) (int, error) {
			schools, err := factory.School.FindActiveByAccountID(txCtx, chain.AccountID)
			return len(schools), err
		})
		measure("operator_stats", iteration, true, func(txCtx context.Context) (int, error) {
			stats, err := factory.OperatorSummaries.Stats(txCtx)
			if err != nil || stats == nil {
				return 0, err
			}
			return 1, nil
		})
		measure("operator_school_summaries", iteration, true, func(txCtx context.Context) (int, error) {
			rows, err := factory.OperatorSummaries.SchoolSummaries(txCtx)
			return len(rows), err
		})
	}
	raw, err := json.Marshal(map[string]any{"postgres": version, "warmup": 5, "samples_per_operation": 30, "concurrency": 1, "samples": samples, "unit_of_work_events_including_warmup": events(), "deadlocks": deadlocks() - beforeDeadlocks})
	require.NoError(t, err)
	t.Logf("identity-membership-runtime %s", raw)
	require.Empty(t, failures, "every measured sample must succeed")
}
