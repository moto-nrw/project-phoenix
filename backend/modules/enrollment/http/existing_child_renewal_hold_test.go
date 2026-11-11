package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Issue #6638: a parent's renewal of an existing child holds the parent's
// authority over that child until the submission commits, behind the same
// guard as every parent write for an existing child. A withdrawal committed
// after the check before the write refuses the renewal and leaves no request
// behind; a withdrawal that starts once the renewal holds the guard waits
// for its commit. The tests synchronize on channels and pg_blocking_pids,
// without sleeps.

const renewalDeadline = 20 * time.Second

// renewalGate parks the first call while armed and reports the backend of
// the submission's transaction.
type renewalGate struct {
	armed   atomic.Bool
	once    sync.Once
	opened  sync.Once
	reached chan renewalBackend
	release chan struct{}
}

type renewalBackend struct {
	pid int
	err error
}

func newRenewalGate() *renewalGate {
	return &renewalGate{reached: make(chan renewalBackend, 1), release: make(chan struct{})}
}

func (g *renewalGate) open() { g.opened.Do(func() { close(g.release) }) }

func (g *renewalGate) park(ctx context.Context) {
	if !g.armed.Load() {
		return
	}
	g.once.Do(func() {
		var backend renewalBackend
		if tx, ok := tenant.TransactionFromContext(ctx); !ok {
			backend.err = errors.New("the renewal runs outside the school transaction")
		} else {
			backend.err = tx.(bun.IDB).NewRaw("SELECT pg_backend_pid()").Scan(ctx, &backend.pid)
		}
		g.reached <- backend
		<-g.release
	})
}

// checkedMatches parks after the first name-and-birthday match of a
// submission, the check before the renewal holds anything.
type checkedMatches struct {
	owners.EnrollmentStudentMatches
	gate *renewalGate
}

func (m checkedMatches) FindEnrolledStudentIDsByIdentities(ctx context.Context, tenantID int64, identities []peopledirectory.StudentIdentity) ([][]int64, error) {
	ids, err := m.EnrollmentStudentMatches.FindEnrolledStudentIDsByIdentities(ctx, tenantID, identities)
	m.gate.park(ctx)
	return ids, err
}

// writingChildren parks before the first write of a request child.
type writingChildren struct {
	enrollmentCompose.IntakeChildren
	gate *renewalGate
}

func (c writingChildren) InsertChild(ctx context.Context, row *enrollmentNative.RequestChild) error {
	c.gate.park(ctx)
	return c.IntakeChildren.InsertChild(ctx, row)
}

// waitingRenewalRequests parks after the parent guard and before either
// phase lock, so an approval can contend for the held student.
type waitingRenewalRequests struct {
	enrollmentCompose.IntakeRequests
	gate *renewalGate
}

func (r waitingRenewalRequests) AcquireSubmissionDedupLock(ctx context.Context, phaseID int64, key uint64) error {
	r.gate.park(ctx)
	return r.IntakeRequests.AcquireSubmissionDedupLock(ctx, phaseID, key)
}

type renewalRig struct {
	*existingChildEnv
	svc                     enrollmentAPI.RequestService
	checked, writing, dedup *renewalGate
}

func setupRenewalRig(t *testing.T) *renewalRig {
	t.Helper()
	return setupRenewalRigFor(t, enrollmentNative.PhaseAudienceOpen)
}

