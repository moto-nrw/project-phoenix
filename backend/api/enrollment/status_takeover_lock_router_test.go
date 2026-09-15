package enrollment_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// ADR 0003 at the public seam: the status-token endpoints are the only surface
// a parent reaches without an account, so the per-child lock is pinned here on
// the HTTP responses of the assembled router.

type takeoverLockEnv struct {
	db       *bun.DB
	router   http.Handler
	tenantID int64
	phaseID  int64
	token    string
	request  *enrollmentService.SubmitResult
	students []*usersModels.Student
}

// stubTakeoverSettings answers the enrollment settings the public status and
// change-request paths resolve. Values mirror the registry defaults a tenant
// with enrollment switched on and offerings switched off would resolve.
type stubTakeoverSettings struct{}

// HasTenantOverride claims every key this stub answers: the Resolve*OrDefault
// helpers fall back to their registry default unless an override exists.
func (stubTakeoverSettings) HasTenantOverride(_ context.Context, key string) (bool, error) {
	return strings.HasPrefix(key, "enrollment."), nil
}

func (stubTakeoverSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	switch key {
	case configModel.KeyEnrollmentEnabled,
		configModel.KeyEnrollmentAllowSubmissionEdit,
		configModel.KeyEnrollmentCollectGradeLevel,
		configModel.KeyEnrollmentWaitlistEnabled:
		return true, nil
	default:
		return false, nil
	}
}

func (stubTakeoverSettings) ResolveString(_ context.Context, key string) (string, error) {
	if key == configModel.KeyEnrollmentDuplicateHandling {
		return configModel.EnrollmentDuplicateHandlingWarn, nil
	}
	return "", nil
}

func (stubTakeoverSettings) ResolveInt(_ context.Context, key string) (int, error) {
	switch key {
	case configModel.KeyEnrollmentGradeLevelMax:
		return 4, nil
	case configModel.KeyEnrollmentStatusTokenTTLDays:
		return 365, nil
	default:
		return 0, nil
	}
}

type discardingOutbox struct{}

func (discardingOutbox) EnqueueOutbox(context.Context, platformModels.OutboxEnqueueRequest) error {
	return nil
}

