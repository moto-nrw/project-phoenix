package enrollmenthttp_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Issue #6638 (rest after #6667): staff paths that reuse a parent's pin ask
// the parent's authority again, the approval (Decide) and the change-request
// approval for a pinned and for an approved child. They hold that authority
// behind the parent guard until commit: a withdrawal committed before the
// guard refuses the parent's part, one that starts while the staff
// transaction holds the guard waits for its commit. The tests synchronize on
// channels and pg_blocking_pids, without sleeps.

// reviewedChangeRequests parks once, when armed, at the first request row
// lock of the review (before the guard) or before the review is recorded
// (after every check and write of the approval).
type reviewedChangeRequests struct {
	enrollmentCompose.ChangeRequestRecords
	locked, reviewed *renewalGate
}

func (r reviewedChangeRequests) RequestByID(ctx context.Context, id int64, forUpdate bool) (*enrollmentNative.Request, error) {
	if forUpdate {
		r.locked.park(ctx)
	}
	return r.ChangeRequestRecords.RequestByID(ctx, id, forUpdate)
}

func (r reviewedChangeRequests) MarkChangeRequestReviewed(ctx context.Context, id int64, status string, note *string, actor int64, at time.Time) error {
	r.reviewed.park(ctx)
	return r.ChangeRequestRecords.MarkChangeRequestReviewed(ctx, id, status, note, actor, at)
}

// pinGuardRig is a renewal rig with a parent's pinned submission.
type pinGuardRig struct {
	*renewalRig
	f         renewalFamily
	submitted *enrollmentAPI.SubmitResult
}

func setupPinGuardRig(t *testing.T, last string) *pinGuardRig {
	t.Helper()
	rig := setupRenewalRig(t)
	f := rig.family(t, last)
	submitted := rig.submit(t, existingChildSubmission{
		accountID: &f.accountID, email: f.email,
		children: []enrollmentAPI.SubmitChild{submitChild(f.first, f.last, existingChildBirthday)},
	})
	require.Equal(t, &f.studentID, submitted.Children[0].MatchedStudentID, "the parent's submission pins the enrolled child")
	return &pinGuardRig{renewalRig: rig, f: f, submitted: submitted}
}

func (r *pinGuardRig) approval() enrollmentAPI.DecideInput {
	return enrollmentAPI.DecideInput{
		RequestID: r.submitted.Request.ID, ChildID: r.submitted.Children[0].ID,
		Status: enrollmentNative.DecisionApproved, ReviewedBy: r.creatorID,
	}
}

// changeRequest creates the family's change of the target grade with the
// gates the test arms around the approval.
func (r *pinGuardRig) changeRequest(t *testing.T, companions ...enrollmentCompose.CompanionGraphCoordinator) (enrollmentAPI.ChangeRequestService, int64, *reviewedChangeRequests) {
	t.Helper()
	records := &reviewedChangeRequests{ChangeRequestRecords: r.repos.Enrollment(), locked: newRenewalGate(), reviewed: newRenewalGate()}
	t.Cleanup(func() { records.locked.open(); records.reviewed.open() })
	svc := newChangeRequestServiceWithDecisionAndIntakeForTest(t, r.decisionTestEnv, records, companions...)
	grade := int16(3)
	child := submitChild(r.f.first, r.f.last, existingChildBirthday)
	child.ID, child.TargetGradeLevel = r.submitted.Children[0].ID, &grade
	restore := liftTakeoverStamp(t, r.decisionTestEnv, r.submitted.Request.ID)
	created, err := svc.Create(testpkg.Ctx(t), r.submitted.Request.StatusToken, enrollmentAPI.CreateChangeRequestInput{
		Submission: enrollmentAPI.SubmitRequest{
			GuardianFirstName: r.submitted.Request.GuardianFirstName, GuardianLastName: r.submitted.Request.GuardianLastName,
			GuardianEmail: r.f.email, GuardianPhone: r.submitted.Request.GuardianPhone,
			ConsentFlags: testFormValues(rolloverConsentFlags()),
			Children:     []enrollmentAPI.SubmitChild{child},
		},
		ParentNote: "Bitte Klassenstufe ändern.",
	})
	restore()
	require.NoError(t, err)
	return svc, created.ChangeRequest.ID, records
}

