package enrollmenthttp_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStaffSubmissionWaitsForRenewalBeforeThePhaseLocks(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	rig := setupRenewalRigFor(t, enrollmentNative.PhaseAudienceExistingStudents)
	f := rig.family(t, "StaffSubmission")

	rig.dedup.armed.Store(true)
	renewal := rig.submitAsync(t, f)
	holder := awaitRenewalGate(t, rig.dedup, renewal)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
	defer cancel()
	staff := make(chan error, 1)
	go func() {
		_, err := rig.matching.Submit(ctx, enrollmentAPI.SubmitRequest{
			TenantID: testpkg.Tenant(t), PhaseID: rig.sourcePhase.ID,
			GuardianFirstName: "Eva", GuardianLastName: f.last, GuardianEmail: "staff-renewal@example.test",
			SubmissionSource:  enrollmentNative.RequestSourceAdminManual,
			GuardianAccountID: &f.accountID, GuardianSubmitEligible: true,
			ConsentFlags: testFormValues(rolloverConsentFlags()),
			Children:     []enrollmentAPI.SubmitChild{submitChild(f.first, f.last, existingChildBirthday)},
		})
		staff <- err
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := rig.db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", holder.pid).
			Scan(ctx, &waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond, "the staff submission must wait for the renewal's child lock")
	rig.dedup.open()
	require.NoError(t, awaitRenewal(t, renewal), "the renewal commits without waiting for a phase lock held by staff")
	require.ErrorIs(t, awaitRenewal(t, staff), enrollmentNative.ErrExistingStudentAlreadyRequested,
		"staff sees the committed renewal rather than a deadlock")
	require.Equal(t, 1, rig.requestsOf(t, f))
}

// Exercise the real approval paths while a renewal holds the student but
// has not yet taken the phase locks. The approval must wait without holding
// a lock the renewal needs to finish.
func TestOfficeChoiceApprovalWaitsForRenewalBeforeTheMatchLock(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	rig := setupRenewalRig(t)
	f := rig.family(t, "OfficeChoice")
	submitted := rig.submit(t, existingChildSubmission{
		email:    "office-choice@example.test",
		children: []enrollmentAPI.SubmitChild{submitChild(f.first, f.last, existingChildBirthday)},
	})
	require.Nil(t, submitted.Children[0].MatchedStudentID)

	rig.dedup.armed.Store(true)
	renewal := rig.submitAsync(t, f)
	holder := awaitRenewalGate(t, rig.dedup, renewal)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
	defer cancel()
	approval := make(chan error, 1)
	go func() {
		approval <- testpkg.WithTenantTx(t, ctx, rig.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			_, err := rig.decision.Decide(txCtx, enrollmentAPI.DecideInput{
				RequestID: submitted.Request.ID, ChildID: submitted.Children[0].ID,
				Status: enrollmentNative.DecisionApproved, ReviewedBy: rig.creatorID, ExistingStudentID: &f.studentID,
			})
			return err
		})
	}()
	awaitApprovalBlockedByRenewal(t, rig.db, ctx, holder.pid, approval)
	rig.dedup.open()
	require.NoError(t, awaitRenewal(t, renewal), "renewal must commit without a deadlock")
	require.ErrorIs(t, awaitRenewal(t, approval), enrollmentNative.ErrExistingStudentAlreadyRequested,
		"the office choice must see the committed renewal and refuse the duplicate")
	child := rig.child(t, submitted.Children[0].ID)
	require.Equal(t, enrollmentNative.ChildStatusSubmitted, child.Status)
	require.Nil(t, child.MatchedStudentID)
}

type approvalCompanionGraph struct {
	careplan.CompanionLocks
	verify func(context.Context) error
}

func (c approvalCompanionGraph) VerifyCompanionStrandingBatch(ctx context.Context) error {
	return c.verify(ctx)
}