func setupTakeoverLockTest(t *testing.T) (*takeoverLockEnv, func()) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)
	repos, repoErr := repositories.NewEnrollmentTestRepositories(db, repositories.NewTestAuditStore(db))
	require.NoError(t, repoErr)
	settings := stubTakeoverSettings{}

	account := testpkg.CreateTestAccount(t, db, "takeover-lock")
	person := &usersModels.Person{
		FirstName: "Takeover",
		LastName:  "Tester",
		AccountID: &account.ID,
	}
	person.SetTenantID(tenantID)
	require.NoError(t, db.NewInsert().Model(person).ModelTableExpr("users.persons").Scan(ctx))
	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Owner:  repos.Enrollment(),
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.CreateSchema(ctx, "Testformular "+t.Name(), []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
	}, account.ID)
	require.NoError(t, err)

	phase := &capability.Phase{
		Name:             "takeover-lock-" + t.Name(),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: capability.Date(timezone.NewDate(2026, 9, 1)),
		ServiceEndDate:   capability.Date(timezone.NewDate(2027, 7, 31)),
		IsActive:         true,
		FormSchemaID:     &schema.ID,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	phase.TenantID = tenantID
	require.NoError(t, repos.Enrollment().InsertPhase(ctx, phase))

	requestSvc := enrollmentService.NewRequestService(enrollmentService.RequestServiceConfig{
		Requests:         repos.Enrollment(),
		Children:         repos.Enrollment(),
		Guardians:        repos.Enrollment(),
		CareOfferingRepo: repos.CareOffering,
		Catalog:          repos.Enrollment(),
		SchoolRepo:       repos.School,
		RateLimitRepo:    repos.Enrollment(),
		OutboxEnqueuer:   discardingOutbox{},
		Settings:         settings,
		FrontendURL:      "http://localhost:3000",
		ParentsURL:       "http://parents.localhost:3000",
		DB:               db,
		Logger:           slog.Default(),
	})
	changeRequestSvc := enrollmentService.NewChangeRequestService(enrollmentService.ChangeRequestServiceConfig{
		Requests:            repos.Enrollment(),
		Children:            repos.Enrollment(),
		Guardians:           repos.Enrollment(),
		LateInviteRepo:      repos.Enrollment(),
		CareOfferingRepo:    repos.CareOffering,
		Catalog:             repos.Enrollment(),
		SchoolRepo:          repos.School,
		GuardianProfileRepo: repos.GuardianProfile,
		GuardianPhoneRepo:   repos.GuardianPhoneNumber,
		StudentRepo:         repos.Student,
		GuardianAuthorizer:  repos.StudentGuardian,
		Settings:            settings,
		OutboxEnqueuer:      discardingOutbox{},
		FrontendURL:         "http://localhost:3000",
		ParentsURL:          "http://parents.localhost:3000",
		DB:                  db,
		Logger:              slog.Default(),
	})

	resource := enrollmentAPI.NewResource(
		nil, nil, requestSvc, nil, nil, nil, nil, nil, changeRequestSvc,
		nil, nil, nil, nil, db,
	)

	submitted, err := requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          tenantID,
		PhaseID:           phase.ID,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     "takeover-lock@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           false,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Lina",
				LastName:         "Beispiel",
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(1),
			},
			{
				FirstName:        "Timo",
				LastName:         "Beispiel",
				DateOfBirth:      timezone.NewDate(2019, 6, 2),
				TargetGradeLevel: testpkg.Int16Ptr(1),
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 2)

	env := &takeoverLockEnv{
		db:       db,
		router:   testpkg.TenantRuntimeMiddleware(t, db)(resource.Router()),
		tenantID: tenantID,
		phaseID:  phase.ID,
		token:    submitted.Request.StatusToken,
		request:  submitted,
	}
	cleanup := func() {
		bg := context.Background()
		for _, table := range []string{
			"enrollment.submission_rate_limits",
		} {
			_, _ = db.NewDelete().TableExpr(table).Where("tenant_id = ?", tenantID).Exec(bg)
		}
		_, _ = db.NewDelete().TableExpr("enrollment.requests").Where("phase_id = ?", phase.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("enrollment.phases").Where("id = ?", phase.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("enrollment.form_schemas").Where("created_by = ?", account.ID).Exec(bg)
		for _, student := range env.students {
			_, _ = db.NewDelete().TableExpr("users.students").Where("id = ?", student.ID).Exec(bg)
			_, _ = db.NewDelete().TableExpr("users.persons").Where("id = ?", student.PersonID).Exec(bg)
		}
		_, _ = db.NewDelete().TableExpr("users.persons").Where("id = ?", person.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(bg)
	}
	return env, cleanup
}

// takeOver puts the child into the state an approval leaves behind: approved
// and linked to a real student row.
func (env *takeoverLockEnv) takeOver(t *testing.T, childID int64, firstName string) {
	t.Helper()
	student := testpkg.CreateTestStudentForTenant(t, env.db, env.tenantID, firstName, "Beispiel", "1a")
	env.students = append(env.students, student)
	_, err := env.db.NewUpdate().
		TableExpr("enrollment.request_children").
		Set("status = ?", enrollmentModels.ChildStatusApproved).
		Set("created_student_id = ?", student.ID).
		Where("id = ?", childID).
		Exec(testpkg.TenantContext(env.tenantID))
	require.NoError(t, err)
}

func (env *takeoverLockEnv) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func (env *takeoverLockEnv) postChangeRequest(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/requests/%s/change-requests", env.token),
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

// changeRequestBody builds the wire payload the public form sends: both
// children travel, gradeLina/gradeTimo decide which one the parent changed.
func (env *takeoverLockEnv) changeRequestBody(gradeLina, gradeTimo int) string {
	return fmt.Sprintf(`{
		"phase_id": %d,
		"guardian_first_name": "Anna",
		"guardian_last_name": "Beispiel",
		"guardian_email": "takeover-lock@example.com",
		"consent_flags": {"agb": true, "data_processing": true, "email_contact": true, "photo": false},
		"children": [
			{"id": "%d", "first_name": "Lina", "last_name": "Beispiel", "date_of_birth": "2018-04-15", "target_grade_level": %d},
			{"id": "%d", "first_name": "Timo", "last_name": "Beispiel", "date_of_birth": "2019-06-02", "target_grade_level": %d}
		],
		"parent_note": "Bitte Jahrgang aendern."
	}`, env.phaseID, env.request.Children[0].ID, gradeLina, env.request.Children[1].ID, gradeTimo)
}

type statusEnvelope struct {
	Data struct {
		EditMode string `json:"edit_mode"`
		Children []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Locked bool   `json:"locked"`
		} `json:"children"`
	} `json:"data"`
}

func decodeStatus(t *testing.T, rec *httptest.ResponseRecorder) statusEnvelope {
	t.Helper()
	var out statusEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), rec.Body.String())
	return out
}

