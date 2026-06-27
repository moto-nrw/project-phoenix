package enrollment_test

import (
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests use setupRequestTest from request_service_test.go; that helper
// calls testpkg.SetupTestDB and owns cleanup for the shared request fixtures.
func newChangeRequestServiceForTest(env *requestTestEnv) enrollmentService.ChangeRequestService {
	repoFactory := repositories.NewFactory(env.db)
	return enrollmentService.NewChangeRequestService(enrollmentService.ChangeRequestServiceConfig{
		ChangeRequestRepo:        repoFactory.ChangeRequest,
		MessageRepo:              repoFactory.ChangeRequestMessage,
		RequestRepo:              repoFactory.Request,
		RequestChildRepo:         repoFactory.RequestChild,
		RequestGuardianRepo:      repoFactory.RequestGuardian,
		RequestChildOfferingRepo: repoFactory.RequestChildOffering,
		CareOfferingRepo:         repoFactory.CareOffering,
		FormSchemaRepo:           repoFactory.FormSchema,
		PhaseRepo:                repoFactory.Phase,
		SchoolRepo:               repoFactory.School,
		Settings:                 env.settings,
		OutboxEnqueuer:           env.outbox,
		FrontendURL:              "http://localhost:3000",
		ParentsURL:               "http://parents.localhost:3000",
		DB:                       env.db,
		Logger:                   slog.Default(),
	})
}

func makeLegacyNoSchemaRequest(t *testing.T, env *requestTestEnv, childStatus string) *enrollmentService.SubmitResult {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)
	require.Len(t, result.Children, 1)

	_, err = env.db.NewUpdate().
		TableExpr("enrollment.requests").
		Set("schema_id = NULL").
		Set("custom_data = ?", map[string]any{
			"seed_source": "legacy-seed",
			"note":        "Demo-Datensatz fuer Anmeldung und Elternportal",
		}).
		Where("id = ?", result.Request.ID).
		Exec(ctx)
	require.NoError(t, err)

	q := env.db.NewUpdate().
		TableExpr("enrollment.request_children").
		Set("custom_data = ?", map[string]any{"allergies": "keine"}).
		Where("id = ?", result.Children[0].ID)
	if childStatus != "" {
		q = q.Set("status = ?", childStatus)
	}
	_, err = q.Exec(ctx)
	require.NoError(t, err)

	return result
}

func TestRequestService_ReplaceEditable_PreservesLegacyCustomDataWithoutSchema(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	result := makeLegacyNoSchemaRequest(t, env, "")

	phone := "+49 221 555 991"
	edit := validSubmission(env.phaseID)
	edit.GuardianPhone = &phone
	edit.CustomData = map[string]any{}
	edit.Children[0].CustomData = map[string]any{}

	_, err := env.svc.ReplaceEditable(ctx, result.Request.StatusToken, edit)
	require.NoError(t, err)

	stored, children, err := env.svc.GetByStatusToken(ctx, result.Request.StatusToken)
	require.NoError(t, err)
	require.Len(t, children, 1)
	require.NotNil(t, stored.GuardianPhone)
	assert.Equal(t, phone, *stored.GuardianPhone)
	assert.Equal(t, "legacy-seed", stored.CustomData["seed_source"])
	assert.Equal(t, "Demo-Datensatz fuer Anmeldung und Elternportal", stored.CustomData["note"])
	assert.Equal(t, "keine", children[0].CustomData["allergies"])
}

func TestChangeRequestService_Approve_PreservesLegacyCustomDataWithoutSchema(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	result := makeLegacyNoSchemaRequest(t, env, enrollmentModels.ChildStatusWaitlisted)
	svc := newChangeRequestServiceForTest(env)

	phone := "+49 221 555 991"
	proposed := validSubmission(env.phaseID)
	proposed.GuardianPhone = &phone
	proposed.CustomData = map[string]any{}
	proposed.Children[0].FirstName = "Lina-Marie"
	proposed.Children[0].CustomData = map[string]any{}

	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte Daten korrigieren.",
	})
	require.NoError(t, err)
	require.NotNil(t, created.ChangeRequest)
	assert.Equal(t, "legacy-seed", created.ChangeRequest.ProposedSnapshot["custom_data"].(map[string]any)["seed_source"])

	_, err = svc.Approve(ctx, created.ChangeRequest.ID, enrollmentService.ReviewChangeRequestInput{
		Note:           "Freigegeben.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)

	stored, children, err := env.svc.GetByStatusToken(ctx, result.Request.StatusToken)
	require.NoError(t, err)
	require.Len(t, children, 1)
	require.NotNil(t, stored.GuardianPhone)
	assert.Equal(t, phone, *stored.GuardianPhone)
	assert.Equal(t, "legacy-seed", stored.CustomData["seed_source"])
	assert.Equal(t, "Demo-Datensatz fuer Anmeldung und Elternportal", stored.CustomData["note"])
	assert.Equal(t, "Lina-Marie", children[0].FirstName)
	assert.Equal(t, "keine", children[0].CustomData["allergies"])
}
