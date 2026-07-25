package enrollment_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/tenant"
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
		StudentRepo:              repoFactory.Student,
		GuardianAuthorizer:       repoFactory.StudentGuardian,
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
	return newChangeRequestServiceWithDecisionAndRepoForTest(t, env, env.repos.ChangeRequest)
}

func newChangeRequestServiceWithDecisionAndRepoForTest(
	t *testing.T,
	env *decisionTestEnv,
	changeRequestRepo enrollmentModels.ChangeRequestRepository,
) enrollmentService.ChangeRequestService {
	t.Helper()
	return enrollmentService.NewChangeRequestService(enrollmentService.ChangeRequestServiceConfig{
		ChangeRequestRepo:        changeRequestRepo,
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
		StudentRepo:              env.repos.Student,
		GuardianAuthorizer:       env.repos.StudentGuardian,
		DecisionService:          changeRequestApplierForTest(t, env),
		Settings:                 env.settings,
		OutboxEnqueuer:           env.outbox,
		FrontendURL:              "http://localhost:3000",
		ParentsURL:               "http://parents.localhost:3000",
		DB:                       env.db,
		Logger:                   slog.Default(),
	})
}

type failAdminAuditChangeRequestRepo struct {
	enrollmentModels.ChangeRequestRepository
}

func (r failAdminAuditChangeRequestRepo) Create(ctx context.Context, row *enrollmentModels.ChangeRequest) error {
	if row.Origin == enrollmentModels.ChangeRequestOriginAdmin {
		return errors.New("forced admin correction audit failure")
	}
	return r.ChangeRequestRepository.Create(ctx, row)
}

func TestNewChangeRequestService_ParentsURLRequired(t *testing.T) {
	assert.PanicsWithValue(t, "PARENTS_URL is required", func() {
		enrollmentService.NewChangeRequestService(enrollmentService.ChangeRequestServiceConfig{
			FrontendURL: "http://localhost:3000",
		})
	})
}

func TestChangeRequestService_CorrectApprovedChildData_RequiresReason(t *testing.T) {
	svc := enrollmentService.NewChangeRequestService(enrollmentService.ChangeRequestServiceConfig{
		ParentsURL: "http://parents.localhost:3000",
	})
	_, err := svc.CorrectApprovedChildData(context.Background(), enrollmentService.CorrectApprovedChildDataInput{
		RequestID:      11,
		ChildID:        12,
		FirstName:      "Lina",
		LastName:       "Muster",
		DateOfBirth:    timezone.NewDate(2018, 4, 15),
		ActorAccountID: 13,
	})
	require.ErrorIs(t, err, enrollmentService.ErrChangeRequestInvalidData)
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
	env.settings.stringValues[configModel.KeyEnrollmentDuplicateHandling] = configModel.EnrollmentDuplicateHandlingBlock

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

func TestChangeRequestService_Create_AllowsKeepingInactiveCurrentOffering(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	repoFactory := repositories.NewFactory(env.db)
	offering := setupCareOfferingForCapacity(t, env, 10)

	req := validSubmission(env.phaseID)
	req.GuardianEmail = "keep-inactive-offering@example.com"
	req.Children[0].OfferingIDs = []int64{offering.ID}
	result, err := env.svc.Submit(ctx, req)
	require.NoError(t, err)
	enableChangeRequestMode(t, env, result.Children[0].ID)

	offering.IsActive = false
	require.NoError(t, repoFactory.CareOffering.Update(ctx, offering))

	phone := "+49 221 222333"
	proposed := proposedChangeSubmission(t, env, result)
	proposed.Children[0].OfferingIDs = []int64{offering.ID}
	proposed.GuardianPhone = &phone
	svc := newChangeRequestServiceForTest(env)
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Telefon korrigieren.",
	})

	require.NoError(t, err)
	require.NotNil(t, created.ChangeRequest)
}