func TestPublicStatus_TakenOverChildIsLockedAndSiblingStaysChangeable(t *testing.T) {
	t.Parallel()
	env, cleanup := setupTakeoverLockTest(t)
	defer cleanup()
	env.takeOver(t, env.request.Children[0].ID, "Lina")

	rec := env.get(t, "/requests/"+env.token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	status := decodeStatus(t, rec)
	require.Len(t, status.Data.Children, 2)
	assert.True(t, status.Data.Children[0].Locked, "the taken-over child is locked")
	assert.False(t, status.Data.Children[1].Locked, "the sibling under review is not")
	assert.Equal(t, "change_request", status.Data.EditMode,
		"the form stays reachable for the sibling")

	// The form itself still opens, with the lock marked per child.
	bootstrap := env.get(t, "/requests/"+env.token+"/edit-bootstrap")
	require.Equal(t, http.StatusOK, bootstrap.Code, bootstrap.Body.String())
	assert.Contains(t, bootstrap.Body.String(), `"locked":true`)

	// Changing the locked child is refused with a stable code…
	locked := env.postChangeRequest(t, env.changeRequestBody(2, 1))
	assert.Equal(t, http.StatusForbidden, locked.Code, locked.Body.String())
	assert.Contains(t, locked.Body.String(), "enrollment.change_request_child_locked")

	// …while the same request for the sibling goes through.
	sibling := env.postChangeRequest(t, env.changeRequestBody(1, 2))
	assert.Equal(t, http.StatusCreated, sibling.Code, sibling.Body.String())

	// Token lookup runs under admin scope, then the write switches to the
	// resolved tenant transaction. The approved child stays untouched.
	withdrawReq := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/requests/%s/withdraw", env.token),
		strings.NewReader(`{}`),
	)
	withdrawReq.Header.Set("Content-Type", "application/json")
	withdrawn := httptest.NewRecorder()
	env.router.ServeHTTP(withdrawn, withdrawReq)
	require.Equal(t, http.StatusNoContent, withdrawn.Code, withdrawn.Body.String())
	withdrawnStatus := decodeStatus(t, env.get(t, "/requests/"+env.token))
	assert.Equal(t, enrollmentModels.ChildStatusApproved, withdrawnStatus.Data.Children[0].Status)
	assert.Equal(t, enrollmentModels.ChildStatusWithdrawn, withdrawnStatus.Data.Children[1].Status)
}

func TestPublicStatus_AllChildrenTakenOverLeavesNoChangeForm(t *testing.T) {
	t.Parallel()
	env, cleanup := setupTakeoverLockTest(t)
	defer cleanup()
	env.takeOver(t, env.request.Children[0].ID, "Lina")
	env.takeOver(t, env.request.Children[1].ID, "Timo")

	rec := env.get(t, "/requests/"+env.token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	status := decodeStatus(t, rec)
	require.Len(t, status.Data.Children, 2)
	assert.True(t, status.Data.Children[0].Locked)
	assert.True(t, status.Data.Children[1].Locked)
	assert.Equal(t, "none", status.Data.EditMode,
		"nothing is left to change, so the status link offers no form")

	bootstrap := env.get(t, "/requests/"+env.token+"/edit-bootstrap")
	assert.Equal(t, http.StatusForbidden, bootstrap.Code, bootstrap.Body.String())

	created := env.postChangeRequest(t, env.changeRequestBody(2, 2))
	assert.Equal(t, http.StatusForbidden, created.Code, created.Body.String())
}
