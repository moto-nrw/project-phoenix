package enrollment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// stubEligibilityStudentRepo implements only the method
// validatePhaseEligibility needs; the embedded interface panics on any
// other call, which would signal an unintended dependency.
type stubEligibilityStudentRepo struct {
	users.StudentRepository
	exists    bool
	err       error
	gotTenant int64
}

func (s *stubEligibilityStudentRepo) ExistsEnrolledByNameAndBirthday(_ context.Context, tenantID int64, _, _ string, _ timezone.Date) (bool, error) {
	s.gotTenant = tenantID
	return s.exists, s.err
}

// stubMatchStudentRepo backs resolveMatchedStudentID: it implements the
// unambiguous single-match lookup (matchID) plus the existence probe (exists)
// the resolver falls back to when the lookup is nil, to distinguish a genuine
// zero match from an ambiguous multi-match.
type stubMatchStudentRepo struct {
	users.StudentRepository
	matchID *int64
	exists  bool
	err     error
}

func (s *stubMatchStudentRepo) FindEnrolledStudentIDByNameAndBirthday(_ context.Context, _ int64, _, _ string, _ timezone.Date) (*int64, error) {
	return s.matchID, s.err
}

func (s *stubMatchStudentRepo) ExistsEnrolledByNameAndBirthday(_ context.Context, _ int64, _, _ string, _ timezone.Date) (bool, error) {
	return s.exists, s.err
}

func int64PtrEligibility(v int64) *int64 { return &v }

// An ambiguous match (the unambiguous lookup returns nil but a match still
// exists) must be rejected rather than silently taking the fresh-create path,
// which would add a third duplicate to the colliding records (#1663).
func TestResolveMatchedStudentID_AmbiguousRejected(t *testing.T) {
	repo := &stubMatchStudentRepo{matchID: nil, exists: true}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{StudentRepo: repo}}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceExistingStudents)

	id, err := svc.resolveMatchedStudentID(context.Background(), int64(7001), phase, 2,
		SubmitChild{FirstName: "Kim", LastName: "Test", DateOfBirth: timezone.NewDate(2019, 4, 12)})
	require.ErrorIs(t, err, ErrChildEnrollmentAmbiguous)
	require.ErrorIs(t, err, ErrInvalidSubmission, "ambiguous error must keep the 400 category")
	assert.Nil(t, id)
}

// A genuine zero match (no enrolled student) stays nil so the fresh-create path
// runs — legitimate for admin-manual / late-invite which bypass the gate.
func TestResolveMatchedStudentID_ZeroMatchAllowsFreshCreate(t *testing.T) {
	repo := &stubMatchStudentRepo{matchID: nil, exists: false}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{StudentRepo: repo}}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceExistingStudents)

	id, err := svc.resolveMatchedStudentID(context.Background(), int64(7002), phase, 0,
		SubmitChild{FirstName: "Kim", LastName: "Test", DateOfBirth: timezone.NewDate(2019, 4, 12)})
	require.NoError(t, err)
	assert.Nil(t, id)
}

// An unambiguous single match pins the concrete student so approval renews it.
func TestResolveMatchedStudentID_SingleMatchReturnsID(t *testing.T) {
	want := int64PtrEligibility(555)
	repo := &stubMatchStudentRepo{matchID: want}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{StudentRepo: repo}}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceExistingStudents)

	id, err := svc.resolveMatchedStudentID(context.Background(), int64(7003), phase, 0,
		SubmitChild{FirstName: "Kim", LastName: "Test", DateOfBirth: timezone.NewDate(2019, 4, 12)})
	require.NoError(t, err)
	require.NotNil(t, id)
	assert.Equal(t, int64(555), *id)
}

// Non-existing_students audiences never resolve a match (and never probe the
// repo), so the child always takes the fresh-create path.
func TestResolveMatchedStudentID_NonExistingAudienceSkips(t *testing.T) {
	repo := &stubMatchStudentRepo{matchID: int64PtrEligibility(1), exists: true}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{StudentRepo: repo}}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceOpen)

	id, err := svc.resolveMatchedStudentID(context.Background(), int64(7004), phase, 0,
		SubmitChild{FirstName: "Kim", LastName: "Test", DateOfBirth: timezone.NewDate(2019, 4, 12)})
	require.NoError(t, err)
	assert.Nil(t, id)
}

