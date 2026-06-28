package enrollment_test

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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
		GuardianProfileRepo:      repoFactory.GuardianProfile,
		GuardianPhoneRepo:        repoFactory.GuardianPhoneNumber,
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

func proposedChangeSubmission(t *testing.T, env *requestTestEnv, result *enrollmentService.SubmitResult) enrollmentService.SubmitRequest {
	t.Helper()
	proposed := validSubmission(env.phaseID)
	require.Len(t, result.Children, len(proposed.Children))
	for i := range proposed.Children {
		proposed.Children[i].ID = result.Children[i].ID
	}
	return proposed
}

func enableChangeRequestMode(t *testing.T, env *requestTestEnv, childID int64) {
	t.Helper()
	reason := "Warteliste"
	require.NoError(t, repositories.NewFactory(env.db).RequestChild.UpdateStatus(
		testpkg.TenantContext(1),
		childID,
		enrollmentModels.ChildStatusWaitlisted,
		&reason,
		env.creatorID,
	))
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

func TestRequestService_ReplaceEditable_PreservesChildCustomDataByIDWhenMiddleChildRemoved(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	base := validSubmission(env.phaseID)
	base.GuardianEmail = "multi-child-edit@example.com"
	base.Children = []enrollmentService.SubmitChild{
		{
			FirstName:        "Lina",
			LastName:         "Beispiel",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: testpkg.Int16Ptr(1),
		},
		{
			FirstName:        "Mika",
			LastName:         "Beispiel",
			DateOfBirth:      timezone.NewDate(2017, 7, 22),
			TargetGradeLevel: testpkg.Int16Ptr(2),
		},
		{
			FirstName:        "Noah",
			LastName:         "Beispiel",
			DateOfBirth:      timezone.NewDate(2016, 8, 3),
			TargetGradeLevel: testpkg.Int16Ptr(3),
		},
	}
	result, err := env.svc.Submit(ctx, base)
	require.NoError(t, err)
	require.Len(t, result.Children, 3)

	_, err = env.db.NewUpdate().
		TableExpr("enrollment.requests").
		Set("schema_id = NULL").
		Where("id = ?", result.Request.ID).
		Exec(ctx)
	require.NoError(t, err)
	legacyByChild := map[int64]string{
		result.Children[0].ID: "first-child",
		result.Children[1].ID: "removed-middle-child",
		result.Children[2].ID: "last-child",
	}
	for childID, legacy := range legacyByChild {
		_, err = env.db.NewUpdate().
			TableExpr("enrollment.request_children").
			Set("custom_data = ?", map[string]any{"legacy_note": legacy}).
			Where("id = ?", childID).
			Exec(ctx)
		require.NoError(t, err)
	}

	edit := base
	edit.Children = []enrollmentService.SubmitChild{
		{
			ID:               result.Children[0].ID,
			FirstName:        base.Children[0].FirstName,
			LastName:         base.Children[0].LastName,
			DateOfBirth:      base.Children[0].DateOfBirth,
			TargetGradeLevel: base.Children[0].TargetGradeLevel,
			CustomData:       map[string]any{},
		},
		{
			ID:               result.Children[2].ID,
			FirstName:        base.Children[2].FirstName,
			LastName:         base.Children[2].LastName,
			DateOfBirth:      base.Children[2].DateOfBirth,
			TargetGradeLevel: base.Children[2].TargetGradeLevel,
			CustomData:       map[string]any{},
		},
	}
	_, err = env.svc.ReplaceEditable(ctx, result.Request.StatusToken, edit)
	require.NoError(t, err)

	_, children, err := env.svc.GetByStatusToken(ctx, result.Request.StatusToken)
	require.NoError(t, err)
	require.Len(t, children, 2)
	assert.Equal(t, "first-child", children[0].CustomData["legacy_note"])
	assert.Equal(t, "last-child", children[1].CustomData["legacy_note"])
	assert.NotEqual(t, "removed-middle-child", children[1].CustomData["legacy_note"])
}

func TestChangeRequestService_Approve_PreservesLegacyCustomDataWithoutSchema(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	result := makeLegacyNoSchemaRequest(t, env, enrollmentModels.ChildStatusWaitlisted)
	svc := newChangeRequestServiceForTest(env)

	phone := "+49 221 555 991"
	proposed := proposedChangeSubmission(t, env, result)
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

func TestChangeRequestService_Create_RequiresStableChildIdentity(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	base := validSubmission(env.phaseID)
	base.Children = append(base.Children, enrollmentService.SubmitChild{
		FirstName:        "Mika",
		LastName:         "Beispiel",
		DateOfBirth:      timezone.NewDate(2017, 7, 22),
		TargetGradeLevel: testpkg.Int16Ptr(2),
	})
	result, err := env.svc.Submit(ctx, base)
	require.NoError(t, err)
	require.Len(t, result.Children, 2)
	enableChangeRequestMode(t, env, result.Children[0].ID)
	svc := newChangeRequestServiceForTest(env)

	missingID := base
	_, err = svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: missingID,
		ParentNote: "Bitte pruefen.",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrChangeRequestInvalidData))

	reordered := base
	reordered.Children[0].ID = result.Children[1].ID
	reordered.Children[1].ID = result.Children[0].ID
	_, err = svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: reordered,
		ParentNote: "Bitte pruefen.",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrChangeRequestInvalidData))
}