func TestChangeRequestService_Create_PreservesGradeCapabilityForInactiveConditionalOffering(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	repoFactory := repositories.NewFactory(env.db)
	env.settings.boolValues[configModel.KeyEnrollmentCollectGradeLevel] = false
	offering := setupCareOfferingForCapacity(t, env, 10)
	offering.AvailabilityRule = requestTestGradeAvailabilityRule(enrollmentModels.AvailabilityOperatorIn, 1)
	require.NoError(t, repoFactory.CareOffering.Update(ctx, offering))

	req := validSubmission(env.phaseID)
	req.GuardianEmail = "inactive-conditional-change-request@example.com"
	req.Children[0].TargetGradeLevel = testpkg.Int16Ptr(1)
	req.Children[0].OfferingIDs = []int64{offering.ID}
	result, err := env.svc.Submit(ctx, req)
	require.NoError(t, err)
	enableChangeRequestMode(t, env, result.Children[0].ID)

	offering.IsActive = false
	require.NoError(t, repoFactory.CareOffering.Update(ctx, offering))

	phone := "+49 221 222334"
	proposed := proposedChangeSubmission(t, env, result)
	proposed.Children[0].TargetGradeLevel = testpkg.Int16Ptr(1)
	proposed.Children[0].OfferingIDs = []int64{offering.ID}
	proposed.GuardianPhone = &phone
	created, err := newChangeRequestServiceForTest(env).Create(
		ctx,
		result.Request.StatusToken,
		enrollmentService.CreateChangeRequestInput{
			Submission: proposed,
			ParentNote: "Telefon korrigieren.",
		},
	)

	require.NoError(t, err)
	require.NotNil(t, created.ChangeRequest)
}

