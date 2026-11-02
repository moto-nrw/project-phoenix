package enrollmenthttp_test

import (
	"errors"
	"testing"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createPendingChangeRequestForNoteTest opens one parent change request that
// waits for the school's decision.
func createPendingChangeRequestForNoteTest(t *testing.T, env *requestTestEnv, svc enrollmentAPI.ChangeRequestService) int64 {
	t.Helper()
	ctx := testpkg.Ctx(t)
	result, err := env.svc.Submit(ctx, validSubmission(t, env.phaseID))
	require.NoError(t, err)
	enableChangeRequestMode(t, env, result.Children[0].ID)

	proposed := proposedChangeSubmission(t, env, result)
	proposed.Children[0].FirstName = "Lina-Marie"
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentAPI.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte Namen korrigieren.",
	})
	require.NoError(t, err)
	require.NotNil(t, created.ChangeRequest)
	return created.ChangeRequest.ID
}

// An approval needs no note (#5024): the school may approve without
// explaining itself. The decision stores no empty note and adds no empty
// staff message to the dialogue with the family.
func TestChangeRequestService_Approve_WithoutNote(t *testing.T) {
	t.Parallel()

	env := setupRequestTest(t)
	ctx := testpkg.Ctx(t)
	svc := newChangeRequestServiceForTest(env)
	changeRequestID := createPendingChangeRequestForNoteTest(t, env, svc)

	approved, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), svc.Approve, changeRequestID, enrollmentAPI.ReviewChangeRequestInput{
		Note:           "   ",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)
	require.NotNil(t, approved.ChangeRequest)
	assert.Equal(t, enrollmentNative.ChangeRequestStatusApproved, approved.ChangeRequest.Status)
	assert.Nil(t, approved.ChangeRequest.AdminDecisionNote, "an approval without a note stores no note")
	for _, message := range approved.Messages {
		assert.NotEqual(t, enrollmentNative.ChangeRequestMessageAuthorStaff, message.AuthorType,
			"an approval without a note writes no staff message")
	}
}

// A note on an approval still reaches the family as the decision note.
func TestChangeRequestService_Approve_KeepsAGivenNote(t *testing.T) {
	t.Parallel()

	env := setupRequestTest(t)
	ctx := testpkg.Ctx(t)
	svc := newChangeRequestServiceForTest(env)
	changeRequestID := createPendingChangeRequestForNoteTest(t, env, svc)

	approved, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), svc.Approve, changeRequestID, enrollmentAPI.ReviewChangeRequestInput{
		Note:           " Freigegeben. ",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)
	require.NotNil(t, approved.ChangeRequest.AdminDecisionNote)
	assert.Equal(t, "Freigegeben.", *approved.ChangeRequest.AdminDecisionNote)
	var staffBodies []string
	for _, message := range approved.Messages {
		if message.AuthorType == enrollmentNative.ChangeRequestMessageAuthorStaff {
			staffBodies = append(staffBodies, message.Body)
		}
	}
	assert.Equal(t, []string{"Freigegeben."}, staffBodies)
}

// A rejection still needs a note: the family has to learn why.
func TestChangeRequestService_Reject_RequiresNote(t *testing.T) {
	t.Parallel()

	env := setupRequestTest(t)
	ctx := testpkg.Ctx(t)
	svc := newChangeRequestServiceForTest(env)
	changeRequestID := createPendingChangeRequestForNoteTest(t, env, svc)

	_, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), svc.Reject, changeRequestID, enrollmentAPI.ReviewChangeRequestInput{
		Note:           "  ",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.Error(t, err)
	var rejection *enrollmentNative.InvalidInputError
	require.True(t, errors.As(err, &rejection))
	assert.Equal(t, enrollmentNative.CodeChangeRequestNoteRequired, rejection.Code)
	assert.Equal(t, "note", rejection.Field)

	stored, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Enrollment().ChangeRequestByID, changeRequestID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentNative.ChangeRequestStatusPendingReview, stored.Status, "a refused rejection changes nothing")
}