func setupRenewalRigFor(t *testing.T, audience string) *renewalRig {
	t.Helper()
	env := setupExistingChildTest(t, audience)
	rig := &renewalRig{existingChildEnv: env, checked: newRenewalGate(), writing: newRenewalGate(), dedup: newRenewalGate()}
	t.Cleanup(func() { rig.checked.open(); rig.writing.open(); rig.dedup.open() })
	rig.svc = newTestRequestService(owners.EnrollmentIntakeSources{
		Bookings: requestTestBookingCommands(), Requests: waitingRenewalRequests{IntakeRequests: env.repos.Enrollment(), gate: rig.dedup},
		Children:  writingChildren{IntakeChildren: env.repos.Enrollment(), gate: rig.writing},
		Guardians: env.repos.Enrollment(), CareOfferingRepo: enrollmentCompose.NewCareOfferingRecords(env.reposCare.CarePlan),
		Catalog: env.repos.Enrollment(), SchoolRepo: factorySchools{repos: env.reposSchools}, RateLimitRepo: env.repos.Enrollment(),
		LateInviteRepo: env.repos.Enrollment(), StudentRepo: checkedMatches{EnrollmentStudentMatches: env.repos.Student, gate: rig.checked},
		GuardianAuthorizer: env.repos.StudentGuardian,
		GuardianHold:       testGuardianHold(env.db, env.repos),
		OutboxEnqueuer:     env.outbox, Settings: env.settings, FrontendURL: "http://localhost:3000", Logger: slog.Default(),
	})
	return rig
}

// renewalFamily is an enrolled child and a parent account whose relationship
// grants the renewal.
type renewalFamily struct {
	studentID, accountID int64
	email                string
	first, last          string
}

func (r *renewalRig) family(t *testing.T, last string) renewalFamily {
	t.Helper()
	existing := enrolledChild(t, r.db, "Lina", last, existingChildBirthday)
	accountID := parentAccountLinkedTo(t, r.db, existing.ID, securityruntime.GuardianRolePrimaryGuardian)
	return renewalFamily{studentID: existing.ID, accountID: accountID, email: "renewal-" + strings.ToLower(last) + "@example.test", first: "Lina", last: last}
}

// submitAsync submits the family's renewal without waiting.
func (r *renewalRig) submitAsync(t *testing.T, f renewalFamily) <-chan error {
	t.Helper()
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
	t.Cleanup(cancel)
	done := make(chan error, 1)
	accountID := f.accountID
	go func() {
		_, err := r.svc.Submit(ctx, enrollmentAPI.SubmitRequest{
			TenantID: testpkg.Tenant(t), PhaseID: r.sourcePhase.ID,
			GuardianFirstName: "Eva", GuardianLastName: f.last, GuardianEmail: f.email,
			GuardianAccountID: &accountID, GuardianSubmitEligible: true,
			ConsentFlags: testFormValues(rolloverConsentFlags()),
			Children:     []enrollmentAPI.SubmitChild{submitChild(f.first, f.last, existingChildBirthday)},
		})
		done <- err
	}()
	return done
}

// requestsOf counts the stored requests of the family's address.
func (r *renewalRig) requestsOf(t *testing.T, f renewalFamily) int {
	t.Helper()
	count, err := r.db.NewSelect().TableExpr(`enrollment.requests AS r`).
		Where("r.tenant_id = ? AND r.guardian_email = ?", testpkg.Tenant(t), f.email).
		Count(context.Background())
	require.NoError(t, err)
	return int(count)
}

// renewalWithdrawal ends one fact the renewal needs, through the owner's
// regular write.
type renewalWithdrawal struct {
	name string
	run  func(t *testing.T, r *renewalRig, f renewalFamily) func() error
}

var renewalWithdrawals = []renewalWithdrawal{
	{name: "Recht enrollment.submit entzogen", run: func(t *testing.T, r *renewalRig, f renewalFamily) func() error {
		links, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), r.db, r.repos.StudentGuardian.FindByStudentID, f.studentID)
		require.NoError(t, err)
		require.Len(t, links, 1)
		link := links[0]
		var permissions map[string]any
		require.NoError(t, json.Unmarshal(link.Permissions, &permissions))
		require.Equal(t, true, permissions["parent_portal.enrollment.submit"], "the fixture grants the renewal")
		permissions["parent_portal.enrollment.submit"] = false
		link.Permissions, err = json.Marshal(permissions)
		require.NoError(t, err)
		ctx := testpkg.Ctx(t)
		return func() error {
			return testpkg.WithTenantTx(t, ctx, r.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
				_, err := r.repos.StudentGuardian.UpdateColumns(txCtx, link, "permissions")
				return err
			})
		}
	}},
	{name: "Beziehung entfernt", run: func(t *testing.T, r *renewalRig, f renewalFamily) func() error {
		links, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), r.db, r.repos.StudentGuardian.FindByStudentID, f.studentID)
		require.NoError(t, err)
		require.Len(t, links, 1)
		id := links[0].ID
		ctx := testpkg.Ctx(t)
		return func() error {
			return testpkg.WithTenantTx(t, ctx, r.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
				return r.repos.StudentGuardian.Delete(txCtx, id)
			})
		}
	}},
	{name: "Schulmitgliedschaft deaktiviert", run: func(t *testing.T, r *renewalRig, f renewalFamily) func() error {
		return func() error {
			_, err := r.db.NewRaw(`UPDATE auth.account_tenants SET status = 'inactive', deactivated_at = NOW(), updated_at = NOW()
				WHERE account_id = ? AND tenant_id = ? AND status = 'active'`, f.accountID, testpkg.Tenant(t)).Exec(context.Background())
			return err
		}
	}},
}