func TestChangeRequestService_Approve_PreservesHiddenOfferingsAcrossDisabledToEnabledToggle(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	repoFactory := repositories.NewFactory(env.db)
	offering := setupCareOfferingForCapacity(t, env, 10)

	request := validSubmission(env.phaseID)
	request.GuardianEmail = "change-request-disabled-enabled@example.com"
	request.Children[0].OfferingIDs = []int64{offering.ID}
	result, err := env.svc.Submit(ctx, request)
	require.NoError(t, err)
	enableChangeRequestMode(t, env, result.Children[0].ID)

	// The parent cannot submit offering fields while the capability is off.
	// Creation must still freeze the persisted hidden link in both snapshots.
	env.settings.boolValues[configModel.KeyEnrollmentCareOfferingsEnabled] = false
	phone := "+49 221 444555"
	proposed := proposedChangeSubmission(t, env, result)
	proposed.GuardianPhone = &phone
	svc := newChangeRequestServiceForTest(env)
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte Telefonnummer korrigieren.",
	})
	require.NoError(t, err)
	require.False(t, created.ChangeRequest.CareOfferingsEnabledAtCreation)
	proposedChildren, ok := created.ChangeRequest.ProposedSnapshot["children"].([]any)
	require.True(t, ok)
	require.Len(t, proposedChildren, 1)
	proposedChild, ok := proposedChildren[0].(map[string]any)
	require.True(t, ok)
	require.NotEmpty(t, proposedChild["offering_ids"], "hidden persisted links must be part of the frozen proposal")

	// Re-enabling the UI capability must not reinterpret the frozen proposal
	// as an offering removal or make the base snapshot look stale.
	env.settings.boolValues[configModel.KeyEnrollmentCareOfferingsEnabled] = true
	_, err = svc.Approve(ctx, created.ChangeRequest.ID, enrollmentService.ReviewChangeRequestInput{
		Note:           "Freigegeben.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)

	links, err := repoFactory.RequestChildOffering.ListByRequestChildID(ctx, result.Children[0].ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, offering.ID, links[0].CareOfferingID)
}

func TestChangeRequestService_Approve_AppliesPinnedOfferingChangeToApprovedChildAfterDisable(t *testing.T) {
	careOfferingsEnabled := true
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{
		careOfferingsEnabled: &careOfferingsEnabled,
	})
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	currentOffering := createAdjustmentCareOffering(t, env, "Frühbetreuung vor Umschaltung")
	replacementOffering := createAdjustmentCareOffering(t, env, "Spätbetreuung vor Umschaltung")
	grade := int16(2)

	submission := enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Toggle",
		GuardianEmail:     "approved-change-request-toggle@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Lina",
			LastName:         "Toggle",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
			OfferingIDs:      []int64{currentOffering.ID},
			OfferingDays: []enrollmentService.SubmitOfferingDays{{
				OfferingID:   currentOffering.ID,
				SelectedDays: []string{"mon"},
			}},
		}},
	}
	result, err := env.requestSvc.Submit(ctx, submission)
	require.NoError(t, err)
	require.Len(t, result.Children, 1)
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  result.Request.ID,
		ChildID:    result.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	proposed := submission
	proposed.Children[0].ID = result.Children[0].ID
	proposed.Children[0].OfferingIDs = []int64{replacementOffering.ID}
	proposed.Children[0].OfferingDays = []enrollmentService.SubmitOfferingDays{{
		OfferingID:   replacementOffering.ID,
		SelectedDays: []string{"tue"},
	}}
	svc := newChangeRequestServiceWithDecisionForTest(t, env)
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte Betreuungsangebot wechseln.",
	})
	require.NoError(t, err)
	require.True(t, created.ChangeRequest.CareOfferingsEnabledAtCreation)

	careOfferingsEnabled = false
	env.settings.boolValues[configModel.KeyEnrollmentCareOfferingsEnabled] = false
	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      result.Request.ID,
		ChildID:        result.Children[0].ID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Direkte Korrektur muss gesperrt bleiben",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{{
			OfferingID:   currentOffering.ID,
			SelectedDays: []string{"mon"},
		}},
	})
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingsDisabled)

	_, err = svc.Approve(ctx, created.ChangeRequest.ID, enrollmentService.ReviewChangeRequestInput{
		Note:           "Angebotswechsel freigegeben.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)

	links, err := env.repos.RequestChildOffering.ListByRequestChildID(ctx, result.Children[0].ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, replacementOffering.ID, links[0].CareOfferingID)
	assert.Equal(t, []string{"tue"}, links[0].SelectedDays)
	adjustments, err := env.decision.ListOfferingAdjustments(ctx, result.Request.ID, result.Children[0].ID)
	require.NoError(t, err)
	require.Len(t, adjustments, 1)
	assert.Equal(t, "Angebotswechsel freigegeben.", adjustments[0].Reason)
}

func TestChangeRequestService_Approve_ReplacesOffenPlaceholderWithNewGrade(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	env.settings.boolValues[configModel.KeyEnrollmentCollectGradeLevel] = false

	submission := enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Klasse",
		GuardianEmail:     "change-request-offen-grade@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:   "Lina",
			LastName:    "Klasse",
			DateOfBirth: timezone.NewDate(2018, 4, 15),
		}},
	}
	result, err := env.requestSvc.Submit(ctx, submission)
	require.NoError(t, err)
	require.Len(t, result.Children, 1)
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  result.Request.ID,
		ChildID:    result.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	student, err := env.repos.Student.FindByID(ctx, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	require.Equal(t, "offen", student.SchoolClass)

	// Grade collection is enabled later. The parent provides a grade but no
	// concrete class, so the approval sync must replace the neutral placeholder
	// instead of treating it as a customized class name.
	env.settings.boolValues[configModel.KeyEnrollmentCollectGradeLevel] = true
	grade := int16(3)
	proposed := submission
	proposed.Children[0].ID = result.Children[0].ID
	proposed.Children[0].TargetGradeLevel = &grade
	svc := newChangeRequestServiceWithDecisionForTest(t, env)
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Klassenstufe nachreichen.",
	})
	require.NoError(t, err)
	_, err = svc.Approve(ctx, created.ChangeRequest.ID, enrollmentService.ReviewChangeRequestInput{
		Note:           "Klassenstufe freigegeben.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)

	student, err = env.repos.Student.FindByID(ctx, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	assert.Equal(t, "3", student.SchoolClass)
}

func TestChangeRequestService_CorrectApprovedChildData_UpdatesEnrollmentStudentAndAudit(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	gradeOne := int16(1)
	submission := enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Mara",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     "admin-correction@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Lina",
			LastName:         "Falsch",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &gradeOne,
			CustomData:       map[string]any{"allergies": "keine"},
		}},
	}
	result, err := env.requestSvc.Submit(ctx, submission)
	require.NoError(t, err)
	require.Len(t, result.Children, 1)
	storedChild, err := env.repos.RequestChild.FindByID(ctx, result.Children[0].ID)
	require.NoError(t, err)
	storedChild.CustomData = map[string]any{"allergies": "keine"}
	require.NoError(t, env.repos.RequestChild.UpdateData(ctx, storedChild))
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  result.Request.ID,
		ChildID:    result.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	gradeTwo := int16(2)
	correctedDOB := timezone.NewDate(2018, 5, 16)
	svc := newChangeRequestServiceWithDecisionForTest(t, env)
	var audit *enrollmentService.ChangeRequestAggregate
	err = tenant.WithTenantTx(ctx, env.db, 1, func(txCtx context.Context, _ bun.Tx) error {
		var correctionErr error
		audit, correctionErr = svc.CorrectApprovedChildData(txCtx, enrollmentService.CorrectApprovedChildDataInput{
			RequestID:        result.Request.ID,
			ChildID:          result.Children[0].ID,
			FirstName:        "Lina",
			LastName:         "Richtig",
			DateOfBirth:      correctedDOB,
			TargetGradeLevel: &gradeTwo,
			Reason:           "Angaben nach Rücksprache berichtigt",
			ActorAccountID:   env.creatorID,
		})
		return correctionErr
	})
	require.NoError(t, err)
	require.NotNil(t, audit)
	assert.Equal(t, enrollmentModels.ChangeRequestOriginAdmin, audit.ChangeRequest.Origin)
	assert.Equal(t, enrollmentModels.ChangeRequestStatusApproved, audit.ChangeRequest.Status)
	assert.Equal(t, result.Children[0].ID, *audit.ChangeRequest.RequestChildID)
	assert.Contains(t, audit.ChangeRequest.Diff["changed"], "children")

	child, err := env.repos.RequestChild.FindByID(ctx, result.Children[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "Richtig", child.LastName)
	assert.Equal(t, correctedDOB, child.DateOfBirth)
	require.NotNil(t, child.TargetGradeLevel)
	assert.Equal(t, gradeTwo, *child.TargetGradeLevel)
	assert.Equal(t, map[string]any{"allergies": "keine"}, child.CustomData)

	student, err := env.repos.Student.FindByID(ctx, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	assert.Equal(t, "2", student.SchoolClass)
	person, err := env.repos.Person.FindByID(ctx, student.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "Richtig", person.LastName)
	require.NotNil(t, person.Birthday)
	assert.Equal(t, correctedDOB, *person.Birthday)
}

func TestChangeRequestService_CorrectApprovedChildData_RejectsOpenParentChangeRequest(t *testing.T) {
	for _, openStatus := range []string{
		enrollmentModels.ChangeRequestStatusPendingReview,
		enrollmentModels.ChangeRequestStatusNeedsParentResponse,
	} {
		t.Run(openStatus, func(t *testing.T) {
			env, cleanup := setupDecisionTest(t)
			defer cleanup()
			ctx := testpkg.TenantContext(1)
			env.settings.boolValues[configModel.KeyEnrollmentChangeRequestEmailNotificationsEnabled] = true

			submission, result := createApprovedAdminCorrectionFixture(t, env, "open-correction-"+openStatus+"@example.com")
			svc := newChangeRequestServiceWithDecisionForTest(t, env)
			proposed := submission
			proposed.Children = append([]enrollmentService.SubmitChild(nil), submission.Children...)
			proposed.Children[0].ID = result.Children[0].ID
			proposed.Children[0].FirstName = "Lina-Marie"
			created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
				Submission: proposed,
				ParentNote: "Bitte den Vornamen ändern.",
			})
			require.NoError(t, err)
			if openStatus == enrollmentModels.ChangeRequestStatusNeedsParentResponse {
				_, err = svc.AskQuestion(ctx, created.ChangeRequest.ID, enrollmentService.ChangeRequestMessageInput{
					Body:           "Bitte bestätigen Sie die Schreibweise.",
					ActorAccountID: env.creatorID,
				})
				require.NoError(t, err)
			}

			reason := "Nachname anhand der Geburtsurkunde berichtigt"
			expectedNote := "Diese Änderungsanfrage wurde durch eine direkte Korrektur der OGS ersetzt. Grund: " + reason
			var audit *enrollmentService.ChangeRequestAggregate
			err = tenant.WithTenantTx(ctx, env.db, 1, func(txCtx context.Context, _ bun.Tx) error {
				var correctionErr error
				audit, correctionErr = svc.CorrectApprovedChildData(txCtx, enrollmentService.CorrectApprovedChildDataInput{
					RequestID:        result.Request.ID,
					ChildID:          result.Children[0].ID,
					FirstName:        submission.Children[0].FirstName,
					LastName:         "Richtig",
					DateOfBirth:      submission.Children[0].DateOfBirth,
					TargetGradeLevel: submission.Children[0].TargetGradeLevel,
					Reason:           reason,
					ActorAccountID:   env.creatorID,
				})
				return correctionErr
			})
			require.NoError(t, err)
			require.NotNil(t, audit)
			assert.Equal(t, enrollmentModels.ChangeRequestOriginAdmin, audit.ChangeRequest.Origin)

			rejected, err := env.repos.ChangeRequest.FindByID(ctx, created.ChangeRequest.ID)
			require.NoError(t, err)
			assert.Equal(t, enrollmentModels.ChangeRequestStatusRejected, rejected.Status)
			require.NotNil(t, rejected.AdminDecisionNote)
			assert.Equal(t, expectedNote, *rejected.AdminDecisionNote)
			require.NotNil(t, rejected.ReviewedByAccountID)
			assert.Equal(t, env.creatorID, *rejected.ReviewedByAccountID)
			assert.NotNil(t, rejected.ReviewedAt)

			messages, err := env.repos.ChangeRequestMessage.ListByChangeRequestID(ctx, rejected.ID, true)
			require.NoError(t, err)
			assert.Condition(t, func() bool {
				for _, message := range messages {
					if message.AuthorType == enrollmentModels.ChangeRequestMessageAuthorStaff && message.Body == expectedNote {
						return true
					}
				}
				return false
			}, "the rejected request must explain that the direct correction replaced it")

			rejectionEmails := env.outbox.ByKind(platformModels.EmailKindEnrollmentChangeRequestRejected)
			require.Len(t, rejectionEmails, 1)
			assert.Equal(t, result.Request.ID, rejectionEmails[0].RelatedEntityID)

			_, err = svc.Approve(ctx, rejected.ID, enrollmentService.ReviewChangeRequestInput{
				Note:           "Darf nicht mehr freigegeben werden.",
				ActorAccountID: env.creatorID,
				ActorRole:      "admin",
			})
			require.ErrorIs(t, err, enrollmentService.ErrChangeRequestInvalidStatus)
		})
	}
}

