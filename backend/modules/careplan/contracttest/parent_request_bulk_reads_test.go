package contracttest_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The bulk approval reads every candidate before it locks anything (#5804).
// These tests pin that read phase: its statement count per approved request,
// that no row is locked when a candidate refuses, the refusal the request
// list reports first, and that a failing read stays a failure.

type bulkReadFixture struct {
	db          *bun.DB
	children    []requestLockFixture
	coordinator parentrequests.Coordinator
}

func newBulkReadFixture(t *testing.T, db *bun.DB, children int) bulkReadFixture {
	t.Helper()
	f := bulkReadFixture{db: db}
	for range children {
		f.children = append(f.children, newRequestLockFixture(t, db))
	}
	first := f.children[0]
	coordinator, err := compose.NewParentRequestCoordinator(compose.ParentRequestCoordinatorDependencies{
		Rights: func(context.Context) compose.ParentRequestRights {
			return compose.ParentRequestRights{WriteQueues: true, Absences: true}
		},
		MasterData: first.master, Excused: first.excused, Care: first.care.svc.(compose.ParentRequestConflicts), Offering: unusedConflictPort{},
	})
	require.NoError(t, err)
	f.coordinator = coordinator
	return f
}

// refs names one Stammdaten and one absence request per child.
func (f bulkReadFixture) refs() []parentrequests.Ref {
	refs := make([]parentrequests.Ref, 0, 2*len(f.children))
	for _, child := range f.children {
		refs = append(refs,
			parentrequests.Ref{Kind: parentrequests.KindMasterData, ID: child.masterRow.ID, ExpectedVersion: careplan.ParentRequestVersion(child.masterRow.UpdatedAt)},
			parentrequests.Ref{Kind: parentrequests.KindExcused, ID: child.excusedRow.ID, ExpectedVersion: careplan.ParentRequestVersion(child.excusedRow.UpdatedAt)},
		)
	}
	return refs
}

func (f bulkReadFixture) approve(t *testing.T, ctx context.Context, refs []parentrequests.Ref) error {
	t.Helper()
	var approveErr error
	_ = testpkg.WithTenantTx(t, ctx, f.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		approveErr = f.coordinator.BulkApprove(txCtx, parentrequests.BulkApproveInput{Requests: refs, Reason: "Reviewed", ReviewerID: f.children[0].care.staffAccount})
		return errRollback
	})
	return approveErr
}

func (f bulkReadFixture) ctx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.children[0].care.staffCtx(f.children[0].care.staffAccount), 20*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// lockStatements are the statements that take a row lock.
func lockStatements(queries []string) []string {
	var locks []string
	for _, query := range queries {
		if strings.Contains(strings.ToUpper(query), "FOR UPDATE") {
			locks = append(locks, query)
		}
	}
	return locks
}

func TestBulkApprovalReadPhaseStatements(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	// Statements before the first row lock, and of the whole approval, for 3
	// and 8 children (6 and 16 requests). The read phase does not grow with
	// the selection (it was 23 and 58 with one read pass per request); the
	// lock and decision phases are unchanged (136 and 361). The fixture binds
	// no guardian-access check, which adds one read per absence in production.
	want := map[int]struct{ beforeLock, total int }{3: {beforeLock: 9, total: 145}, 8: {beforeLock: 9, total: 370}}
	for _, children := range []int{3, 8} {
		t.Run(fmt.Sprintf("children=%d", children), func(t *testing.T) {
			f := newBulkReadFixture(t, db, children)
			ctx := f.ctx(t)
			counter := testpkg.CaptureQueries(t, db)
			require.NoError(t, f.approve(t, ctx, f.refs()))
			counter.Stop()
			queries := counter.Queries()
			beforeLock := len(queries)
			for i, query := range queries {
				if strings.Contains(strings.ToUpper(query), "FOR UPDATE") {
					beforeLock = i
					break
				}
			}
			t.Logf("children=%d: statements before the first lock %d, total %d", children, beforeLock, len(queries))
			assert.Equal(t, want[children].beforeLock, beforeLock, "read phase")
			assert.Equal(t, want[children].total, len(queries), "whole approval")
		})
	}
}

