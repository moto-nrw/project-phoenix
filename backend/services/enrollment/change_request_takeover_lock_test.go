package enrollment_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ADR 0003: once a child is taken over into care, change wishes for that child
// run through the parent app only. The status link stays readable, the change
// form locks that child — per child, so a sibling still under review keeps its
// change path.

// markTakenOver moves a request child into the state an approval leaves behind:
// approved and linked to a real student row.
func markTakenOver(t *testing.T, env *requestTestEnv, childID int64, firstName string) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	student := testpkg.CreateTestStudent(t, env.db, firstName, "Beispiel", "1a")
	_, err := env.db.NewUpdate().
		TableExpr("enrollment.request_children").
		Set("status = ?", enrollmentModels.ChildStatusApproved).
		Set("created_student_id = ?", student.ID).
		Where("id = ?", childID).
		Exec(ctx)
	require.NoError(t, err)
}

func twoChildSubmission(phaseID int64) enrollmentService.SubmitRequest {
	submission := validSubmission(phaseID)
	submission.Children = append(submission.Children, enrollmentService.SubmitChild{
		FirstName:        "Timo",
		LastName:         "Beispiel",
		DateOfBirth:      timezone.NewDate(2019, 6, 2),
		TargetGradeLevel: testpkg.Int16Ptr(1),
	})
	return submission
}

func TestChangeRequest_AllChildrenTakenOverLeavesNoChangeForm(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)
	markTakenOver(t, env, result.Children[0].ID, "Lina")

	req, children, err := env.svc.GetByStatusToken(ctx, result.Request.StatusToken)
	require.NoError(t, err)
	mode, err := env.svc.EditModeForStatus(ctx, req, children)
	require.NoError(t, err)
	assert.Equal(t, enrollmentService.EditModeNone, mode,
		"a fully taken-over enrollment offers no change form at all")

	_, err = env.svc.GetEditDraft(ctx, result.Request.StatusToken)
	require.ErrorIs(t, err, enrollmentService.ErrEditNotAllowed)

	svc := newChangeRequestServiceForTest(env)
	_, err = svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposedChangeSubmission(t, env, result),
	})
	require.ErrorIs(t, err, enrollmentService.ErrChangeRequestNotAllowed)
}

func TestChangeRequest_SiblingUnderReviewStaysChangeable(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result, err := env.svc.Submit(ctx, twoChildSubmission(env.phaseID))
	require.NoError(t, err)
	require.Len(t, result.Children, 2)
	markTakenOver(t, env, result.Children[0].ID, "Lina")
	enableChangeRequestMode(t, env, result.Children[1].ID)

	req, children, err := env.svc.GetByStatusToken(ctx, result.Request.StatusToken)
	require.NoError(t, err)
	mode, err := env.svc.EditModeForStatus(ctx, req, children)
	require.NoError(t, err)
	assert.Equal(t, enrollmentService.EditModeChangeRequest, mode)

	draft, err := env.svc.GetEditDraft(ctx, result.Request.StatusToken)
	require.NoError(t, err)
	require.Len(t, draft.Children, 2)
	assert.True(t, enrollmentService.ChildTakenOver(draft.Children[0]))
	assert.False(t, enrollmentService.ChildTakenOver(draft.Children[1]))

	proposed := twoChildSubmission(env.phaseID)
	for i := range proposed.Children {
		proposed.Children[i].ID = result.Children[i].ID
	}
	proposed.Children[1].TargetGradeLevel = testpkg.Int16Ptr(2)

	svc := newChangeRequestServiceForTest(env)
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte Jahrgang aendern.",
	})
	require.NoError(t, err)
	require.NotNil(t, created.ChangeRequest)
	proposedChildren, ok := created.ChangeRequest.ProposedSnapshot["children"].([]any)
	require.True(t, ok)
	require.Len(t, proposedChildren, 2)
	sibling, ok := proposedChildren[1].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 2, sibling["target_grade_level"],
		"the sibling still under review carries the requested change")
}

func TestChangeRequest_TakenOverChildCannotBeChanged(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result, err := env.svc.Submit(ctx, twoChildSubmission(env.phaseID))
	require.NoError(t, err)
	require.Len(t, result.Children, 2)
	markTakenOver(t, env, result.Children[0].ID, "Lina")
	enableChangeRequestMode(t, env, result.Children[1].ID)

	proposed := twoChildSubmission(env.phaseID)
	for i := range proposed.Children {
		proposed.Children[i].ID = result.Children[i].ID
	}
	proposed.Children[0].TargetGradeLevel = testpkg.Int16Ptr(2)

	svc := newChangeRequestServiceForTest(env)
	_, err = svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
	})
	require.ErrorIs(t, err, enrollmentService.ErrChangeRequestChildLocked)
}
