package enrollment_test

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
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

func newChangeRequestServiceWithDecisionForTest(t *testing.T, env *decisionTestEnv) enrollmentService.ChangeRequestService {
	t.Helper()
	return enrollmentService.NewChangeRequestService(enrollmentService.ChangeRequestServiceConfig{
		ChangeRequestRepo:        env.repos.ChangeRequest,
		MessageRepo:              env.repos.ChangeRequestMessage,
		RequestRepo:              env.repos.Request,
		RequestChildRepo:         env.repos.RequestChild,
		RequestGuardianRepo:      env.repos.RequestGuardian,
		RequestChildOfferingRepo: env.repos.RequestChildOffering,
		CareOfferingRepo:         env.repos.CareOffering,
		FormSchemaRepo:           env.repos.FormSchema,
		PhaseRepo:                env.repos.Phase,
		SchoolRepo:               env.repos.School,
		GuardianProfileRepo:      env.repos.GuardianProfile,
		GuardianPhoneRepo:        env.repos.GuardianPhoneNumber,
		DecisionService:          changeRequestApplierForTest(t, env),
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

func TestChangeRequestService_Approve_RejectsActiveDuplicateAfterRename(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	first := validSubmission(env.phaseID)
	first.GuardianEmail = "duplicate-approval@example.com"
	first.Children[0].FirstName = "Lina"
	first.Children[0].LastName = "Beispiel"
	firstResult, err := env.svc.Submit(ctx, first)
	require.NoError(t, err)
	enableChangeRequestMode(t, env, firstResult.Children[0].ID)

	second := validSubmission(env.phaseID)
	second.GuardianEmail = "duplicate-approval@example.com"
	second.Children[0].FirstName = "Noah"
	second.Children[0].LastName = "Beispiel"
	_, err = env.svc.Submit(ctx, second)
	require.NoError(t, err)

	svc := newChangeRequestServiceForTest(env)
	proposed := first
	proposed.Children[0].ID = firstResult.Children[0].ID
	proposed.Children[0].FirstName = "Noah"
	created, err := svc.Create(ctx, firstResult.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte Namen korrigieren.",
	})
	require.NoError(t, err)

	_, err = svc.Approve(ctx, created.ChangeRequest.ID, enrollmentService.ReviewChangeRequestInput{
		Note:           "Freigegeben.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDuplicateEnrollment))

	child, err := repositories.NewFactory(env.db).RequestChild.FindByID(ctx, firstResult.Children[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "Lina", child.FirstName)
}

func TestChangeRequestService_Approve_PreservesAdditionalGuardianProfileID(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	phone := "+49 221 555 010"
	req := validSubmission(env.phaseID)
	req.GuardianEmail = "guardian-stamp@example.com"
	req.AdditionalGuardians = []enrollmentService.SubmitGuardian{
		{FirstName: "Opa", LastName: "Schmidt", Phone: &phone},
	}
	result, err := env.svc.Submit(ctx, req)
	require.NoError(t, err)
	enableChangeRequestMode(t, env, result.Children[0].ID)

	repoFactory := repositories.NewFactory(env.db)
	profile := &usersModels.GuardianProfile{
		FirstName:              "Opa",
		LastName:               "Schmidt",
		PreferredContactMethod: "phone",
		LanguagePreference:     "de",
	}
	profile.SetTenantID(1)
	require.NoError(t, repoFactory.GuardianProfile.Create(ctx, profile))
	guardians, err := repoFactory.RequestGuardian.ListByRequestID(ctx, result.Request.ID)
	require.NoError(t, err)
	require.Len(t, guardians, 1)
	require.NoError(t, repoFactory.RequestGuardian.StampResolvedProfile(ctx, guardians[0].ID, profile.ID))

	guardianPhone := "+49 221 555 011"
	proposed := proposedChangeSubmission(t, env, result)
	proposed.GuardianEmail = "attacker@example.com"
	proposed.GuardianPhone = &guardianPhone
	proposed.AdditionalGuardians = req.AdditionalGuardians
	svc := newChangeRequestServiceForTest(env)
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

	guardians, err = repoFactory.RequestGuardian.ListByRequestID(ctx, result.Request.ID)
	require.NoError(t, err)
	require.Len(t, guardians, 1)
	require.NotNil(t, guardians[0].GuardianProfileID)
	assert.Equal(t, profile.ID, *guardians[0].GuardianProfileID)
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

func TestChangeRequestService_Approve_WaitlistsNonApprovedChildMovedOntoFullOffering(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	setPhaseOverflowMode(t, env, enrollmentModels.PhaseCareOverflowWaitlist)
	offering := setupCareOfferingForCapacity(t, env, 1)
	repoFactory := repositories.NewFactory(env.db)

	holderReq := validSubmission(env.phaseID)
	holderReq.GuardianEmail = "holder-change-request-waitlist@example.com"
	holderReq.Children[0].FirstName = "Slot"
	holderReq.Children[0].LastName = "Holder"
	holderReq.Children[0].OfferingIDs = []int64{offering.ID}
	_, err := env.svc.Submit(ctx, holderReq)
	require.NoError(t, err)

	candidateReq := validSubmission(env.phaseID)
	candidateReq.GuardianEmail = "candidate-change-request-waitlist@example.com"
	candidateReq.Children[0].FirstName = "Capacity"
	candidateReq.Children[0].LastName = "Candidate"
	candidate, err := env.svc.Submit(ctx, candidateReq)
	require.NoError(t, err)
	reason := "Bitte pruefen"
	require.NoError(t, repoFactory.RequestChild.UpdateStatus(
		ctx,
		candidate.Children[0].ID,
		enrollmentModels.ChildStatusUnderReview,
		&reason,
		env.creatorID,
	))

	proposed := proposedChangeSubmission(t, env, candidate)
	proposed.Children[0].OfferingIDs = []int64{offering.ID}
	svc := newChangeRequestServiceForTest(env)
	created, err := svc.Create(ctx, candidate.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte Betreuungsangebot wechseln.",
	})
	require.NoError(t, err)

	_, err = svc.Approve(ctx, created.ChangeRequest.ID, enrollmentService.ReviewChangeRequestInput{
		Note:           "Freigegeben.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)

	child, err := repoFactory.RequestChild.FindByID(ctx, candidate.Children[0].ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusWaitlisted, child.Status)
	links, err := repoFactory.RequestChildOffering.ListByRequestChildID(ctx, candidate.Children[0].ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, offering.ID, links[0].CareOfferingID)
}

func TestChangeRequestService_Approve_RejectsNonApprovedChildMovedOntoFullOffering(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	setPhaseOverflowMode(t, env, enrollmentModels.PhaseCareOverflowReject)
	offering := setupCareOfferingForCapacity(t, env, 1)
	repoFactory := repositories.NewFactory(env.db)

	holderReq := validSubmission(env.phaseID)
	holderReq.GuardianEmail = "holder-change-request-reject@example.com"
	holderReq.Children[0].FirstName = "Reject"
	holderReq.Children[0].LastName = "Holder"
	holderReq.Children[0].OfferingIDs = []int64{offering.ID}
	_, err := env.svc.Submit(ctx, holderReq)
	require.NoError(t, err)

	candidateReq := validSubmission(env.phaseID)
	candidateReq.GuardianEmail = "candidate-change-request-reject@example.com"
	candidateReq.Children[0].FirstName = "Reject"
	candidateReq.Children[0].LastName = "Candidate"
	candidate, err := env.svc.Submit(ctx, candidateReq)
	require.NoError(t, err)
	reason := "Bitte pruefen"
	require.NoError(t, repoFactory.RequestChild.UpdateStatus(
		ctx,
		candidate.Children[0].ID,
		enrollmentModels.ChildStatusUnderReview,
		&reason,
		env.creatorID,
	))
	beforeLinks, err := repoFactory.RequestChildOffering.ListByRequestChildID(ctx, candidate.Children[0].ID)
	require.NoError(t, err)
	require.Empty(t, beforeLinks)

	proposed := proposedChangeSubmission(t, env, candidate)
	proposed.Children[0].OfferingIDs = []int64{offering.ID}
	svc := newChangeRequestServiceForTest(env)
	created, err := svc.Create(ctx, candidate.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte Betreuungsangebot wechseln.",
	})
	require.NoError(t, err)

	_, err = svc.Approve(ctx, created.ChangeRequest.ID, enrollmentService.ReviewChangeRequestInput{
		Note:           "Freigegeben.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingFull), "expected ErrCareOfferingFull, got %v", err)

	changeRequest, err := repoFactory.ChangeRequest.FindByID(ctx, created.ChangeRequest.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChangeRequestStatusPendingReview, changeRequest.Status)
	child, err := repoFactory.RequestChild.FindByID(ctx, candidate.Children[0].ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusUnderReview, child.Status)
	afterLinks, err := repoFactory.RequestChildOffering.ListByRequestChildID(ctx, candidate.Children[0].ID)
	require.NoError(t, err)
	assert.Empty(t, afterLinks)
}

func TestChangeRequestService_Approve_RollsBackApprovedChildScheduleReplacementFailure(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	publishDecisionScheduleSchema(t, env, "pickup_times", enrollmentModels.TargetSchedulePickup)
	_, reviewerWithStaffAccountID := createReviewerStaffWithDistinctAccount(t, env)

	reqID, childID := submitOneChildWithCustomData(t, env, "pickup-change-request-rollback@example.com", "Anna", "Rollback", map[string]any{
		"pickup_times": map[string]any{"mon": "14:45"},
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: reviewerWithStaffAccountID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID
	rows, err := env.repos.StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 14, rows[0].PickupTime.Hour())
	assert.Equal(t, 45, rows[0].PickupTime.Minute())

	requestRow, err := env.repos.Request.FindByID(ctx, reqID)
	require.NoError(t, err)
	grade := int16(2)
	proposed := enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Test",
		GuardianEmail:     "ignored@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{
			{
				ID:               childID,
				FirstName:        "Anna",
				LastName:         "Rollback",
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: &grade,
				CustomData: map[string]any{
					"pickup_times": map[string]any{"mon": "16:00"},
				},
			},
		},
	}
	svc := newChangeRequestServiceWithDecisionForTest(t, env)
	created, err := svc.Create(ctx, requestRow.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte Abholzeit anpassen.",
	})
	require.NoError(t, err)
	reviewerWithoutStaff := testpkg.CreateTestAccount(t, env.db, "reviewer-without-staff-change-request")

	_, err = svc.Approve(ctx, created.ChangeRequest.ID, enrollmentService.ReviewChangeRequestInput{
		Note:           "Freigegeben.",
		ActorAccountID: reviewerWithoutStaff.ID,
		ActorRole:      "admin",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "targeted-field replacement sync")

	changeRequest, err := env.repos.ChangeRequest.FindByID(ctx, created.ChangeRequest.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChangeRequestStatusPendingReview, changeRequest.Status)
	rows, err = env.repos.StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, rows, 1, "failed replacement sync must roll back the schedule delete")
	assert.Equal(t, 14, rows[0].PickupTime.Hour())
	assert.Equal(t, 45, rows[0].PickupTime.Minute())
}