// A candidate that refuses (stale, ineligible, gone) refuses before ANY row
// lock: the read phase never locks, and the lock phase starts only after
// every candidate passed.
func TestBulkApprovalRefusesBeforeLockingAnyRow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	f := newBulkReadFixture(t, db, 3)
	ctx := f.ctx(t)
	for _, tc := range []struct {
		name   string
		mutate func([]parentrequests.Ref) []parentrequests.Ref
		want   error
	}{
		{"last request stale", func(refs []parentrequests.Ref) []parentrequests.Ref {
			refs[len(refs)-1].ExpectedVersion = "stale"
			return refs
		}, parentrequests.ErrStale},
		{"last absence gone", func(refs []parentrequests.Ref) []parentrequests.Ref {
			refs[len(refs)-1].ID = refs[len(refs)-1].ID + 1_000_000
			return refs
		}, parentrequests.ErrNotFound},
		{"last Stammdaten request gone", func(refs []parentrequests.Ref) []parentrequests.Ref {
			refs[len(refs)-2].ID = refs[len(refs)-2].ID + 1_000_000
			return refs
		}, parentrequests.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			counter := testpkg.CaptureQueries(t, db)
			err := f.approve(t, ctx, tc.mutate(f.refs()))
			counter.Stop()
			require.ErrorIs(t, err, tc.want)
			assert.Empty(t, lockStatements(counter.Queries()), "no row may be locked before every candidate passed")
		})
	}
}

// Requests sort by kind ("excused" before "master_data"). A Stammdaten request
// that is gone wins over an earlier absence that is merely stale: loading
// stops at the missing request before any version is compared.
func TestBulkApprovalReportsAMissingRequestBeforeAStaleOne(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	f := newBulkReadFixture(t, db, 2)
	ctx := f.ctx(t)
	refs := f.refs()
	refs[1].ExpectedVersion = "stale"   // absence of child 1
	refs[2].ID = refs[2].ID + 1_000_000 // Stammdaten of child 2
	err := f.approve(t, ctx, refs)
	require.ErrorIs(t, err, parentrequests.ErrNotFound)
	require.NotErrorIs(t, err, parentrequests.ErrStale)
}

// A read that fails in the database fails the approval: no 4xx refusal, no
// success, and nothing is written. The second connection holds the request
// table so the read waits; cancelling that backend fails the statement.
func TestBulkApprovalFailedReadFailsTheApproval(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	f := newBulkReadFixture(t, db, 3)
	ctx := f.ctx(t)
	for _, table := range []string{"active.excused_absence_requests", "users.student_data_change_requests"} {
		t.Run(table, func(t *testing.T) {
			done := make(chan error, 1)
			holder, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = holder.Rollback() }()
			_, err = holder.ExecContext(ctx, "LOCK TABLE "+table+" IN ACCESS EXCLUSIVE MODE")
			require.NoError(t, err)
			go func() { done <- f.approve(t, ctx, f.refs()) }()
			var pid int
			require.Eventually(t, func() bool {
				return holder.NewRaw("SELECT COALESCE((SELECT l.pid FROM pg_locks l WHERE NOT l.granted AND l.relation = ?::regclass LIMIT 1), 0)", table).Scan(ctx, &pid) == nil && pid != 0
			}, 5*time.Second, 10*time.Millisecond, "the approval must wait on the request table")
			_, err = holder.ExecContext(ctx, "SELECT pg_cancel_backend(?)", pid)
			require.NoError(t, err)
			approveErr := awaitRequestWriter(t, ctx, done)
			require.Error(t, approveErr)
			for _, refusal := range []error{parentrequests.ErrStale, parentrequests.ErrNotFound, parentrequests.ErrBulkIneligible} {
				require.NotErrorIs(t, approveErr, refusal)
			}
			require.NoError(t, holder.Rollback())
			for _, child := range f.children {
				var pending int
				require.NoError(t, db.NewRaw(`SELECT
					(SELECT count(*) FROM active.excused_absence_requests WHERE id = ? AND status = 'pending') +
					(SELECT count(*) FROM users.student_data_change_requests WHERE id = ? AND status = 'pending')`, child.excusedRow.ID, child.masterRow.ID).Scan(ctx, &pending))
				require.Equal(t, 2, pending, "a failed approval decides nothing")
			}
		})
	}
}
