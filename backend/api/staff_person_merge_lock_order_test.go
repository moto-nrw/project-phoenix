package api

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	peopleCompose "github.com/moto-ogs/project-phoenix/modules/peopledirectory/compose"
	membershipCompose "github.com/moto-ogs/project-phoenix/modules/schoolmembership/compose"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/moto-ogs/project-phoenix/workflows/staffpersonmerge"
	mergeCompose "github.com/moto-ogs/project-phoenix/workflows/staffpersonmerge/compose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// mergeRaceAudit records nothing; the merge's own writes are what the race
// checks. It reports the merge's lock ledger at its audit write, inside the
// merge transaction.
type mergeRaceAudit struct {
	held   chan bool
	ledger func(context.Context) bool
}

func (a mergeRaceAudit) RecordStaffPersonMerge(ctx context.Context, _, _ int64, _ staffpersonmerge.Candidate) error {
	select {
	case a.held <- a.ledger(ctx):
	default:
	}
	return nil
}

func (mergeRaceAudit) RecordStaffPersonMergePreview(context.Context, int64, int64, net.IP) error {
	return nil
}

// mergeRaceOffboarding is the production staff offboarding over the test
// database, acting as the given staff member.
func mergeRaceOffboarding(t *testing.T, db *bun.DB, actorStaffID int64) func(context.Context, int64) error {
	t.Helper()
	identity, err := IdentityAccessForTests(IdentityAccessTestConfig{FrontendURL: "http://localhost:3000", PasswordResetExpiry: time.Hour},
		testpkg.TenantRuntime(t, db), testpkg.NewAuditCommand(owners.NewTestAuditPersistence(testpkg.NewDatabase(t, db), nil).AuthEvent.Create),
		testpkg.NewDatabase(t, db), nil)
	require.NoError(t, err)
	actorAccount := testpkg.CreateTestAccount(t, db, "merge-race-actor@example.org")
	deps, err := NewStaffOffboardingOwners(testpkg.NewDatabase(t, db), peopleCompose.MustNewReads(db, testpkg.StudentDirectory{}),
		owners.MembershipStaffEmployment(MustNewStaffEmployment(testpkg.NewDatabase(t, db))), nil)
	require.NoError(t, err)
	deps.Access = StaffOffboardingAccess(identity)
	offboard, err := StaffOffboardForTest(deps, actorStaffID, actorAccount.ID)
	require.NoError(t, err)
	return offboard
}

// mergeRaceTx is one transaction of the race: its backend, then its outcome.
type mergeRaceTx struct {
	pid  int64
	done chan error
}

func startMergeRaceTx(t *testing.T, db *bun.DB, admin bool, write func(context.Context) error) mergeRaceTx {
	t.Helper()
	pid, done := make(chan int64, 1), make(chan error, 1)
	run := func(ctx context.Context, tx bun.Tx) error {
		var backend int64
		if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &backend); err != nil {
			return err
		}
		pid <- backend
		return write(ctx)
	}
	go func() {
		if admin {
			done <- testpkg.WithAdminTx(t, context.Background(), db, run)
			return
		}
		done <- testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), run)
	}()
	select {
	case backend := <-pid:
		return mergeRaceTx{pid: backend, done: done}
	case err := <-done:
		require.FailNow(t, "race transaction failed before it started", "%v", err)
		return mergeRaceTx{}
	}
}