// The enrolled-student gate ran before the write tx and proved the child is
// enrolled; an unpinned match afterwards means the scheduler flipped that
// student's status underneath the submission. Approving it would create a
// duplicate Person/Student, so the submission must be rejected instead (#1663).
func TestAssertExistingStudentMatchResolved_RejectsRacedZeroMatch(t *testing.T) {
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceExistingStudents)

	err := assertExistingStudentMatchResolved(phase, nil, true, 1)
	require.ErrorIs(t, err, ErrChildNotEnrolled)
	require.ErrorIs(t, err, ErrInvalidSubmission, "must keep the 400 category")
}

// Admin-manual / late-invite submissions skip the enrolled gate on purpose and
// may create a fresh record, so an unpinned match stays legal for them.
func TestAssertExistingStudentMatchResolved_TrustedPathKeepsFreshCreate(t *testing.T) {
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceExistingStudents)

	require.NoError(t, assertExistingStudentMatchResolved(phase, nil, false, 0))
}

// A pinned match, and any non-existing_students audience, are unaffected.
func TestAssertExistingStudentMatchResolved_PinnedAndOtherAudiencesPass(t *testing.T) {
	existing := eligibilityTestPhase(enrollmentModels.PhaseAudienceExistingStudents)
	require.NoError(t, assertExistingStudentMatchResolved(existing, int64PtrEligibility(42), true, 0))

	open := eligibilityTestPhase(enrollmentModels.PhaseAudienceOpen)
	require.NoError(t, assertExistingStudentMatchResolved(open, nil, true, 0))
}

func eligibilityTestPhase(audience string, eligibleClasses ...string) *enrollmentModels.Phase {
	return &enrollmentModels.Phase{
		Audience:              audience,
		EligibleSchoolClasses: eligibleClasses,
	}
}

func strPtrEligibility(s string) *string { return &s }

func TestValidatePhaseEligibility_LinkedParentsRejectsAnonymous(t *testing.T) {
	svc := &requestService{}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceLinkedParents)

	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{})
	require.ErrorIs(t, err, ErrPhaseNotEligible)
}

func TestValidatePhaseEligibility_LinkedParentsRequiresSubmitEligibility(t *testing.T) {
	svc := &requestService{}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceLinkedParents)
	accountID := int64(4711)

	// Authenticated but without a permission-granting guardian link.
	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		GuardianAccountID: &accountID,
	})
	require.ErrorIs(t, err, ErrPhaseNotEligible)

	// Authenticated with a guardian link granting the submit permission.
	err = svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		GuardianAccountID:      &accountID,
		GuardianSubmitEligible: true,
	})
	require.NoError(t, err)
}

func TestValidatePhaseEligibility_TrustedPathsBypass(t *testing.T) {
	svc := &requestService{}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceLinkedParents, "2a")

	// Admin manual enrollment and late invites set AllowClosedPhase and
	// must bypass every eligibility rule.
	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		AllowClosedPhase: true,
		Children:         []SubmitChild{{FirstName: "Kim", LastName: "Test"}},
	})
	require.NoError(t, err)
}

func TestValidatePhaseEligibility_EligibleClassesEnforced(t *testing.T) {
	svc := &requestService{}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceOpen, "2a", " 3b ")

	// No class declared → rejected.
	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		Children: []SubmitChild{{FirstName: "Kim", LastName: "Test"}},
	})
	require.ErrorIs(t, err, ErrChildClassNotEligible)
	require.ErrorIs(t, err, ErrInvalidSubmission, "class error must keep the 400 category")

	// Wrong class → rejected.
	err = svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		Children: []SubmitChild{{FirstName: "Kim", LastName: "Test", TargetSchoolClass: strPtrEligibility("4c")}},
	})
	require.ErrorIs(t, err, ErrChildClassNotEligible)

	// Listed class (with surrounding whitespace in config) → accepted.
	err = svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		Children: []SubmitChild{{FirstName: "Kim", LastName: "Test", TargetSchoolClass: strPtrEligibility("3b")}},
	})
	require.NoError(t, err)
}