func (r *pinGuardRig) approveChangeAsync(t *testing.T, svc enrollmentAPI.ChangeRequestService, id int64) <-chan error {
	t.Helper()
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() {
		_, err := testpkg.TenantQuery2(t, ctx, r.db, svc.Approve, id, enrollmentAPI.ReviewChangeRequestInput{
			ActorAccountID: r.creatorID, ActorRole: "admin", Note: "Klassenstufe freigegeben.",
		})
		done <- err
	}()
	return done
}

// approvedWithCompanions approves the pinned child and returns the companion
// graph the change-request approval locks for it.
func (r *pinGuardRig) approvedWithCompanions(t *testing.T) approvalCompanionGraph {
	t.Helper()
	outcome := r.decideOutcome(t, r.approval())
	require.Equal(t, &r.f.studentID, outcome.Child.CreatedStudentID)
	people := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, r.db))
	return approvalCompanionGraph{
		CompanionLocks: api.MustNewStudentCompanions(r.reposCare.CarePlan, r.repos.Student, people, nil),
		verify:         r.repos.Student.VerifyCompanionStrandingBatch,
	}
}

// staffRace parks a staff transaction right after its authority check: a
// blocker holds a row the staff transaction needs only after the check. A
// withdrawal then either waits for the staff transaction (the guard holds)
// or commits first, in which case the staff transaction must refuse the
// parent's part once the blocker lets it go. Without the guard the
// withdrawal commits and the staff transaction still writes on the parent's
// withdrawn authority.
type staffRace struct {
	// block takes the lock the staff transaction needs after its check.
	block func(ctx context.Context, tx bun.Tx) error
	// staff starts the staff transaction.
	staff func() <-chan error
	// refused is the staff transaction's refusal after a withdrawal.
	refused error
}

func (race staffRace) run(t *testing.T, db *bun.DB, apply func() error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
	defer cancel()
	blocker := newRenewalGate()
	blocker.armed.Store(true)
	t.Cleanup(blocker.open)
	blocked := make(chan error, 1)
	go func() {
		blocked <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			if err := race.block(txCtx, tx); err != nil {
				return err
			}
			blocker.park(txCtx)
			return nil
		})
	}()
	holder := awaitHolder(t, blocker.reached, blocked, "the blocker")

	done := race.staff()
	staffPID := awaitWaiter(t, db, holder.pid, done, "the staff transaction must wait for the blocker after its check")

	withdrawn := make(chan error, 1)
	go func() { withdrawn <- apply() }()
	withdrawnFirst := false
	var withdrawErr error
	require.Eventually(t, func() bool {
		select {
		case withdrawErr = <-withdrawn:
			withdrawnFirst = true
			return true
		default:
		}
		return waitsFor(db, staffPID)
	}, 10*time.Second, 10*time.Millisecond, "the withdrawal either commits or waits for the staff transaction")

	blocker.open()
	require.NoError(t, awaitRenewal(t, blocked))
	staffErr := awaitRenewal(t, done)
	if withdrawnFirst {
		require.NoError(t, withdrawErr)
		require.ErrorIs(t, staffErr, race.refused,
			"the withdrawal committed while the staff transaction waited after its check; it must not write on the withdrawn authority")
		return
	}
	require.NoError(t, staffErr, "the staff transaction that holds the guard commits first")
	require.NoError(t, awaitRenewal(t, withdrawn), "the withdrawal finishes after the staff transaction")
}

// waitsFor reports whether some backend waits for a lock pid holds.
func waitsFor(db *bun.DB, pid int) bool {
	var waiting bool
	err := db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", pid).
		Scan(context.Background(), &waiting)
	return err == nil && waiting
}