func TestChangeRequestService_Approve_RejectsStaleBaseSnapshot(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)
	enableChangeRequestMode(t, env, result.Children[0].ID)
	svc := newChangeRequestServiceForTest(env)

	proposed := proposedChangeSubmission(t, env, result)
	proposed.Children[0].FirstName = "Lina-Marie"
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte Namen korrigieren.",
	})
	require.NoError(t, err)

	repoFactory := repositories.NewFactory(env.db)
	newer, err := repoFactory.Request.FindByID(ctx, result.Request.ID)
	require.NoError(t, err)
	newer.GuardianLastName = "Neuer"
	require.NoError(t, repoFactory.Request.UpdateGuardianDataWithEmail(ctx, newer))

	_, err = svc.Approve(ctx, created.ChangeRequest.ID, enrollmentService.ReviewChangeRequestInput{
		Note:           "Freigegeben.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrChangeRequestConflict))
}

func TestChangeRequestService_Create_AllowsOnlyOneOpenRequest(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)
	enableChangeRequestMode(t, env, result.Children[0].ID)
	svc := newChangeRequestServiceForTest(env)

	first := proposedChangeSubmission(t, env, result)
	first.Children[0].FirstName = "Lina-Marie"
	_, err = svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: first,
		ParentNote: "Bitte Namen korrigieren.",
	})
	require.NoError(t, err)

	second := proposedChangeSubmission(t, env, result)
	second.GuardianLastName = "Neu"
	_, err = svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: second,
		ParentNote: "Noch eine Aenderung.",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrChangeRequestNotAllowed))
}

func TestChangeRequestService_Create_ForcesStoredGuardianEmail(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)
	enableChangeRequestMode(t, env, result.Children[0].ID)
	svc := newChangeRequestServiceForTest(env)

	proposed := proposedChangeSubmission(t, env, result)
	proposed.GuardianEmail = "attacker@example.com"
	proposed.Children[0].FirstName = "Lina-Marie"
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte Namen korrigieren.",
	})
	require.NoError(t, err)
	assert.Equal(t, result.Request.GuardianEmail, created.ChangeRequest.ProposedSnapshot["guardian_email"])
}

func TestChangeRequestService_Approve_DoesNotReopenUnchangedRejectedChild(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	base := validSubmission(env.phaseID)
	base.Children = append(base.Children, enrollmentService.SubmitChild{
		FirstName:        "Mika",
		LastName:         "Beispiel",
		DateOfBirth:      timezone.NewDate(2017, 7, 22),
		TargetGradeLevel: testpkg.Int16Ptr(2),
	})
	result, err := env.svc.Submit(ctx, base)
	require.NoError(t, err)
	require.Len(t, result.Children, 2)
	repoFactory := repositories.NewFactory(env.db)
	reason := "kein Platz"
	require.NoError(t, repoFactory.RequestChild.UpdateStatus(ctx, result.Children[0].ID, enrollmentModels.ChildStatusRejected, &reason, env.creatorID))
	svc := newChangeRequestServiceForTest(env)

	proposed := base
	for i := range proposed.Children {
		proposed.Children[i].ID = result.Children[i].ID
	}
	phone := "+49 221 555 991"
	proposed.GuardianPhone = &phone
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte Telefonnummer korrigieren.",
	})
	require.NoError(t, err)

	_, err = svc.Approve(ctx, created.ChangeRequest.ID, enrollmentService.ReviewChangeRequestInput{
		Note:           "Freigegeben.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)

	child, err := repoFactory.RequestChild.FindByID(ctx, result.Children[0].ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusRejected, child.Status)
}