func TestValidatePhaseEligibility_EmptyClassListMeansNoRestriction(t *testing.T) {
	svc := &requestService{}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceOpen)

	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		Children: []SubmitChild{{FirstName: "Kim", LastName: "Test"}},
	})
	require.NoError(t, err)
}

func TestValidatePhaseEligibility_NewStudentsRejectsEnrolledChild(t *testing.T) {
	repo := &stubEligibilityStudentRepo{exists: true}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{StudentRepo: repo}}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceNewStudents)
	tenantID := int64(9042)

	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		TenantID: tenantID,
		Children: []SubmitChild{{FirstName: "Kim", LastName: "Test", DateOfBirth: timezone.NewDate(2019, 4, 12)}},
	})
	require.ErrorIs(t, err, ErrChildAlreadyEnrolled)
	assert.Equal(t, tenantID, repo.gotTenant, "lookup must be tenant-scoped explicitly")
}

func TestValidatePhaseEligibility_NewStudentsAcceptsUnknownChild(t *testing.T) {
	repo := &stubEligibilityStudentRepo{exists: false}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{StudentRepo: repo}}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceNewStudents)

	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		TenantID: int64(9043),
		Children: []SubmitChild{{FirstName: "Kim", LastName: "Test", DateOfBirth: timezone.NewDate(2019, 4, 12)}},
	})
	require.NoError(t, err)
}

// TestPhaseEligibility_ClassBypassClosedAfterCanonicalization proves the
// class-eligibility gate can no longer be bypassed by declaring an eligible
// class the form can never actually collect. Submit runs
// validateAndNormalizeSchoolClasses (which erases the class for grade 1 or
// when concrete-class collection is off) BEFORE validatePhaseEligibility, so
// the gate sees the persisted nil and rejects (#1663).
func TestPhaseEligibility_ClassBypassClosedAfterCanonicalization(t *testing.T) {
	phase := &enrollmentModels.Phase{
		Audience:               enrollmentModels.PhaseAudienceOpen,
		EligibleSchoolClasses:  []string{"2a"},
		AvailableSchoolClasses: []string{"2a"},
	}

	// Grade-1 child crafts an eligible "2a"; canonicalization drops it
	// because grade 1 never declares a concrete class.
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{Settings: schoolClassSettingsStub{collect: true}}}
	children := []SubmitChild{{FirstName: "Kim", LastName: "Test", TargetGradeLevel: grade(1), TargetSchoolClass: classPtr("2a")}}
	require.NoError(t, svc.validateAndNormalizeSchoolClasses(context.Background(), phase, children))
	require.Nil(t, children[0].TargetSchoolClass, "grade 1 must not keep a concrete class")
	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{Children: children})
	require.ErrorIs(t, err, ErrChildClassNotEligible, "canonicalized empty class must fail eligibility")

	// Concrete-class collection disabled tenant-wide: same erase-then-reject.
	svcOff := &requestService{RequestServiceConfig: RequestServiceConfig{Settings: schoolClassSettingsStub{collect: false}}}
	children2 := []SubmitChild{{FirstName: "Kim", LastName: "Test", TargetGradeLevel: grade(2), TargetSchoolClass: classPtr("2a")}}
	require.NoError(t, svcOff.validateAndNormalizeSchoolClasses(context.Background(), phase, children2))
	require.Nil(t, children2[0].TargetSchoolClass, "collection off must drop the class")
	err = svcOff.validatePhaseEligibility(context.Background(), phase, SubmitRequest{Children: children2})
	require.ErrorIs(t, err, ErrChildClassNotEligible, "collection-off must not bypass the class gate")
}

