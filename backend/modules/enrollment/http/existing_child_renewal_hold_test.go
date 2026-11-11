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

// checkedProbe parks after the unlocked relationship check that pins the
// child, the check before the write.
type checkedProbe struct {
	enrollmentCompose.GuardianStudentAuthorizer
	gate *renewalGate
}

func (p checkedProbe) AccountHasStudentPermission(ctx context.Context, accountID, studentID, tenantID int64, permission string) (bool, error) {
	granted, err := p.GuardianStudentAuthorizer.AccountHasStudentPermission(ctx, accountID, studentID, tenantID, permission)
	p.gate.park(ctx)
	return granted, err
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

type renewalRig struct {
	*existingChildEnv
	svc              enrollmentAPI.RequestService
	checked, writing *renewalGate
}

func setupRenewalRig(t *testing.T) *renewalRig {
	t.Helper()
	return setupRenewalRigFor(t, enrollmentNative.PhaseAudienceOpen)
}

func setupRenewalRigFor(t *testing.T, audience string) *renewalRig {
	t.Helper()
	env := setupExistingChildTest(t, audience)
	rig := &renewalRig{existingChildEnv: env, checked: newRenewalGate(), writing: newRenewalGate()}
	t.Cleanup(func() { rig.checked.open(); rig.writing.open() })
	rig.svc = newTestRequestService(owners.EnrollmentIntakeSources{
		Bookings: requestTestBookingCommands(), Requests: env.repos.Enrollment(),
		Children:  writingChildren{IntakeChildren: env.repos.Enrollment(), gate: rig.writing},
		Guardians: env.repos.Enrollment(), CareOfferingRepo: enrollmentCompose.NewCareOfferingRecords(env.reposCare.CarePlan),
		Catalog: env.repos.Enrollment(), SchoolRepo: factorySchools{repos: env.reposSchools}, RateLimitRepo: env.repos.Enrollment(),
		LateInviteRepo: env.repos.Enrollment(), StudentRepo: env.repos.Student,
		GuardianAuthorizer: checkedProbe{GuardianStudentAuthorizer: env.repos.StudentGuardian, gate: rig.checked},
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

// A withdrawal committed after the check before the write refuses the
// renewal; nothing of it is stored.
func TestParentRenewalRefusesAfterWithdrawalBeforeTheGuard(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, change := range renewalWithdrawals {
		t.Run(change.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			rig := setupRenewalRig(t)
			f := rig.family(t, "Vorher")
			apply := change.run(t, rig, f)

			rig.checked.armed.Store(true)
			done := rig.submitAsync(t, f)
			select {
			case backend := <-rig.checked.reached:
				require.NoError(t, backend.err)
			case err := <-done:
				t.Fatalf("the renewal answered before its relationship check: %v", err)
			case <-time.After(renewalDeadline):
				t.Fatal("the renewal never checked the relationship")
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