func awaitMergeRaceBlockedBy(t *testing.T, db *bun.DB, run mergeRaceTx, blocker int64, what string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		select {
		case err := <-run.done:
			require.FailNow(t, what+" finished instead of waiting", "%v", err)
		default:
		}
		if time.Now().After(deadline) {
			require.FailNow(t, what+" never waited for the blocker")
		}
		var blocked bool
		require.NoError(t, db.NewRaw(`SELECT ?::int = ANY(pg_blocking_pids(?::int))`, blocker, run.pid).Scan(context.Background(), &blocked))
		if blocked {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func requireMergeRaceFinished(t *testing.T, run mergeRaceTx, what string) {
	t.Helper()
	select {
	case err := <-run.done:
		var state interface{ Field(byte) string }
		if errors.As(err, &state) && state.Field('C') == "40P01" {
			require.FailNow(t, what+" was the deadlock victim (40P01)", "%v", err)
		}
		require.NoError(t, err, "%s must succeed", what)
	case <-time.After(30 * time.Second):
		require.FailNow(t, what+" never finished")
	}
}

// M1 (#6702): offboarding of X has locked X's staff row and waits at X's
// caregiver profile, which a third transaction holds. The merge of a
// namesake pair of the same school starts meanwhile and must wait for X's
// staff row. Before, it locked every person of the school first, X's person
// included, so offboarding, released, then waited for the merge's person
// lock while the merge waited for X's staff row: 40P01. Now the merge locks
// the school's staff rows, ascending and admitted through tenant.OrderLock
// (LockStaffRows), before their people (Konto → Schule → Profil), waits for
// the staff row holding no person, and runs after offboarding.
func TestStaffPersonMergeAndOffboardingDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	actor := testpkg.CreateTestStaff(t, db, "Race", "Actor")
	subject := testpkg.CreateTestTeacher(t, db, "Race", "Subject")
	keeper, _ := testpkg.CreateTestStaffWithAccount(t, db, "Rennen", "Doppelsatz")
	donor := testpkg.CreateTestStaff(t, db, "Rennen", "Doppelsatz")
	_, err := db.NewRaw(`UPDATE users.staff_school_memberships SET deleted_at = '2026-09-02T12:00:00Z' WHERE tenant_id = ? AND id = ?`, testpkg.Tenant(t), donor.ID).Exec(ctx)
	require.NoError(t, err)
	operator := testpkg.CreateTestOperator(t, db)
	audit := mergeRaceAudit{held: make(chan bool, 1), ledger: func(ctx context.Context) bool {
		return tenant.HoldsLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(subject.StaffID)) &&
			tenant.HoldsLock(ctx, tenant.LockStaffPersonMerge, tenant.LockIDKey(testpkg.Tenant(t)))
	}}
	merge, err := mergeCompose.New(peopleCompose.NewStaffPersonMerge(), membershipCompose.NewStaffPersonMerge(), audit,
		func(context.Context) int64 { return operator.ID })
	require.NoError(t, err)
	var candidates []staffpersonmerge.Candidate
	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, func(ctx context.Context) error {
		candidates, err = merge.Preview(ctx, testpkg.Tenant(t), net.IPv4(127, 0, 0, 1))
		return err
	}))
	require.Len(t, candidates, 1)
	require.Equal(t, keeper.PersonID, candidates[0].Keeper.Person.ID)
	offboard := mergeRaceOffboarding(t, db, actor.ID)

	locked, release := make(chan struct{}), make(chan struct{})
	barrier := startMergeRaceTx(t, db, false, func(ctx context.Context) error {
		tx, _ := tenant.CurrentTransaction[bun.IDB](ctx)
		if _, err := tx.NewRaw(`SELECT id FROM users.teachers WHERE id = ? FOR UPDATE`, subject.ID).Exec(ctx); err != nil {
			return err
		}
		close(locked)
		<-release
		return nil
	})
	select {
	case <-locked:
	case err := <-barrier.done:
		require.FailNow(t, "the barrier failed", "%v", err)
	}
	exit := startMergeRaceTx(t, db, false, func(ctx context.Context) error {
		return offboard(ctx, subject.StaffID)
	})
	awaitMergeRaceBlockedBy(t, db, exit, barrier.pid, "offboarding")
	repair := startMergeRaceTx(t, db, true, func(ctx context.Context) error {
		return merge.Execute(ctx, testpkg.Tenant(t), staffpersonmerge.Request{SchoolConfirmed: true, Cases: []staffpersonmerge.Selection{
			{DonorID: candidates[0].Donor.Person.ID, KeeperID: candidates[0].Keeper.Person.ID, Revision: candidates[0].Revision},
		}})
	})
	awaitMergeRaceBlockedBy(t, db, repair, exit.pid, "the merge")
	close(release)
	requireMergeRaceFinished(t, barrier, "the barrier")
	requireMergeRaceFinished(t, exit, "offboarding")
	requireMergeRaceFinished(t, repair, "the merge")
	assert.True(t, <-audit.held, "the merge admitted the staff rows and its gate in the lock order")

	var retired bool
	require.NoError(t, db.NewRaw(`SELECT deleted_at IS NOT NULL FROM users.staff_school_memberships WHERE id = ?`, subject.StaffID).Scan(ctx, &retired))
	assert.True(t, retired, "offboarding retired X")
	var donorPerson int64
	require.NoError(t, db.NewRaw(`SELECT person_id FROM users.staff_school_memberships WHERE id = ?`, donor.ID).Scan(ctx, &donorPerson))
	assert.Equal(t, keeper.PersonID, donorPerson, "the merge moved the donor's employment to the keeper")
}