func TestValidatePhaseEligibility_ExistingStudentsRejectsUnknownChild(t *testing.T) {
	repo := &stubEligibilityStudentRepo{exists: false}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{StudentRepo: repo}}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceExistingStudents)
	tenantID := int64(9052)
	accountID := int64(4712)

	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		TenantID:               tenantID,
		GuardianAccountID:      &accountID,
		GuardianSubmitEligible: true,
		Children:               []SubmitChild{{FirstName: "Kim", LastName: "Test", DateOfBirth: timezone.NewDate(2019, 4, 12)}},
	})
	require.ErrorIs(t, err, ErrChildNotEnrolled)
	require.ErrorIs(t, err, ErrInvalidSubmission, "not-enrolled error must keep the 400 category")
	assert.Equal(t, tenantID, repo.gotTenant, "lookup must be tenant-scoped explicitly")
}

func TestValidatePhaseEligibility_ExistingStudentsAcceptsEnrolledChild(t *testing.T) {
	repo := &stubEligibilityStudentRepo{exists: true}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{StudentRepo: repo}}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceExistingStudents)
	accountID := int64(4713)

	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		TenantID:               int64(9053),
		GuardianAccountID:      &accountID,
		GuardianSubmitEligible: true,
		Children:               []SubmitChild{{FirstName: "Kim", LastName: "Test", DateOfBirth: timezone.NewDate(2019, 4, 12)}},
	})
	require.NoError(t, err)
}

// An existing_students phase never CREATES a record — it renews a live student
// and attaches the submitter as its guardian — so name+birthday alone must not
// admit a submission. It carries the same authentication gate as
// linked_parents; account-less parents come in through a late invite or admin
// manual enrollment, both of which set AllowClosedPhase (#1663).
func TestValidatePhaseEligibility_ExistingStudentsRejectsAnonymous(t *testing.T) {
	repo := &stubEligibilityStudentRepo{exists: true}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{StudentRepo: repo}}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceExistingStudents)
	accountID := int64(4714)
	child := []SubmitChild{{FirstName: "Kim", LastName: "Test", DateOfBirth: timezone.NewDate(2019, 4, 12)}}

	// Anonymous: no guardian account at all.
	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		TenantID: int64(9054),
		Children: child,
	})
	require.ErrorIs(t, err, ErrPhaseNotEligible)

	// Authenticated but without a permission-granting guardian link.
	err = svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		TenantID:          int64(9054),
		GuardianAccountID: &accountID,
		Children:          child,
	})
	require.ErrorIs(t, err, ErrPhaseNotEligible)

	// The trusted override paths keep working.
	err = svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		TenantID:         int64(9054),
		AllowClosedPhase: true,
		Children:         child,
	})
	require.NoError(t, err)
}

// validatePhaseChildEligibility is the gate the editable-request path reuses.
// It must enforce the per-child rules (class + already-enrolled) but must NOT
// re-apply the linked_parents audience gate — an editable request already
// passed that when it was created, so an anonymous-looking edit request (no
// guardian account) still passes the child gate for a linked_parents phase.
func TestValidatePhaseChildEligibility_SkipsLinkedParentsAudienceGate(t *testing.T) {
	svc := &requestService{}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceLinkedParents)

	// No guardian account, no submit eligibility: the full audience gate
	// would reject this, but the child-only gate must let it through.
	err := svc.validatePhaseChildEligibility(context.Background(), phase, SubmitRequest{
		Children: []SubmitChild{{FirstName: "Kim", LastName: "Test"}},
	})
	require.NoError(t, err)
}

// The child gate keeps enforcing eligible_school_classes regardless of
// audience, so an edit that swaps in an ineligible class is rejected.
func TestValidatePhaseChildEligibility_EnforcesClassRegardlessOfAudience(t *testing.T) {
	svc := &requestService{}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceLinkedParents, "2a")

	err := svc.validatePhaseChildEligibility(context.Background(), phase, SubmitRequest{
		Children: []SubmitChild{{FirstName: "Kim", LastName: "Test", TargetSchoolClass: strPtrEligibility("4c")}},
	})
	require.ErrorIs(t, err, ErrChildClassNotEligible)
}