func TestChangeRequestService_CorrectApprovedChildData_PreservesTerminalParentChangeRequest(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	submission, result := createApprovedAdminCorrectionFixture(t, env, "terminal-correction@example.com")
	svc := newChangeRequestServiceWithDecisionForTest(t, env)
	proposed := submission
	proposed.Children = append([]enrollmentService.SubmitChild(nil), submission.Children...)
	proposed.Children[0].ID = result.Children[0].ID
	proposed.Children[0].FirstName = "Lina-Marie"
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte den Vornamen ändern.",
	})
	require.NoError(t, err)
	_, err = svc.Reject(ctx, created.ChangeRequest.ID, enrollmentService.ReviewChangeRequestInput{
		Note:           "Die Schreibweise ist bereits korrekt.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)
	before, err := env.repos.ChangeRequest.FindByID(ctx, created.ChangeRequest.ID)
	require.NoError(t, err)

	err = tenant.WithTenantTx(ctx, env.db, 1, func(txCtx context.Context, _ bun.Tx) error {
		_, correctionErr := svc.CorrectApprovedChildData(txCtx, enrollmentService.CorrectApprovedChildDataInput{
			RequestID:        result.Request.ID,
			ChildID:          result.Children[0].ID,
			FirstName:        submission.Children[0].FirstName,
			LastName:         "Richtig",
			DateOfBirth:      submission.Children[0].DateOfBirth,
			TargetGradeLevel: submission.Children[0].TargetGradeLevel,
			Reason:           "Nachname berichtigt",
			ActorAccountID:   env.creatorID,
		})
		return correctionErr
	})
	require.NoError(t, err)

	after, err := env.repos.ChangeRequest.FindByID(ctx, created.ChangeRequest.ID)
	require.NoError(t, err)
	assert.Equal(t, before.Status, after.Status)
	assert.Equal(t, before.AdminDecisionNote, after.AdminDecisionNote)
	assert.Equal(t, before.ReviewedByAccountID, after.ReviewedByAccountID)
	assert.Equal(t, before.ReviewedAt, after.ReviewedAt)
}

func TestChangeRequestService_CorrectApprovedChildData_RollsBackRejectedRequestWhenAuditFails(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	submission, result := createApprovedAdminCorrectionFixture(t, env, "rollback-correction@example.com")
	regularService := newChangeRequestServiceWithDecisionForTest(t, env)
	proposed := submission
	proposed.Children = append([]enrollmentService.SubmitChild(nil), submission.Children...)
	proposed.Children[0].ID = result.Children[0].ID
	proposed.Children[0].FirstName = "Lina-Marie"
	created, err := regularService.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte den Vornamen ändern.",
	})
	require.NoError(t, err)

	storedChild, err := env.repos.RequestChild.FindByID(ctx, result.Children[0].ID)
	require.NoError(t, err)
	require.NotNil(t, storedChild.CreatedStudentID)
	student, err := env.repos.Student.FindByID(ctx, *storedChild.CreatedStudentID)
	require.NoError(t, err)
	personBefore, err := env.repos.Person.FindByID(ctx, student.PersonID)
	require.NoError(t, err)

	failingService := newChangeRequestServiceWithDecisionAndRepoForTest(t, env, failAdminAuditChangeRequestRepo{
		ChangeRequestRepository: env.repos.ChangeRequest,
	})
	err = tenant.WithTenantTx(ctx, env.db, 1, func(txCtx context.Context, _ bun.Tx) error {
		_, correctionErr := failingService.CorrectApprovedChildData(txCtx, enrollmentService.CorrectApprovedChildDataInput{
			RequestID:        result.Request.ID,
			ChildID:          result.Children[0].ID,
			FirstName:        submission.Children[0].FirstName,
			LastName:         "Richtig",
			DateOfBirth:      submission.Children[0].DateOfBirth,
			TargetGradeLevel: submission.Children[0].TargetGradeLevel,
			Reason:           "Nachname berichtigt",
			ActorAccountID:   env.creatorID,
		})
		return correctionErr
	})
	require.ErrorContains(t, err, "forced admin correction audit failure")

	openRequest, err := env.repos.ChangeRequest.FindByID(ctx, created.ChangeRequest.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChangeRequestStatusPendingReview, openRequest.Status)
	assert.Nil(t, openRequest.AdminDecisionNote)
	assert.Nil(t, openRequest.ReviewedByAccountID)
	assert.Nil(t, openRequest.ReviewedAt)

	childAfter, err := env.repos.RequestChild.FindByID(ctx, result.Children[0].ID)
	require.NoError(t, err)
	assert.Equal(t, submission.Children[0].LastName, childAfter.LastName)
	personAfter, err := env.repos.Person.FindByID(ctx, student.PersonID)
	require.NoError(t, err)
	assert.Equal(t, personBefore.LastName, personAfter.LastName)

	messages, err := env.repos.ChangeRequestMessage.ListByChangeRequestID(ctx, openRequest.ID, true)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "Bitte den Vornamen ändern.", messages[0].Body)
	rows, err := env.repos.ChangeRequest.ListByRequestID(ctx, result.Request.ID)
	require.NoError(t, err)
	for _, row := range rows {
		assert.NotEqual(t, enrollmentModels.ChangeRequestOriginAdmin, row.Origin)
	}
}

