package enrollmenthttp_test

import (
	"testing"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setChildStatus forces one request_children row into the given status —
// the shortcut the rollover would otherwise take via renewalInitialStatus.
func setChildStatus(t *testing.T, env *requestTestEnv, childID int64, status string) {
	t.Helper()
	require.NoError(t, testpkg.TenantCommand4(t, testpkg.TenantContext(env.phase.TenantID), testpkg.SetupTestDB(t), api.NewEnrollmentSuiteRepositories(testpkg.NewDatabase(t, env.db)).Enrollment().UpdateChildStatus, childID, status, nil, env.creatorID))
}

// A parent-initiated change request on a renewal request IS the parent's
// reaction to the Halbjahreswechsel (#2251): the affected children must
// leave the automatic renewal pipeline (deadline worker would otherwise
// withdraw/auto-process them despite the reaction) and re-enter the normal
// admin queue as submitted. The base snapshot must capture the flipped
// status, or the deadline worker's later transition would make the change
// request permanently unapprovable via the snapshot conflict guard.
func TestChangeRequestService_Create_FlipsPendingRenewalToSubmitted(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(env.phase.TenantID)

	result, err := env.svc.Submit(ctx, validSubmission(t, env.phaseID))
	require.NoError(t, err)
	require.Len(t, result.Children, 1)
	setChildStatus(t, env, result.Children[0].ID, enrollmentNative.ChildStatusPendingRenewal)

	svc := newChangeRequestServiceForTest(env)
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentAPI.CreateChangeRequestInput{
		Submission: proposedChangeSubmission(t, env, result),
		ParentNote: "Nur Angebote angepasst.",
	})
	require.NoError(t, err)

	stored, err := enrollmentAPI.ReadOwnerChildForTest(ctx, api.NewEnrollmentSuiteRepositories(testpkg.NewDatabase(t, env.db)).Enrollment(), result.Children[0].ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentNative.ChildStatusSubmitted, stored.Status)

	// The base snapshot must already carry the flipped status so the later
	// approval's conflict guard compares equal states.
	children, ok := created.ChangeRequest.BaseSnapshot["children"].([]any)
	require.True(t, ok)
	require.Len(t, children, 1)
	row, ok := children[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, enrollmentNative.ChildStatusSubmitted, row["status"])

	// And the approval must go through without a snapshot conflict.
	_, err = testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), svc.Approve, created.ChangeRequest.ID, enrollmentAPI.ReviewChangeRequestInput{
		Note:           "Freigegeben.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)
}

func TestChangeRequestService_Create_FlipsAutoRenewedToSubmitted(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(env.phase.TenantID)

	result, err := env.svc.Submit(ctx, validSubmission(t, env.phaseID))
	require.NoError(t, err)
	setChildStatus(t, env, result.Children[0].ID, enrollmentNative.ChildStatusAutoRenewed)

	svc := newChangeRequestServiceForTest(env)
	_, err = svc.Create(ctx, result.Request.StatusToken, enrollmentAPI.CreateChangeRequestInput{
		Submission: proposedChangeSubmission(t, env, result),
	})
	require.NoError(t, err)

	stored, err := enrollmentAPI.ReadOwnerChildForTest(ctx, api.NewEnrollmentSuiteRepositories(testpkg.NewDatabase(t, env.db)).Enrollment(), result.Children[0].ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentNative.ChildStatusSubmitted, stored.Status)
}

func TestChangeRequestService_Create_LeavesNonRenewalStatusesAlone(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(env.phase.TenantID)

	result, err := env.svc.Submit(ctx, validSubmission(t, env.phaseID))
	require.NoError(t, err)
	enableChangeRequestMode(t, env, result.Children[0].ID)

	svc := newChangeRequestServiceForTest(env)
	_, err = svc.Create(ctx, result.Request.StatusToken, enrollmentAPI.CreateChangeRequestInput{
		Submission: proposedChangeSubmission(t, env, result),
	})
	require.NoError(t, err)

	stored, err := enrollmentAPI.ReadOwnerChildForTest(ctx, api.NewEnrollmentSuiteRepositories(testpkg.NewDatabase(t, env.db)).Enrollment(), result.Children[0].ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentNative.ChildStatusWaitlisted, stored.Status)
}
