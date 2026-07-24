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