// The child gate keeps enforcing the new_students already-enrolled check, so
// an edit that swaps in an enrolled child's identity is rejected.
func TestValidatePhaseChildEligibility_EnforcesNewStudents(t *testing.T) {
	repo := &stubEligibilityStudentRepo{exists: true}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{StudentRepo: repo}}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceNewStudents)

	err := svc.validatePhaseChildEligibility(context.Background(), phase, SubmitRequest{
		TenantID: int64(9099),
		Children: []SubmitChild{{FirstName: "Kim", LastName: "Test", DateOfBirth: timezone.NewDate(2019, 4, 12)}},
	})
	require.ErrorIs(t, err, ErrChildAlreadyEnrolled)
}

// Trusted-path requests (admin manual / late invite) deliberately bypassed
// eligibility at creation, so the editable-request path must keep that
// override rather than newly rejecting an edit; every self-service source
// must re-run the child gate.
func TestIsTrustedEnrollmentSource(t *testing.T) {
	assert.True(t, isTrustedEnrollmentSource(enrollmentModels.RequestSourceLateInvite))
	assert.True(t, isTrustedEnrollmentSource(enrollmentModels.RequestSourceAdminManual))
	assert.True(t, isTrustedEnrollmentSource("  "+enrollmentModels.RequestSourceLateInvite+"  "))
	assert.False(t, isTrustedEnrollmentSource(enrollmentModels.RequestSourcePublic))
	assert.False(t, isTrustedEnrollmentSource(""))
}

func TestValidatePhaseEligibility_NewStudentsLookupErrorPropagates(t *testing.T) {
	repo := &stubEligibilityStudentRepo{err: errors.New("boom")}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{StudentRepo: repo}}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceNewStudents)

	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		TenantID: int64(9044),
		Children: []SubmitChild{{FirstName: "Kim", LastName: "Test"}},
	})
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrChildAlreadyEnrolled)
}

// --- assertGuardianMayReEnrollStudent (per-child re-enrollment gate, #1663) ---

// stubGuardianAuthorizer records its calls and returns a fixed verdict/error for
// both identity probes (account and guardian email).
type stubGuardianAuthorizer struct {
	granted      bool
	err          error
	called       bool
	emailCalled  bool
	gotAccount   int64
	gotEmail     string
	gotStudent   int64
	gotTenant    int64
	gotPerm      string
	emailGranted *bool
	emailErr     error
}

func (s *stubGuardianAuthorizer) AccountHasStudentPermission(_ context.Context, accountID, studentID, tenantID int64, permission string) (bool, error) {
	s.called = true
	s.gotAccount, s.gotStudent, s.gotTenant, s.gotPerm = accountID, studentID, tenantID, permission
	return s.granted, s.err
}

func (s *stubGuardianAuthorizer) GuardianEmailHasStudentPermission(_ context.Context, email string, studentID, tenantID int64, permission string) (bool, error) {
	s.emailCalled = true
	s.gotEmail, s.gotStudent, s.gotTenant, s.gotPerm = email, studentID, tenantID, permission
	if s.emailErr != nil {
		return false, s.emailErr
	}
	if s.emailGranted != nil {
		return *s.emailGranted, nil
	}
	return s.granted, s.err
}

// portalSubmitter is the authenticated parents-portal identity.
func portalSubmitter(accountID int64) reEnrollmentSubmitter {
	return reEnrollmentSubmitter{GuardianAccountID: int64PtrEligibility(accountID), GuardianEmail: "parent@example.test"}
}

// A guardian holding parent_portal.enrollment.submit on the MATCHED student may
// re-enroll it; the probe is called with the exact (account, student, tenant).
func TestAssertGuardianMayReEnrollStudent_GrantedPasses(t *testing.T) {
	auth := &stubGuardianAuthorizer{granted: true}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{GuardianAuthorizer: auth}}

	err := svc.assertGuardianMayReEnrollStudent(context.Background(),
		portalSubmitter(4711), int64PtrEligibility(555), int64(9001), 0)
	require.NoError(t, err)
	require.True(t, auth.called)
	assert.False(t, auth.emailCalled, "an authenticated account is decided by the account probe alone")
	assert.Equal(t, int64(4711), auth.gotAccount)
	assert.Equal(t, int64(555), auth.gotStudent)
	assert.Equal(t, int64(9001), auth.gotTenant)
	assert.Equal(t, authorize.GuardianPermissionEnrollmentSubmit, auth.gotPerm)
}