func awaitRenewal(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(renewalDeadline + 5*time.Second):
		t.Fatal("the renewal did not answer")
		return nil
	}
}

// A withdrawal committed after the check before the guard refuses the
// renewal of an existing_students phase; nothing of it is stored. (An open
// phase without the relationship stores the child unpinned for the office.)
func TestParentRenewalRefusesAfterWithdrawalBeforeTheGuard(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, change := range renewalWithdrawals {
		t.Run(change.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			rig := setupRenewalRigFor(t, enrollmentNative.PhaseAudienceExistingStudents)
			f := rig.family(t, "Vorher")
			apply := change.run(t, rig, f)

			rig.checked.armed.Store(true)
			done := rig.submitAsync(t, f)
			select {
			case backend := <-rig.checked.reached:
				require.NoError(t, backend.err)
			case err := <-done:
				t.Fatalf("the renewal answered before it matched the child: %v", err)
			case <-time.After(renewalDeadline):
				t.Fatal("the renewal never matched the child")
			}
			require.NoError(t, apply(), "the withdrawal commits while the renewal waits behind its check")
			rig.checked.open()

			err := awaitRenewal(t, done)
			require.ErrorIs(t, err, enrollmentNative.ErrChildEnrollmentNotPermitted, "the renewal is refused after %s", change.name)
			require.Zero(t, rig.requestsOf(t, f), "a refused renewal stores nothing")
		})
	}
}

// A withdrawal that starts while the renewal holds the guard waits for the
// renewal's commit.
func TestParentRenewalHoldsTheGuardUntilCommit(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, change := range renewalWithdrawals {
		t.Run(change.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			rig := setupRenewalRig(t)
			f := rig.family(t, "Nachher")
			apply := change.run(t, rig, f)

			rig.writing.armed.Store(true)
			done := rig.submitAsync(t, f)
			var held renewalBackend
			select {
			case held = <-rig.writing.reached:
				require.NoError(t, held.err)
			case err := <-done:
				t.Fatalf("the renewal answered before it wrote its child: %v", err)
			case <-time.After(renewalDeadline):
				t.Fatal("the renewal never wrote its child")
			}
			withdrawn := make(chan error, 1)
			go func() { withdrawn <- apply() }()
			require.Eventually(t, func() bool {
				var waiting bool
				err := rig.db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", held.pid).
					Scan(context.Background(), &waiting)
				return err == nil && waiting
			}, 10*time.Second, 10*time.Millisecond, "the withdrawal must wait for the renewal that holds the guard")
			select {
			case err := <-withdrawn:
				t.Fatalf("the withdrawal committed while the renewal held the guard: %v", err)
			default:
			}
			rig.writing.open()
			require.NoError(t, awaitRenewal(t, done), "the held renewal commits first")
			select {
			case err := <-withdrawn:
				require.NoError(t, err)
			case <-time.After(renewalDeadline):
				t.Fatal("the withdrawal did not finish after the renewal")
			}
			require.Equal(t, 1, rig.requestsOf(t, f))
		})
	}
}