func TestChangeRequestApprovalWaitsForRenewalBeforeTheDedupLock(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	rig := setupRenewalRig(t)
	f := rig.family(t, "ChangeApproval")
	submitted := rig.submit(t, existingChildSubmission{
		accountID: &f.accountID, email: f.email,
		children: []enrollmentAPI.SubmitChild{submitChild(f.first, f.last, existingChildBirthday)},
	})
	outcome := rig.decideOutcome(t, enrollmentAPI.DecideInput{RequestID: submitted.Request.ID, ChildID: submitted.Children[0].ID})
	require.Equal(t, &f.studentID, outcome.Child.CreatedStudentID)
	people := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, rig.db))
	graph := approvalCompanionGraph{
		CompanionLocks: api.MustNewStudentCompanions(rig.reposCare.CarePlan, rig.repos.Student, people, nil),
		verify:         rig.repos.Student.VerifyCompanionStrandingBatch,
	}
	svc := newChangeRequestServiceWithDecisionAndIntakeForTest(t, rig.decisionTestEnv, rig.repos.Enrollment(), graph)
	grade := int16(3)
	child := submitChild(f.first, f.last, existingChildBirthday)
	child.ID, child.TargetGradeLevel = submitted.Children[0].ID, &grade
	restore := liftTakeoverStamp(t, rig.decisionTestEnv, submitted.Request.ID)
	created, err := svc.Create(testpkg.Ctx(t), submitted.Request.StatusToken, enrollmentAPI.CreateChangeRequestInput{
		Submission: enrollmentAPI.SubmitRequest{
			GuardianFirstName: submitted.Request.GuardianFirstName, GuardianLastName: submitted.Request.GuardianLastName,
			GuardianEmail: f.email, GuardianPhone: submitted.Request.GuardianPhone,
			ConsentFlags: testFormValues(rolloverConsentFlags()),
			Children:     []enrollmentAPI.SubmitChild{child},
		},
		ParentNote: "Bitte Klassenstufe ändern.",
	})
	restore()
	require.NoError(t, err)
	rig.settings.stringValues[settingKeyDuplicateHandling] = settingValueDuplicateBlock

	rig.dedup.armed.Store(true)
	renewal := rig.submitAsync(t, f)
	holder := awaitRenewalGate(t, rig.dedup, renewal)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), renewalDeadline)
	defer cancel()
	approval := make(chan error, 1)
	go func() {
		_, err := testpkg.TenantQuery2(t, ctx, rig.db, svc.Approve, created.ChangeRequest.ID, enrollmentAPI.ReviewChangeRequestInput{
			ActorAccountID: rig.creatorID, ActorRole: "admin", Note: "Klassenstufe freigegeben.",
		})
		approval <- err
	}()
	awaitApprovalBlockedByRenewal(t, rig.db, ctx, holder.pid, approval)
	rig.dedup.open()
	require.ErrorIs(t, awaitRenewal(t, renewal), enrollmentNative.ErrDuplicateEnrollment,
		"the renewal must reach its duplicate check without a deadlock")
	require.NoError(t, awaitRenewal(t, approval), "the change request must commit after the renewal rolls back")
	stored := rig.child(t, child.ID)
	require.Equal(t, &grade, stored.TargetGradeLevel)
	require.Equal(t, enrollmentNative.ChildStatusApproved, stored.Status)
	require.Equal(t, 1, rig.requestsOf(t, f))
}

func awaitRenewalGate(t *testing.T, gate *renewalGate, done <-chan error) renewalBackend {
	t.Helper()
	select {
	case holder := <-gate.reached:
		require.NoError(t, holder.err)
		return holder
	case err := <-done:
		t.Fatalf("renewal ended before reaching the phase locks: %v", err)
	case <-time.After(renewalDeadline):
		t.Fatal("renewal never reached the phase locks")
	}
	return renewalBackend{}
}

func awaitApprovalBlockedByRenewal(t *testing.T, db *bun.DB, ctx context.Context, holderPID int, approval <-chan error) {
	t.Helper()
	var ended bool
	var approvalErr error
	require.Eventually(t, func() bool {
		select {
		case approvalErr = <-approval:
			ended = true
			return true
		default:
		}
		var waiting bool
		err := db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", holderPID).
			Scan(ctx, &waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond, "approval must wait for the student held by the renewal")
	require.False(t, ended, "approval ended before waiting for the student: %v", approvalErr)
}