// A guardian WITHOUT the submit permission on the matched student is rejected —
// the core P1 fix: authority over child A must not renew child B. An email that
// WOULD pass must not rescue the denied account: the account is the identity
// approval attaches, so it is the identity that has to hold the permission.
func TestAssertGuardianMayReEnrollStudent_NotGrantedRejected(t *testing.T) {
	emailGranted := true
	auth := &stubGuardianAuthorizer{granted: false, emailGranted: &emailGranted}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{GuardianAuthorizer: auth}}

	err := svc.assertGuardianMayReEnrollStudent(context.Background(),
		portalSubmitter(4711), int64PtrEligibility(555), int64(9001), 3)
	require.ErrorIs(t, err, ErrChildEnrollmentNotPermitted)
	assert.NotErrorIs(t, err, ErrInvalidSubmission, "not-permitted is a 403, not a 400")
	assert.False(t, auth.emailCalled, "no email fallback for an account submission")
}

// No matched student (fresh create) skips the probe entirely.
func TestAssertGuardianMayReEnrollStudent_NoMatchSkips(t *testing.T) {
	auth := &stubGuardianAuthorizer{granted: false}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{GuardianAuthorizer: auth}}

	err := svc.assertGuardianMayReEnrollStudent(context.Background(),
		portalSubmitter(4711), nil, int64(9001), 0)
	require.NoError(t, err)
	assert.False(t, auth.called, "no matched student → no per-child probe")
	assert.False(t, auth.emailCalled)
}

// An accountless submission (the late-invite path) is authorized by the guardian
// EMAIL the request is bound to: the invited parent may renew a child they are
// already a permitted guardian of.
func TestAssertGuardianMayReEnrollStudent_EmailIdentityGrantedPasses(t *testing.T) {
	auth := &stubGuardianAuthorizer{granted: true}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{GuardianAuthorizer: auth}}

	err := svc.assertGuardianMayReEnrollStudent(context.Background(),
		reEnrollmentSubmitter{GuardianEmail: "invited@example.test"}, int64PtrEligibility(555), int64(9001), 0)
	require.NoError(t, err)
	require.True(t, auth.emailCalled)
	assert.False(t, auth.called, "no account → no account probe")
	assert.Equal(t, "invited@example.test", auth.gotEmail)
	assert.Equal(t, int64(555), auth.gotStudent)
	assert.Equal(t, int64(9001), auth.gotTenant)
	assert.Equal(t, authorize.GuardianPermissionEnrollmentSubmit, auth.gotPerm)
}

// The reviewed P1: a late-invite recipient naming a child they have no guardian
// relationship with is rejected. The invite proves the school invited this email
// into the closed phase, never authority over one enrolled child.
func TestAssertGuardianMayReEnrollStudent_EmailIdentityNotGrantedRejected(t *testing.T) {
	auth := &stubGuardianAuthorizer{granted: false}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{GuardianAuthorizer: auth}}

	err := svc.assertGuardianMayReEnrollStudent(context.Background(),
		reEnrollmentSubmitter{GuardianEmail: "invited@example.test"}, int64PtrEligibility(555), int64(9001), 2)
	require.ErrorIs(t, err, ErrChildEnrollmentNotPermitted)
	require.True(t, auth.emailCalled)
}

// A submission carrying NO identity at all cannot be authorized against
// anything, so it may not pin a live student.
func TestAssertGuardianMayReEnrollStudent_NoIdentityFailsClosed(t *testing.T) {
	auth := &stubGuardianAuthorizer{granted: true}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{GuardianAuthorizer: auth}}

	err := svc.assertGuardianMayReEnrollStudent(context.Background(),
		reEnrollmentSubmitter{}, int64PtrEligibility(555), int64(9001), 0)
	require.ErrorIs(t, err, ErrChildEnrollmentNotPermitted)
	assert.False(t, auth.called)
	assert.False(t, auth.emailCalled)
}