// awaitWaiter returns the backend that waits for holderPID.
func awaitWaiter(t *testing.T, db *bun.DB, holderPID int, done <-chan error, msg string) int {
	t.Helper()
	var pid int
	var ended bool
	var endErr error
	require.Eventually(t, func() bool {
		select {
		case endErr = <-done:
			ended = true
			return true
		default:
		}
		err := db.NewRaw("SELECT l.pid FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)) LIMIT 1", holderPID).
			Scan(context.Background(), &pid)
		return err == nil && pid > 0
	}, 10*time.Second, 10*time.Millisecond, msg)
	require.False(t, ended, "%s: it ended without waiting: %v", msg, endErr)
	return pid
}

func awaitHolder(t *testing.T, reached <-chan renewalBackend, done <-chan error, what string) renewalBackend {
	t.Helper()
	select {
	case holder := <-reached:
		require.NoError(t, holder.err)
		return holder
	case err := <-done:
		t.Fatalf("%s ended before it was parked: %v", what, err)
	case <-time.After(renewalDeadline):
		t.Fatalf("%s was never parked", what)
	}
	return renewalBackend{}
}

// lockStudent holds the student row, which an approval of a parent's pin
// locks only when it renews the student, after its authority check.
func lockStudent(rig *pinGuardRig) func(context.Context, bun.Tx) error {
	return func(ctx context.Context, _ bun.Tx) error {
		_, err := rig.repos.Student.FindByIDForUpdate(ctx, rig.f.studentID)
		return err
	}
}

// parkingOutbox parks the approval at its parent mail, after every check
// and write and before the commit.
type parkingOutbox struct {
	testMailOutbox
	gate *renewalGate
}

func (o *parkingOutbox) EnqueueMail(ctx context.Context, mail TestMail) error {
	o.gate.park(ctx)
	return o.testMailOutbox.EnqueueMail(ctx, mail)
}

// lockMatch holds the phase's existing-student match lock, which the
// change-request approval of a pin takes after its authority check.
func lockMatch(rig *pinGuardRig) func(context.Context, bun.Tx) error {
	return func(ctx context.Context, _ bun.Tx) error {
		return rig.repos.Enrollment().AcquireExistingStudentMatchLock(ctx, rig.sourcePhase.ID)
	}
}

// lockPerson holds the approved student's person row, which the sync of a
// family's change writes after its authority check.
func lockPerson(rig *pinGuardRig) func(context.Context, bun.Tx) error {
	return func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewRaw(`SELECT p.id FROM users.persons AS p
			JOIN users.student_profiles AS sp ON sp.person_id = p.id
			WHERE sp.id = ? FOR UPDATE OF p`, rig.f.studentID).Exec(ctx)
		return err
	}
}

func (r *pinGuardRig) decideAsync(t *testing.T, decision enrollmentAPI.DecisionService) <-chan error {
	t.Helper()
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, ctx, r.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			_, err := decision.Decide(txCtx, r.approval())
			return err
		})
	}()
	return done
}

// underReview moves the pinned child out of submitted, so the family's edit
// becomes a change request.
func (r *pinGuardRig) underReview(t *testing.T) {
	t.Helper()
	input := r.approval()
	input.Status = enrollmentNative.DecisionUnderReview
	require.NoError(t, r.decide(t, input))
}

// The office's approval of a parent's pin holds the parent's authority
// until it commits.
func TestParentPinApprovalHoldsTheGuardUntilCommit(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, change := range renewalWithdrawals {
		t.Run(change.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			t.Run("im Guard", func(t *testing.T) {
				testpkg.OwnTenant(t)
				rig := setupPinGuardRig(t, "Freigabe")
				apply := change.run(t, rig.renewalRig, rig.f)
				staffRace{
					block:   lockStudent(rig),
					staff:   func() <-chan error { return rig.decideAsync(t, rig.decision) },
					refused: enrollmentNative.ErrChildPossiblyEnrolled,
				}.run(t, rig.db, apply)
			})
			t.Run("bis zum Commit", func(t *testing.T) {
				testpkg.OwnTenant(t)
				rig := setupPinGuardRig(t, "Freigabe")
				apply := change.run(t, rig.renewalRig, rig.f)
				outbox := &parkingOutbox{testMailOutbox: rig.outbox, gate: newRenewalGate()}
				outbox.gate.armed.Store(true)
				t.Cleanup(outbox.gate.open)
				decision := newDecisionServiceForTestWithDependencies(rig.rolloverTestEnv, nil, nil, nil, nil, outbox)
				done := rig.decideAsync(t, decision)
				holder := awaitHolder(t, outbox.gate.reached, done, "the approval")
				withdrawn := make(chan error, 1)
				go func() { withdrawn <- apply() }()
				require.Eventually(t, func() bool { return waitsFor(rig.db, holder.pid) }, 10*time.Second, 10*time.Millisecond,
					"the withdrawal must wait for the approval that holds the guard")
				select {
				case err := <-withdrawn:
					t.Fatalf("the withdrawal committed while the approval held the guard: %v", err)
				default:
				}
				outbox.gate.open()
				require.NoError(t, awaitRenewal(t, done), "the approval commits first")
				require.NoError(t, awaitRenewal(t, withdrawn), "the withdrawal finishes after the approval")
			})
		})
	}
}