// The accountless identity of a late invite holds its relationship as
// well: a removal of the invited guardian's relationship waits for the
// renewal, and the guard asks no school membership or portal access of a
// guardian without an account.
func TestParentLateInviteRenewalHoldsTheGuardUntilCommit(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	rig := setupRenewalRigFor(t, enrollmentNative.PhaseAudienceExistingStudents)
	existing := enrolledChild(t, rig.db, "Lina", "Eingeladen", existingChildBirthday)
	profile := testpkg.CreateTestGuardianProfileNamed(t, rig.db, "Eva", "Eingeladen", "late-invite-guardian")
	require.NotNil(t, profile.Email)
	testpkg.CreateTestStudentGuardianLink(t, rig.db, existing.ID, profile.ID, securityruntime.GuardianRolePrimaryGuardian)
	email := strings.ToLower(*profile.Email)
	invite, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), rig.db, rig.svc.CreateLateInvite, enrollmentAPI.CreateLateInviteInput{
		PhaseID: rig.sourcePhase.ID, GuardianEmail: email, CreatedBy: rig.creatorID,
	})
	require.NoError(t, err)
	links, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), rig.db, rig.repos.StudentGuardian.FindByStudentID, existing.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	linkID := links[0].ID

	rig.writing.armed.Store(true)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := rig.svc.Submit(ctx, enrollmentAPI.SubmitRequest{
			TenantID: testpkg.Tenant(t), PhaseID: rig.sourcePhase.ID, LateInviteToken: invite.Token,
			GuardianFirstName: "Eva", GuardianLastName: "Eingeladen", GuardianEmail: email,
			ConsentFlags: testFormValues(rolloverConsentFlags()),
			Children:     []enrollmentAPI.SubmitChild{submitChild("Lina", "Eingeladen", existingChildBirthday)},
		})
		done <- err
	}()
	var held renewalBackend
	select {
	case held = <-rig.writing.reached:
		require.NoError(t, held.err)
	case err := <-done:
		t.Fatalf("the renewal answered before it wrote its child: %v", err)
	case <-time.After(renewalDeadline):
		t.Fatal("the renewal never wrote its child")
	}
	removeCtx := testpkg.Ctx(t)
	removed := make(chan error, 1)
	go func() {
		removed <- testpkg.WithTenantTx(t, removeCtx, rig.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			return rig.repos.StudentGuardian.Delete(txCtx, linkID)
		})
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := rig.db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", held.pid).
			Scan(context.Background(), &waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond, "the removal must wait for the renewal that holds the guard")
	rig.writing.open()
	require.NoError(t, awaitRenewal(t, done), "the held renewal of the invited guardian commits first")
	select {
	case err := <-removed:
		require.NoError(t, err)
	case <-time.After(renewalDeadline):
		t.Fatal("the removal did not finish after the renewal")
	}
}