// Admin manual enrollment is the one deliberate bypass: staff act with their own
// authorization and must be able to re-enroll a child whose guardian is not yet
// recorded, exactly like the paper form.
func TestAssertGuardianMayReEnrollStudent_AdminManagedBypasses(t *testing.T) {
	auth := &stubGuardianAuthorizer{granted: false}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{GuardianAuthorizer: auth}}

	err := svc.assertGuardianMayReEnrollStudent(context.Background(),
		reEnrollmentSubmitterFor(enrollmentModels.RequestSourceAdminManual, nil, "office@example.test"),
		int64PtrEligibility(555), int64(9001), 0)
	require.NoError(t, err)
	assert.False(t, auth.called)
	assert.False(t, auth.emailCalled)
}

// A late invite is NOT an admin override: it stays subject to the per-child gate.
func TestReEnrollmentSubmitterFor_OnlyAdminManualOverrides(t *testing.T) {
	assert.True(t, reEnrollmentSubmitterFor(enrollmentModels.RequestSourceAdminManual, nil, "a@b.test").AdminManaged)
	assert.False(t, reEnrollmentSubmitterFor(enrollmentModels.RequestSourceLateInvite, nil, "a@b.test").AdminManaged)
	assert.False(t, reEnrollmentSubmitterFor(enrollmentModels.RequestSourcePublic, nil, "a@b.test").AdminManaged)
	assert.False(t, reEnrollmentSubmitterFor("", nil, "a@b.test").AdminManaged)
	// The email is normalized so the probe hits the LOWER(email) unique index.
	assert.Equal(t, "a@b.test", reEnrollmentSubmitterFor("", nil, "  A@B.Test ").GuardianEmail)
}

// Fail closed: a submission that resolved an existing student with no authorizer
// wired is a misconfiguration, never a silent bypass.
func TestAssertGuardianMayReEnrollStudent_NilAuthorizerFailsClosed(t *testing.T) {
	svc := &requestService{}

	err := svc.assertGuardianMayReEnrollStudent(context.Background(),
		portalSubmitter(4711), int64PtrEligibility(555), int64(9001), 1)
	require.ErrorIs(t, err, ErrChildEnrollmentNotPermitted)
}

// Same for the accountless identity: no authorizer, no pin.
func TestAssertGuardianMayReEnrollStudent_NilAuthorizerFailsClosedForEmail(t *testing.T) {
	svc := &requestService{}

	err := svc.assertGuardianMayReEnrollStudent(context.Background(),
		reEnrollmentSubmitter{GuardianEmail: "invited@example.test"}, int64PtrEligibility(555), int64(9001), 1)
	require.ErrorIs(t, err, ErrChildEnrollmentNotPermitted)
}

// A repository error propagates and is NOT mistaken for a denial.
func TestAssertGuardianMayReEnrollStudent_ProbeErrorPropagates(t *testing.T) {
	auth := &stubGuardianAuthorizer{err: errors.New("boom")}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{GuardianAuthorizer: auth}}

	err := svc.assertGuardianMayReEnrollStudent(context.Background(),
		portalSubmitter(4711), int64PtrEligibility(555), int64(9001), 0)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrChildEnrollmentNotPermitted)
}

// The email probe's errors get the same treatment: unknown is not "denied", and
// never silently "allowed" either.
func TestAssertGuardianMayReEnrollStudent_EmailProbeErrorPropagates(t *testing.T) {
	auth := &stubGuardianAuthorizer{emailErr: errors.New("boom")}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{GuardianAuthorizer: auth}}

	err := svc.assertGuardianMayReEnrollStudent(context.Background(),
		reEnrollmentSubmitter{GuardianEmail: "invited@example.test"}, int64PtrEligibility(555), int64(9001), 0)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrChildEnrollmentNotPermitted)
}