// A withdrawal committed before the approval's guard leaves the pin without
// authority: the approval asks the office instead of renewing on the
// parent's behalf.
func TestParentPinApprovalRefusesAfterWithdrawalBeforeTheGuard(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, change := range renewalWithdrawals {
		t.Run(change.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			rig := setupPinGuardRig(t, "Vorab")
			apply := change.run(t, rig.renewalRig, rig.f)
			gate := newRenewalGate()
			t.Cleanup(gate.open)
			decision := newDecisionServiceForTest(rig.rolloverTestEnv, nil, func(ctx context.Context) error {
				gate.park(ctx)
				return nil
			})
			gate.armed.Store(true)
			ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- testpkg.WithTenantTx(t, ctx, rig.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
					_, err := decision.Decide(txCtx, rig.approval())
					return err
				})
			}()
			awaitHolder(t, gate.reached, done, "the approval")
			require.NoError(t, apply(), "the withdrawal commits while the approval waits before its guard")
			gate.open()

			require.ErrorIs(t, awaitRenewal(t, done), enrollmentNative.ErrChildPossiblyEnrolled,
				"the approval asks the office after %s", change.name)
			child := rig.child(t, rig.submitted.Children[0].ID)
			require.Equal(t, enrollmentNative.ChildStatusSubmitted, child.Status)
			require.Nil(t, child.CreatedStudentID, "a refused approval renews nothing")
		})
	}
}

// The change-request approval of a pinned, undecided child re-checks the
// pin and holds the parent's authority until it commits.
func TestChangeRequestPinApprovalHoldsTheGuardUntilCommit(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, change := range renewalWithdrawals {
		t.Run(change.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			rig := setupPinGuardRig(t, "Antrag")
			rig.underReview(t)
			svc, id, _ := rig.changeRequest(t)
			apply := change.run(t, rig.renewalRig, rig.f)
			staffRace{
				block:   lockMatch(rig),
				staff:   func() <-chan error { return rig.approveChangeAsync(t, svc, id) },
				refused: enrollmentNative.ErrChildEnrollmentNotPermitted,
			}.run(t, rig.db, apply)
		})
	}
}

// A withdrawal committed before the change-request approval's guard refuses
// the pin; nothing of the change is stored.
func TestChangeRequestPinApprovalRefusesAfterWithdrawalBeforeTheGuard(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, change := range renewalWithdrawals {
		t.Run(change.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			rig := setupPinGuardRig(t, "AntragVorab")
			rig.underReview(t)
			svc, id, records := rig.changeRequest(t)
			apply := change.run(t, rig.renewalRig, rig.f)

			records.locked.armed.Store(true)
			done := rig.approveChangeAsync(t, svc, id)
			awaitHolder(t, records.locked.reached, done, "the change-request approval")
			require.NoError(t, apply(), "the withdrawal commits while the approval waits before its guard")
			records.locked.open()

			require.ErrorIs(t, awaitRenewal(t, done), enrollmentNative.ErrChildEnrollmentNotPermitted)
			child := rig.child(t, rig.submitted.Children[0].ID)
			require.Equal(t, int16(2), *child.TargetGradeLevel, "a refused approval changes nothing")
		})
	}
}

