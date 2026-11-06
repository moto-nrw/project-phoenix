package integration

import (
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The review list's child name search finds "Özdemir" for "ozdem" (#4949).
func TestChangeRequestReviewSearchIgnoresAccents(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := enrollmentCompose.New()
	ctx := testpkg.Ctx(t)

	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	request := &enrollment.Request{PhaseID: phase.ID, GuardianFirstName: "First", GuardianLastName: "Last", GuardianEmail: "guardian@example.test", StatusToken: fmt.Sprintf("review-search-%d", testpkg.Tenant(t))}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertRequest, request))
	child := &enrollment.RequestChild{RequestID: request.ID, FirstName: "Zoë", LastName: "Özdemir", DateOfBirth: "2020-03-29"}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertChild, child))
	changeRequest := &enrollment.ChangeRequest{RequestID: request.ID, Status: "pending_review"}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertChangeRequest, changeRequest))

	for _, search := range []string{"Özdem", "ozdem", "zoe ozdemir"} {
		rows, err := testpkg.TenantQuery1(t, ctx, db, owner.ChangeRequestsForReview, enrollment.ChangeRequestReviewFilters{Statuses: []string{"pending_review"}, Search: search, Limit: 10})
		require.NoError(t, err, search)
		require.Len(t, rows, 1, search)
		require.Equal(t, changeRequest.ID, rows[0].ID, search)
	}
}

// A fullwidth ％ folds to "%"; the search escapes after folding, so it
// matches literally instead of every child (#5791).
func TestChangeRequestReviewSearchTreatsFullwidthLikeCharactersLiterally(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := enrollmentCompose.New()
	ctx := testpkg.Ctx(t)

	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	request := &enrollment.Request{PhaseID: phase.ID, GuardianFirstName: "First", GuardianLastName: "Last", GuardianEmail: "guardian@example.test", StatusToken: fmt.Sprintf("review-search-fullwidth-%d", testpkg.Tenant(t))}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertRequest, request))
	child := &enrollment.RequestChild{RequestID: request.ID, FirstName: "Zoë", LastName: "Özdemir", DateOfBirth: "2020-03-29"}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertChild, child))
	changeRequest := &enrollment.ChangeRequest{RequestID: request.ID, Status: "pending_review"}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertChangeRequest, changeRequest))

	for _, search := range []string{"％", "＿", "＼"} {
		rows, err := testpkg.TenantQuery1(t, ctx, db, owner.ChangeRequestsForReview, enrollment.ChangeRequestReviewFilters{Statuses: []string{"pending_review"}, Search: search, Limit: 10})
		require.NoError(t, err, search)
		require.Empty(t, rows, search)
	}
}