func createApprovedAdminCorrectionFixture(
	t *testing.T,
	env *decisionTestEnv,
	guardianEmail string,
) (enrollmentService.SubmitRequest, *enrollmentService.SubmitResult) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	grade := int16(1)
	submission := enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Mara",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     guardianEmail,
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Lina",
			LastName:         "Falsch",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
		}},
	}
	result, err := env.requestSvc.Submit(ctx, submission)
	require.NoError(t, err)
	require.Len(t, result.Children, 1)
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  result.Request.ID,
		ChildID:    result.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	return submission, result
}

func TestChangeRequestService_CorrectApprovedChildData_PreservesHistoricalTargetsWhenCollectionIsDisabled(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	env.settings.boolValues[configModel.KeyEnrollmentCollectSchoolClass] = true
	env.sourcePhase.AvailableSchoolClasses = []string{"2a", "2b", "3a"}
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))

	gradeTwo := int16(2)
	targetClass := "2a"
	result, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Mara",
		GuardianLastName:  "Historie",
		GuardianEmail:     "admin-historical-correction@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:         "Lina",
			LastName:          "Alt",
			DateOfBirth:       timezone.NewDate(2018, 4, 15),
			TargetGradeLevel:  &gradeTwo,
			TargetSchoolClass: &targetClass,
		}},
	})
	require.NoError(t, err)
	require.Len(t, result.Children, 1)
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  result.Request.ID,
		ChildID:    result.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	// These current rules no longer describe the historical submission. An
	// unrelated correction must neither revalidate nor erase its target data.
	env.settings.boolValues[configModel.KeyEnrollmentCollectGradeLevel] = false
	env.settings.boolValues[configModel.KeyEnrollmentCollectSchoolClass] = false
	env.settings.intValues[configModel.KeyEnrollmentGradeLevelMax] = 1
	env.sourcePhase.AvailableSchoolClasses = []string{"3a"}
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))

	svc := newChangeRequestServiceWithDecisionForTest(t, env)
	correct := func(input enrollmentService.CorrectApprovedChildDataInput) error {
		return tenant.WithTenantTx(ctx, env.db, 1, func(txCtx context.Context, _ bun.Tx) error {
			_, correctionErr := svc.CorrectApprovedChildData(txCtx, input)
			return correctionErr
		})
	}
	baseInput := enrollmentService.CorrectApprovedChildDataInput{
		RequestID:         result.Request.ID,
		ChildID:           result.Children[0].ID,
		FirstName:         "Lina",
		LastName:          "Richtig",
		DateOfBirth:       timezone.NewDate(2018, 4, 15),
		TargetGradeLevel:  &gradeTwo,
		TargetSchoolClass: &targetClass,
		Reason:            "Nachname berichtigt",
		ActorAccountID:    env.creatorID,
	}
	require.NoError(t, correct(baseInput))

	gradeThree := int16(3)
	changedGrade := baseInput
	changedGrade.TargetGradeLevel = &gradeThree
	changedGrade.Reason = "Klassenstufe ändern"
	require.ErrorIs(t, correct(changedGrade), enrollmentService.ErrChangeRequestInvalidData)

	changedClassValue := "2b"
	changedClass := baseInput
	changedClass.TargetSchoolClass = &changedClassValue
	changedClass.Reason = "Zielklasse ändern"
	require.ErrorIs(t, correct(changedClass), enrollmentService.ErrChangeRequestInvalidData)

	child, err := env.repos.RequestChild.FindByID(ctx, result.Children[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "Richtig", child.LastName)
	require.NotNil(t, child.TargetGradeLevel)
	assert.Equal(t, gradeTwo, *child.TargetGradeLevel)
	require.NotNil(t, child.TargetSchoolClass)
	assert.Equal(t, targetClass, *child.TargetSchoolClass)

	student, err := env.repos.Student.FindByID(ctx, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	assert.Equal(t, targetClass, student.SchoolClass)
	person, err := env.repos.Person.FindByID(ctx, student.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "Richtig", person.LastName)

	changeRequests, err := env.repos.ChangeRequest.ListByRequestID(ctx, result.Request.ID)
	require.NoError(t, err)
	adminAudits := 0
	for _, changeRequest := range changeRequests {
		if changeRequest.Origin == enrollmentModels.ChangeRequestOriginAdmin {
			adminAudits++
		}
	}
	assert.Equal(t, 1, adminAudits, "rejected corrections must not create audit rows")
}

func TestChangeRequestService_Create_RejectsInactiveOfferingOnlyCurrentForAnotherChild(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	repoFactory := repositories.NewFactory(env.db)
	firstOffering := setupCareOfferingForCapacity(t, env, 10)
	secondOffering := setupCareOfferingForCapacity(t, env, 10)
	secondOffering.Name = "Second inactive slot"
	require.NoError(t, repoFactory.CareOffering.Update(ctx, secondOffering))

	req := validSubmission(env.phaseID)
	req.GuardianEmail = "cross-child-inactive-offering@example.com"
	req.Children[0].OfferingIDs = []int64{firstOffering.ID}
	req.Children = append(req.Children, enrollmentService.SubmitChild{
		FirstName:        "Noah",
		LastName:         "Beispiel",
		DateOfBirth:      timezone.NewDate(2017, 8, 3),
		TargetGradeLevel: testpkg.Int16Ptr(2),
		OfferingIDs:      []int64{secondOffering.ID},
	})
	result, err := env.svc.Submit(ctx, req)
	require.NoError(t, err)
	require.Len(t, result.Children, 2)
	enableChangeRequestMode(t, env, result.Children[0].ID)

	secondOffering.IsActive = false
	require.NoError(t, repoFactory.CareOffering.Update(ctx, secondOffering))

	proposed := req
	proposed.Children[0].ID = result.Children[0].ID
	proposed.Children[0].OfferingIDs = []int64{secondOffering.ID}
	proposed.Children[1].ID = result.Children[1].ID
	proposed.Children[1].OfferingIDs = []int64{secondOffering.ID}
	svc := newChangeRequestServiceForTest(env)
	_, err = svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Betreuungsangebot wechseln.",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingClosed), "expected ErrCareOfferingClosed, got %v", err)
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
	digests := env.outbox.ByKind(platformModels.EmailKindEnrollmentDecisionDigest)
	require.Len(t, digests, 1)
	assert.Equal(t, []string{child.FirstName + " " + child.LastName}, digests[0].Payload["waitlisted_names"])
}

func TestChangeRequestService_Approve_DoesNotDoubleCountPreservedOfferingCapacity(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	setPhaseOverflowMode(t, env, enrollmentModels.PhaseCareOverflowWaitlist)
	offering := setupCareOfferingForCapacity(t, env, 2)
	repoFactory := repositories.NewFactory(env.db)

	req := validSubmission(env.phaseID)
	req.GuardianEmail = "preserved-capacity-change-request@example.com"
	req.Children[0].OfferingIDs = []int64{offering.ID}
	req.Children = append(req.Children, enrollmentService.SubmitChild{
		FirstName:        "Noah",
		LastName:         "Beispiel",
		DateOfBirth:      timezone.NewDate(2017, 8, 3),
		TargetGradeLevel: testpkg.Int16Ptr(2),
	})
	result, err := env.svc.Submit(ctx, req)
	require.NoError(t, err)
	require.Len(t, result.Children, 2)
	reason := "Bitte pruefen"
	require.NoError(t, repoFactory.RequestChild.UpdateStatus(
		ctx,
		result.Children[0].ID,
		enrollmentModels.ChildStatusUnderReview,
		&reason,
		env.creatorID,
	))

	proposed := req
	proposed.Children[0].ID = result.Children[0].ID
	proposed.Children[0].OfferingIDs = []int64{offering.ID}
	proposed.Children[1].ID = result.Children[1].ID
	proposed.Children[1].OfferingIDs = []int64{offering.ID}
	svc := newChangeRequestServiceForTest(env)
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Zweites Kind soll ebenfalls das Angebot nutzen.",
	})
	require.NoError(t, err)

	_, err = svc.Approve(ctx, created.ChangeRequest.ID, enrollmentService.ReviewChangeRequestInput{
		Note:           "Freigegeben.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)

	for _, child := range result.Children {
		stored, err := repoFactory.RequestChild.FindByID(ctx, child.ID)
		require.NoError(t, err)
		assert.NotEqual(t, enrollmentModels.ChildStatusWaitlisted, stored.Status)
		links, err := repoFactory.RequestChildOffering.ListByRequestChildID(ctx, child.ID)
		require.NoError(t, err)
		require.Len(t, links, 1)
		assert.Equal(t, offering.ID, links[0].CareOfferingID)
	}
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