// The change-request approval of an approved child carries the family's
// data onto the student only while the parent holds the authority, and
// holds it until it commits.
func TestApprovedChildChangeHoldsTheGuardUntilCommit(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, change := range renewalWithdrawals {
		t.Run(change.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			rig := setupPinGuardRig(t, "Bestaetigt")
			graph := rig.approvedWithCompanions(t)
			svc, id, _ := rig.changeRequest(t, graph)
			apply := change.run(t, rig.renewalRig, rig.f)
			staffRace{
				block:   lockPerson(rig),
				staff:   func() <-chan error { return rig.approveChangeAsync(t, svc, id) },
				refused: enrollmentNative.ErrChangeRequestParentNotPermitted,
			}.run(t, rig.db, apply)
		})
	}
}

// A withdrawal committed before the guard refuses the family's new data for
// the approved child.
func TestApprovedChildChangeRefusesAfterWithdrawalBeforeTheGuard(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, change := range renewalWithdrawals {
		t.Run(change.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			rig := setupPinGuardRig(t, "BestaetigtVorab")
			graph := rig.approvedWithCompanions(t)
			svc, id, records := rig.changeRequest(t, graph)
			apply := change.run(t, rig.renewalRig, rig.f)

			records.locked.armed.Store(true)
			done := rig.approveChangeAsync(t, svc, id)
			awaitHolder(t, records.locked.reached, done, "the change-request approval")
			require.NoError(t, apply(), "the withdrawal commits while the approval waits before its guard")
			records.locked.open()

			require.ErrorIs(t, awaitRenewal(t, done), enrollmentNative.ErrChangeRequestParentNotPermitted)
			child := rig.child(t, rig.submitted.Children[0].ID)
			require.Equal(t, int16(2), *child.TargetGradeLevel, "a refused approval changes nothing")
		})
	}
}

// The change-request approval takes the companion graph of its approved
// children inside the guard's children slot, in one ascending pass with
// the held students. A companion edit locks the pair in ascending order as
// well; had the guard held the child before its lower-numbered companion,
// both would wait for each other.
func TestChangeRequestApprovalTakesTheCompanionGraphInTheGuard(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	renewal := setupRenewalRig(t)
	companion := enrolledChild(t, renewal.db, "Mila", "Begleitung", calendar.NewDate(2018, 3, 4))
	f := renewal.family(t, "Begleitet")
	require.Less(t, companion.ID, f.studentID, "the companion must lock before the child")
	submitted := renewal.submit(t, existingChildSubmission{
		accountID: &f.accountID, email: f.email,
		children: []enrollmentAPI.SubmitChild{submitChild(f.first, f.last, existingChildBirthday)},
	})
	rig := &pinGuardRig{renewalRig: renewal, f: f, submitted: submitted}
	graph := rig.approvedWithCompanions(t)
	_, err := rig.db.NewRaw(`INSERT INTO users.student_companions (tenant_id, student_low_id, student_high_id, weekday) VALUES (?, ?, ?, 2)`,
		testpkg.Tenant(t), companion.ID, f.studentID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	svc, id, _ := rig.changeRequest(t, graph)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
	defer cancel()
	editor := newRenewalGate()
	editor.armed.Store(true)
	t.Cleanup(editor.open)
	edited := make(chan error, 1)
	go func() {
		edited <- testpkg.WithTenantTx(t, ctx, rig.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			if _, err := rig.repos.Student.FindByIDForUpdate(txCtx, companion.ID); err != nil {
				return err
			}
			editor.park(txCtx)
			_, err := rig.repos.Student.FindByIDForUpdate(txCtx, f.studentID)
			return err
		})
	}()
	holder := awaitHolder(t, editor.reached, edited, "the companion edit")
	done := rig.approveChangeAsync(t, svc, id)
	awaitWaiter(t, rig.db, holder.pid, done, "the approval must wait for the companion the edit holds")
	editor.open()
	require.NoError(t, awaitRenewal(t, edited), "the companion edit takes the child and commits")
	require.NoError(t, awaitRenewal(t, done), "the approval commits after the companion edit")
}
