package enrollmenthttp_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A change request created before ADR 0170 recorded empty answers and consent
// flags as null in its base snapshot, against rows that stored null. The
// owner now reads those rows as {} (#6092); the approval must not read the
// old null as a conflicting edit.
func TestChangeRequestService_Approve_AcceptsNullEmptyValuesInOlderBaseSnapshot(t *testing.T) {
	t.Parallel()

	env := setupRequestTest(t)
	ctx := testpkg.Ctx(t)
	result, err := env.svc.Submit(ctx, validSubmission(t, env.phaseID))
	require.NoError(t, err)
	enableChangeRequestMode(t, env, result.Children[0].ID)
	svc := newChangeRequestServiceForTest(env)

	proposed := proposedChangeSubmission(t, env, result)
	proposed.Children[0].FirstName = "Lina-Marie"
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentAPI.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte Namen korrigieren.",
	})
	require.NoError(t, err)

	tenantID := testpkg.Tenant(t)
	for _, statement := range []string{
		"UPDATE enrollment.requests SET custom_data = 'null'::jsonb, consent_flags = 'null'::jsonb WHERE tenant_id = ? AND id = ?",
		"UPDATE enrollment.request_children SET custom_data = 'null'::jsonb WHERE tenant_id = ? AND request_id = ?",
	} {
		_, err = env.db.NewRaw(statement, tenantID, result.Request.ID).Exec(ctx)
		require.NoError(t, err)
	}
	_, err = env.db.NewRaw(`UPDATE enrollment.change_requests
		SET base_snapshot = jsonb_set(jsonb_set(jsonb_set(base_snapshot, '{custom_data}', 'null'), '{consent_flags}', 'null'), '{children,0,custom_data}', 'null')
		WHERE tenant_id = ? AND id = ?`, tenantID, created.ChangeRequest.ID).Exec(ctx)
	require.NoError(t, err)

	_, err = testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), svc.Approve, created.ChangeRequest.ID, enrollmentAPI.ReviewChangeRequestInput{
		Note:           "Freigegeben.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)
}
