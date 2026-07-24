package enrollment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		TenantID: tenantID,
		Children: []SubmitChild{{FirstName: "Kim", LastName: "Test", DateOfBirth: timezone.NewDate(2019, 4, 12)}},
	})
	require.ErrorIs(t, err, ErrChildNotEnrolled)
	require.ErrorIs(t, err, ErrInvalidSubmission, "not-enrolled error must keep the 400 category")
	assert.Equal(t, tenantID, repo.gotTenant, "lookup must be tenant-scoped explicitly")
}

func TestValidatePhaseEligibility_ExistingStudentsAcceptsEnrolledChild(t *testing.T) {
	repo := &stubEligibilityStudentRepo{exists: true}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{StudentRepo: repo}}
	phase := eligibilityTestPhase(enrollmentModels.PhaseAudienceExistingStudents)

	err := svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{
		TenantID: int64(9053),
		Children: []SubmitChild{{FirstName: "Kim", LastName: "Test", DateOfBirth: timezone.NewDate(2019, 4, 12)}},
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
