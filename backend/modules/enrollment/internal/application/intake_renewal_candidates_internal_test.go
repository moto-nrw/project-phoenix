package application

import (
	"context"
	"errors"
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

	require.NoError(t, svc.holdRenewalCandidates(context.Background(), 7001, phase, children, carried,
		func() (reEnrollmentSubmitter, error) { return reEnrollmentSubmitter{AdminManaged: true}, nil }))
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
