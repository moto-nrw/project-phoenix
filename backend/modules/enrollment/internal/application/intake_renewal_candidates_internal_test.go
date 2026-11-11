package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

type renewalCandidateMatches struct {
	StudentMatches
	matches    [][]int64
	err        error
	calls      int
	identities []enrollment.StudentIdentity
	tenantID   int64
	locked     []int64
}

func (r *renewalCandidateMatches) FindEnrolledStudentIDsByIdentities(_ context.Context, tenantID int64, identities []enrollment.StudentIdentity) ([][]int64, error) {
	r.calls++
	r.tenantID, r.identities = tenantID, identities
	return r.matches, r.err
}

func (r *renewalCandidateMatches) LockStudents(_ context.Context, ids []int64) error {
	r.locked = ids
	return r.err
}

func TestRenewalCandidatesBatchUniqueMatchesAndCarriedPins(t *testing.T) {
	t.Parallel()
	repo := &renewalCandidateMatches{matches: [][]int64{{30}, {10, 20}, nil, {30}}}
	svc := NewIntake(IntakeDependencies{Students: repo, Settings: intakeSettingsFake{}})
	children := []SubmitChild{
		{FirstName: "Lina", LastName: "Test", DateOfBirth: calendar.NewDate(2018, 4, 15)},
		{FirstName: "Mila", LastName: "Test", DateOfBirth: calendar.NewDate(2018, 4, 15)},
		{FirstName: "Kim", LastName: "Test", DateOfBirth: calendar.NewDate(2018, 4, 15)},
		{FirstName: "Lina", LastName: "Test", DateOfBirth: calendar.NewDate(2018, 4, 15)},
	}
	phase := eligibilityTestPhase(enrollment.PhaseAudienceExistingStudents)
	carried := []*int64{int64PtrEligibility(40), int64PtrEligibility(5)}
	ids, err := svc.renewalCandidates(context.Background(), 7001, phase, children, carried, true)
	require.NoError(t, err)
	require.Equal(t, []int64{5, 30, 40}, ids, "ambiguous matches add no candidate; carried pins survive")
	require.Equal(t, 1, repo.calls, "one batch regardless of child count")
	require.Equal(t, int64(7001), repo.tenantID)
	require.Len(t, repo.identities, len(children))
	for i, child := range children {
		require.Equal(t, enrollment.StudentIdentity{FirstName: child.FirstName, LastName: child.LastName, Birthday: child.DateOfBirth}, repo.identities[i])
	}

	holds, err := svc.holdRenewalCandidates(context.Background(), 7001, phase, children, carried,
		func() (reEnrollmentSubmitter, error) { return reEnrollmentSubmitter{AdminManaged: true}, nil })
	require.NoError(t, err)
	require.True(t, holds[30])
	require.Equal(t, ids, repo.locked, "staff lock the candidates without guardian authorization")
}

func TestRenewalCandidatesFailClosedOnLookupErrors(t *testing.T) {
	t.Parallel()
	lookupErr := errors.New("directory unavailable")
	for _, repo := range []*renewalCandidateMatches{{err: lookupErr}, {matches: nil}} {
		svc := NewIntake(IntakeDependencies{Students: repo, Settings: intakeSettingsFake{}})
		ids, err := svc.renewalCandidates(context.Background(), 7001,
			eligibilityTestPhase(enrollment.PhaseAudienceExistingStudents), []SubmitChild{{FirstName: "Lina"}}, nil, true)
		require.Error(t, err)
		require.Nil(t, ids)
		if repo.err != nil {
			require.ErrorIs(t, err, lookupErr)
		}
	}
}

func TestRenewalCandidatesHoldSiblingsOnce(t *testing.T) {
	t.Parallel()
	hold := &countingRenewalHold{}
	svc := NewIntake(IntakeDependencies{Students: &renewalCandidateMatches{matches: [][]int64{{30}, {10}}}, GuardianHold: hold, Settings: intakeSettingsFake{}})
	account := int64(7)
	holds, err := svc.holdRenewalCandidates(context.Background(), 7001, eligibilityTestPhase(enrollment.PhaseAudienceExistingStudents), []SubmitChild{{}, {}}, nil,
		func() (reEnrollmentSubmitter, error) { return reEnrollmentSubmitter{GuardianAccountID: &account}, nil })
	require.NoError(t, err)
	require.True(t, holds[30])
	require.Equal(t, 1, hold.calls, "one held authorization regardless of sibling count")
}

type countingRenewalHold struct{ calls int }

func (h *countingRenewalHold) HoldAccountStudentsPermission(_ context.Context, _ int64, ids []int64, _ int64, _ string) (map[int64]bool, error) {
	h.calls++
	grants := map[int64]bool{}
	for _, id := range ids {
		grants[id] = true
	}
	return grants, nil
}
func (h *countingRenewalHold) HoldGuardianEmailStudentsPermission(ctx context.Context, _ string, ids []int64, tenantID int64, permission string) (map[int64]bool, error) {
	return h.HoldAccountStudentsPermission(ctx, 0, ids, tenantID, permission)
}

// The directory can gain a new match while the dedup lock is being acquired.
// Neither submit nor replace may take that child's lock after the phase locks,
// including the staff path, which bypasses guardian authorization.
func TestIntakeRejectsMatchesOutsideItsHeldCandidates(t *testing.T) {
	t.Parallel()
	for _, staff := range []bool{false, true} {
		t.Run(fmt.Sprintf("staff=%t", staff), func(t *testing.T) {
			t.Parallel()
			repo := &renewalCandidateMatches{matches: [][]int64{{10}}}
			svc := NewIntake(IntakeDependencies{Students: repo, GuardianHold: &countingRenewalHold{}, Settings: intakeSettingsFake{}})
			account := int64(7)
			submitter := reEnrollmentSubmitter{GuardianAccountID: &account, AdminManaged: staff}
			phase := eligibilityTestPhase(enrollment.PhaseAudienceExistingStudents)
			children := []SubmitChild{{FirstName: "Lina"}}
			holds, err := svc.holdRenewalCandidates(context.Background(), 7001, phase, children, nil, func() (reEnrollmentSubmitter, error) { return submitter, nil })
			require.NoError(t, err)
			repo.matches = [][]int64{{20}}
			_, err = svc.createSubmittedChildren(context.Background(), &SubmitRequest{TenantID: 7001, Children: children}, &submissionPlan{phase: phase, renewalHolds: holds}, &enrollment.Request{}, nil, submitter)
			require.ErrorIs(t, err, enrollment.ErrChildEnrollmentNotPermitted)
			_, err = svc.replacementStudentPin(context.Background(), &replacePlan{
				req: &enrollment.Request{TenantID: 7001, GuardianAccountID: &account}, phase: phase,
				matchedExisting: []*enrollment.RequestChild{nil}, renewalHolds: holds, renewalMatches: repo.matches,
			}, 0, children[0])
			require.ErrorIs(t, err, enrollment.ErrChildEnrollmentNotPermitted)
		})
	}
}