// The renewal takes the child before the intake's phase locks, the order of
// the staff decisions, which lock students before the duplicate and match
// locks of the phase. A staff transaction that holds the child and then
// takes those phase locks therefore commits while the renewal waits for the
// child, instead of both waiting for each other.
func TestParentRenewalTakesTheChildBeforeThePhaseLocks(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	rig := setupRenewalRig(t)
	f := rig.family(t, "Reihenfolge")

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
	defer cancel()
	started := make(chan renewalBackend, 1)
	proceed := make(chan struct{})
	staff := make(chan error, 1)
	go func() {
		staff <- testpkg.WithTenantTx(t, ctx, rig.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			var backend renewalBackend
			backend.err = tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &backend.pid)
			if backend.err == nil {
				_, backend.err = rig.repos.Student.FindByIDForUpdate(txCtx, f.studentID)
			}
			started <- backend
			if backend.err != nil {
				return backend.err
			}
			select {
			case <-proceed:
			case <-txCtx.Done():
				return txCtx.Err()
			}
			records := rig.repos.Enrollment()
			if err := records.AcquireSubmissionDedupLock(txCtx, rig.sourcePhase.ID, enrollmentNative.SubmissionDedupLockKey(f.email)); err != nil {
				return err
			}
			return records.AcquireExistingStudentMatchLock(txCtx, rig.sourcePhase.ID)
		})
	}()
	var holder renewalBackend
	select {
	case holder = <-started:
		require.NoError(t, holder.err)
	case <-ctx.Done():
		t.Fatal("the staff transaction never locked the child")
	}

	done := rig.submitAsync(t, f)
	require.Eventually(t, func() bool {
		var waiting bool
		err := rig.db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", holder.pid).
			Scan(context.Background(), &waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond, "the renewal must wait for the child the staff transaction holds")
	close(proceed)

	select {
	case err := <-staff:
		require.NoError(t, err, "the staff transaction takes the phase locks and commits")
	case <-time.After(renewalDeadline):
		t.Fatal("the staff transaction did not commit")
	}
	require.NoError(t, awaitRenewal(t, done), "the renewal commits after the staff transaction")
	require.Equal(t, 1, rig.requestsOf(t, f))
}

// Contact edits take their child before the guardian profile. While a
// sibling renewal waits for the second child, it must not hold the shared
// profile: that would make the contact edit and renewal wait for each other.
// Exercise the contact guard's owner locks against a real late-invite submit,
// both for an accountless profile and an email identity carrying an account.
func TestParentSiblingRenewalTakesAllChildrenBeforeGuardianProfiles(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, withAccount := range []bool{false, true} {
		t.Run(map[bool]string{false: "without_account", true: "with_account"}[withAccount], func(t *testing.T) {
			testpkg.OwnTenant(t)
			rig := setupRenewalRigFor(t, enrollmentNative.PhaseAudienceExistingStudents)
			first := enrolledChild(t, rig.db, "Lina", "Geschwister", existingChildBirthday)
			second := enrolledChild(t, rig.db, "Mila", "Geschwister", existingChildBirthday)
			require.Less(t, first.ID, second.ID)
			profile := testpkg.CreateTestGuardianProfileNamed(t, rig.db, "Eva", "Geschwister", "sibling-renewal")
			if withAccount {
				account := testpkg.CreateTestAccount(t, rig.db, "sibling-renewal")
				_, err := rig.db.NewUpdate().TableExpr(`users.guardian_profiles`).
					Set("account_id = ?", account.ID).Set("has_account = TRUE").
					Where("id = ?", profile.ID).Exec(testpkg.Ctx(t))
				require.NoError(t, err)
			}
			testpkg.CreateTestStudentGuardianLink(t, rig.db, first.ID, profile.ID, securityruntime.GuardianRolePrimaryGuardian)
			testpkg.CreateTestStudentGuardianLink(t, rig.db, second.ID, profile.ID, securityruntime.GuardianRolePrimaryGuardian)
			require.NotNil(t, profile.Email)
			invite, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), rig.db, rig.svc.CreateLateInvite, enrollmentAPI.CreateLateInviteInput{
				PhaseID: rig.sourcePhase.ID, GuardianEmail: *profile.Email, CreatedBy: rig.creatorID,
			})
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
			defer cancel()
			started := make(chan renewalBackend, 1)
			proceed := newRenewalGate()
			defer proceed.open()
			contact := make(chan error, 1)
			go func() {
				contact <- testpkg.WithTenantTx(t, ctx, rig.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
					var backend renewalBackend
					backend.err = tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &backend.pid)
					if backend.err == nil {
						_, backend.err = rig.repos.Student.FindByIDForUpdate(txCtx, second.ID)
					}
					started <- backend
					if backend.err != nil {
						return backend.err
					}
					select {
					case <-proceed.release:
					case <-txCtx.Done():
						return txCtx.Err()
					}
					return rig.repos.GuardianProfile.LockByIDForUpdate(txCtx, profile.ID)
				})
			}()
			var holder renewalBackend
			select {
			case holder = <-started:
				require.NoError(t, holder.err)
			case <-ctx.Done():
				t.Fatal("the contact edit never locked the second child")
			}

			done := make(chan error, 1)
			var result *enrollmentAPI.SubmitResult
			go func() {
				var submitErr error
				result, submitErr = rig.svc.Submit(ctx, enrollmentAPI.SubmitRequest{
					TenantID: testpkg.Tenant(t), PhaseID: rig.sourcePhase.ID, LateInviteToken: invite.Token,
					GuardianFirstName: "Eva", GuardianLastName: "Geschwister", GuardianEmail: *profile.Email,
					ConsentFlags: testFormValues(rolloverConsentFlags()),
					// Reverse input order still locks the lower child ID first.
					Children: []enrollmentAPI.SubmitChild{
						submitChild("Mila", "Geschwister", existingChildBirthday),
						submitChild("Lina", "Geschwister", existingChildBirthday),
					},
				})
				done <- submitErr
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				err := rig.db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", holder.pid).
					Scan(ctx, &waiting)
				return err == nil && waiting
			}, 10*time.Second, 10*time.Millisecond, "the renewal must wait for the second child")
			proceed.open()
			require.NoError(t, awaitRenewal(t, contact), "the contact edit takes the profile and commits without a deadlock")
			require.NoError(t, awaitRenewal(t, done), "the sibling renewal commits after the contact edit")
			require.Len(t, result.Children, 2)
			require.Equal(t, &second.ID, result.Children[0].MatchedStudentID)
			require.Equal(t, &first.ID, result.Children[1].MatchedStudentID)
		})
	}
}

func TestParentRenewalRefusesANewMatchAfterCandidateLookup(t *testing.T) {
	t.Parallel()
	rig := setupRenewalRig(t)
	family := rig.family(t, "VorKorrektur")
	family.last = "NachKorrektur"
	rig.checked.armed.Store(true)
	done := rig.submitAsync(t, family)
	select {
	case backend := <-rig.checked.reached:
		require.NoError(t, backend.err)
	case err := <-done:
		t.Fatalf("submission returned before the candidate lookup: %v", err)
	case <-time.After(renewalDeadline):
		t.Fatal("candidate lookup was not reached")
	}
	_, err := rig.db.NewRaw(`UPDATE users.persons SET last_name = ?
  WHERE id = (SELECT person_id FROM users.student_profiles WHERE id = ? AND tenant_id = ?)`, family.last, family.studentID, testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	rig.checked.open()
	require.ErrorIs(t, awaitRenewal(t, done), enrollmentNative.ErrChildEnrollmentNotPermitted)
	require.Zero(t, rig.requestsOf(t, family), "a new match must abort without storing a request")
}

func TestRenewalHoldQueryCountDoesNotGrowWithSiblings(t *testing.T) {
	t.Parallel()
	rig := setupRenewalRig(t)
	first := rig.family(t, "Gebündelt")
	second := enrolledChild(t, rig.db, "Mila", first.last, existingChildBirthday)
	links, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), rig.db, rig.repos.StudentGuardian.FindByStudentID, first.studentID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	testpkg.CreateTestStudentGuardianLink(t, rig.db, second.ID, links[0].GuardianProfileID, securityruntime.GuardianRolePrimaryGuardian)
	hold := testGuardianHold(rig.db, rig.repos)
	counter := testpkg.CaptureQueriesForContext(t, rig.db)
	counts := make([]int, 0, 2)
	for _, ids := range [][]int64{{first.studentID}, {second.ID, first.studentID}} {
		require.NoError(t, testpkg.WithTenantTx(t, counter.Context(testpkg.Ctx(t)), rig.db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
			counter.Reset()
			counter.Start()
			defer counter.Stop()
			grants, err := hold.HoldAccountStudentsPermission(ctx, first.accountID, ids, testpkg.Tenant(t), securityruntime.GuardianPermissionEnrollmentSubmit)
			require.NoError(t, err)
			for _, id := range ids {
				require.True(t, grants[id])
			}
			counts = append(counts, counter.Total())
			return nil
		}))
	}
	require.Positive(t, counts[0])
	require.Equal(t, counts[0], counts[1], "profile, membership, children and permissions must use a constant query count")
	t.Logf("held authorization: %d statements for one child and for two siblings", counts[0])
}
