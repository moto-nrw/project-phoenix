package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	request  *enrollmentAPI.SubmitResult
	students []*usersModels.Student
}

// stubTakeoverSettings answers the enrollment settings the public status and
// change-request paths resolve. Values mirror the registry defaults a tenant
// with enrollment switched on and offerings switched off would resolve.
type stubTakeoverSettings struct{}

type staticGuardianInvitationAvailability struct{ redeemable bool }

func (a staticGuardianInvitationAvailability) HasRedeemableGuardianInvitation(context.Context, int64) (bool, error) {
	return a.redeemable, nil
}

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

// notifyModeSettings reads the decision notification mode from a suite's
// settings double, the way the root binds the parent notifications.
type notifyModeSettings struct {
	settings interface {
		ResolveString(ctx context.Context, key string) (string, error)
	}
}

func (s notifyModeSettings) NotifyPerDecision(ctx context.Context) (string, error) {
	return s.settings.ResolveString(ctx, configModel.KeyEnrollmentNotifyPerDecision)
}

func setupTakeoverLockTest(t *testing.T, redeemableInvitation ...bool) (*takeoverLockEnv, func()) {
	t.Helper()
	invitationAvailable := true
	if len(redeemableInvitation) > 0 {
		invitationAvailable = redeemableInvitation[0]
	}
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
	schemaSvc := NewTestFormSchemas(repos.Enrollment())
	schema, err := schemaSvc.CreateSchema(ctx, "Testformular "+t.Name(), []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
	}, account.ID)
	require.NoError(t, err)

	phase := &capability.Phase{
		Name:             "takeover-lock-" + t.Name(),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: capability.Date(calendar.NewDate(2026, 9, 1)),
		ServiceEndDate:   capability.Date(calendar.NewDate(2027, 7, 31)),
		IsActive:         true,
		FormSchemaID:     &schema.ID,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	phase.TenantID = tenantID
	require.NoError(t, repos.Enrollment().InsertPhase(ctx, phase))

	requestSvc := enrollmentAPI.NewRequestService(testutil.NewEnrollmentIntake(testutil.EnrollmentIntakeSources{
		Requests:         repos.Enrollment(),
		Children:         repos.Enrollment(),
		Guardians:        repos.Enrollment(),
		CareOfferingRepo: testutil.NewEnrollmentCareOfferingRecords(repos.CarePlan),
		Capacity:         testutil.NewEnrollmentOfferingCapacity(testutil.NewEnrollmentCareOfferingRecords(repos.CarePlan), repos.Enrollment(), settings),
		Catalog:          repos.Enrollment(),
		SchoolRepo:       capabilitySchools{schools: repos.School},
		Notifications:    NewTestNotifications(repos.Enrollment(), notifyModeSettings{settings: settings}, discardingOutbox{}, capabilitySchools{schools: repos.School}),
		RateLimitRepo:    repos.Enrollment(),
		OutboxEnqueuer:   discardingOutbox{},
		Settings:         settings,
		FrontendURL:      "http://localhost:3000",
		ParentsURL:       "http://parents.localhost:3000",
		Logger:           slog.Default(),
	}))
	changeRequestSvc := enrollmentAPI.NewChangeRequestService(testutil.NewEnrollmentChangeRequests(testutil.EnrollmentChangeRequestSources{
		Requests:            repos.Enrollment(),
		Children:            repos.Enrollment(),
		Guardians:           repos.Enrollment(),
		LateInviteRepo:      repos.Enrollment(),
		CareOfferingRepo:    testutil.NewEnrollmentCareOfferingRecords(repos.CarePlan),
		Capacity:            testutil.NewEnrollmentOfferingCapacity(testutil.NewEnrollmentCareOfferingRecords(repos.CarePlan), repos.Enrollment(), settings),
		Catalog:             repos.Enrollment(),
		Notifications:       NewTestNotifications(repos.Enrollment(), notifyModeSettings{settings: settings}, discardingOutbox{}, capabilitySchools{schools: repos.School}),
		GuardianProfileRepo: repos.GuardianProfile,
		GuardianPhoneRepo:   repos.GuardianPhoneNumber,
		GuardianInvitations: staticGuardianInvitationAvailability{redeemable: invitationAvailable},
		StudentRepo:         repos.Student,
		GuardianAuthorizer:  repos.StudentGuardian,
		Settings:            settings,
		OutboxEnqueuer:      discardingOutbox{},
		FrontendURL:         "http://localhost:3000",
		ParentsURL:          "http://parents.localhost:3000",
		Logger:              slog.Default(),
	}))

	resource := enrollmentAPI.NewResource(
		nil, nil, requestSvc, nil, nil, nil, nil, nil, changeRequestSvc,
		nil, enrollmentAPI.GuardianInvitationRuntime{}, nil, nil,
	)

	submitted, err := requestSvc.Submit(ctx, enrollmentAPI.SubmitRequest{
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
		Children: []enrollmentAPI.SubmitChild{
			{
				FirstName:        "Lina",
				LastName:         "Beispiel",
				DateOfBirth:      calendar.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(1),
			},
			{
				FirstName:        "Timo",
				LastName:         "Beispiel",
				DateOfBirth:      calendar.NewDate(2019, 6, 2),
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
			_, _ = db.NewDelete().TableExpr("users.student_profiles").Where("id = ?", student.ID).Exec(bg)
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

// #3742: the status page links a taken-over child to the parent app only when
// the family can log in there. The flag is pinned on the public status
// response of the assembled router.

const takeoverLockGuardianEmail = "takeover-lock@example.com"

type parentAccountEnvelope struct {
	Data struct {
		HasParentAccount   *bool   `json:"has_parent_account"`
		ParentPortalAccess *string `json:"parent_portal_access"`
	} `json:"data"`
}

func (env *takeoverLockEnv) statusParentAccount(t *testing.T) parentAccountEnvelope {
	t.Helper()
	rec := env.get(t, "/requests/"+env.token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out parentAccountEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), rec.Body.String())
	return out
}

func assertParentPortalAccess(t *testing.T, status parentAccountEnvelope, hasAccount bool, access string) {
	t.Helper()
	require.NotNil(t, status.Data.HasParentAccount)
	require.NotNil(t, status.Data.ParentPortalAccess)
	assert.Equal(t, hasAccount, *status.Data.HasParentAccount)
	assert.Equal(t, access, *status.Data.ParentPortalAccess)
}

// insertGuardianProfile stores a guardian profile at a school, linked to an
// account when accountID is set.
func insertGuardianProfile(t *testing.T, db *bun.DB, tenantID int64, email string, accountID *int64) {
	t.Helper()
	profile := &usersModels.GuardianProfile{
		FirstName:              "Anna",
		LastName:               "Beispiel",
		Email:                  &email,
		AccountID:              accountID,
		HasAccount:             accountID != nil,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	profile.SetTenantID(tenantID)
	require.NoError(t, db.NewInsert().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles`).
		Scan(testpkg.TenantContext(tenantID)))
}

func makeParentAccountReachable(t *testing.T, db *bun.DB, accountID, tenantID int64) {
	t.Helper()
	_, err := db.NewRaw("UPDATE auth.accounts SET password_hash = 'test-password-hash' WHERE id = ?", accountID).Exec(context.Background())
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, accountID, tenantID)
	_, err = db.NewRaw(`
		INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		SELECT ?, id, ? FROM auth.roles WHERE name = 'guardian' AND tenant_id IS NULL`,
		accountID, tenantID,
	).Exec(context.Background())
	require.NoError(t, err)
}

func TestPublicStatus_ParentAccountFlagOnlyWhileAChildIsTakenOver(t *testing.T) {
	t.Parallel()
	env, cleanup := setupTakeoverLockTest(t)
	defer cleanup()

	status := env.statusParentAccount(t)
	assert.Nil(t, status.Data.HasParentAccount,
		"without a taken-over child the page needs no parent-app link")
	assert.Nil(t, status.Data.ParentPortalAccess)
}

func TestPublicStatus_ParentAccountFlagFalseWithoutAccount(t *testing.T) {
	t.Parallel()
	env, cleanup := setupTakeoverLockTest(t)
	defer cleanup()
	insertGuardianProfile(t, env.db, env.tenantID, takeoverLockGuardianEmail, nil)
	env.takeOver(t, env.request.Children[0].ID, "Lina")

	assertParentPortalAccess(t, env.statusParentAccount(t), false, "invitation")
}

func TestPublicStatus_ParentAccountFlagContactsOGSWithoutRedeemableInvitation(t *testing.T) {
	t.Parallel()
	env, cleanup := setupTakeoverLockTest(t, false)
	defer cleanup()
	insertGuardianProfile(t, env.db, env.tenantID, takeoverLockGuardianEmail, nil)
	env.takeOver(t, env.request.Children[0].ID, "Lina")

	assertParentPortalAccess(t, env.statusParentAccount(t), false, "contact_ogs")
}

func TestPublicStatus_ParentAccountFlagTrueWithLinkedAccount(t *testing.T) {
	t.Parallel()
	env, cleanup := setupTakeoverLockTest(t)
	defer cleanup()
	account := testpkg.CreateTestAccount(t, env.db, "status-parent-account")
	makeParentAccountReachable(t, env.db, account.ID, env.tenantID)
	insertGuardianProfile(t, env.db, env.tenantID, takeoverLockGuardianEmail, &account.ID)
	env.takeOver(t, env.request.Children[0].ID, "Lina")

	assertParentPortalAccess(t, env.statusParentAccount(t), true, "account")
}

func TestPublicStatus_ParentAccountFlagFalseWithInactiveAccount(t *testing.T) {
	t.Parallel()
	env, cleanup := setupTakeoverLockTest(t)
	defer cleanup()
	account := testpkg.CreateTestAccount(t, env.db, "status-parent-account-inactive")
	makeParentAccountReachable(t, env.db, account.ID, env.tenantID)
	_, err := env.db.NewRaw("UPDATE auth.accounts SET active = false WHERE id = ?", account.ID).Exec(context.Background())
	require.NoError(t, err)
	insertGuardianProfile(t, env.db, env.tenantID, takeoverLockGuardianEmail, &account.ID)
	env.takeOver(t, env.request.Children[0].ID, "Lina")

	assertParentPortalAccess(t, env.statusParentAccount(t), false, "contact_ogs")
}

func TestPublicStatus_ParentAccountFlagFalseWithInactiveTenantMapping(t *testing.T) {
	t.Parallel()
	env, cleanup := setupTakeoverLockTest(t)
	defer cleanup()
	account := testpkg.CreateTestAccount(t, env.db, "status-parent-account-inactive-mapping")
	makeParentAccountReachable(t, env.db, account.ID, env.tenantID)
	_, err := env.db.NewRaw(
		"UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?",
		account.ID,
		env.tenantID,
	).Exec(context.Background())
	require.NoError(t, err)
	insertGuardianProfile(t, env.db, env.tenantID, takeoverLockGuardianEmail, &account.ID)
	env.takeOver(t, env.request.Children[0].ID, "Lina")

	assertParentPortalAccess(t, env.statusParentAccount(t), false, "contact_ogs")
}

func TestPublicStatus_ParentAccountFlagFalseWithoutGuardianRole(t *testing.T) {
	t.Parallel()
	env, cleanup := setupTakeoverLockTest(t)
	defer cleanup()
	account := testpkg.CreateTestAccount(t, env.db, "status-parent-account-no-guardian-role")
	testpkg.EnsureAccountTenant(t, env.db, account.ID, env.tenantID)
	insertGuardianProfile(t, env.db, env.tenantID, takeoverLockGuardianEmail, &account.ID)
	env.takeOver(t, env.request.Children[0].ID, "Lina")

	assertParentPortalAccess(t, env.statusParentAccount(t), false, "contact_ogs")
}

func TestPublicStatus_ParentAccountWithoutPasswordContactsOGS(t *testing.T) {
	t.Parallel()
	env, cleanup := setupTakeoverLockTest(t)
	defer cleanup()
	account := testpkg.CreateTestAccount(t, env.db, "status-parent-account-no-password")
	makeParentAccountReachable(t, env.db, account.ID, env.tenantID)
	_, err := env.db.NewRaw("UPDATE auth.accounts SET password_hash = NULL WHERE id = ?", account.ID).Exec(context.Background())
	require.NoError(t, err)
	insertGuardianProfile(t, env.db, env.tenantID, takeoverLockGuardianEmail, &account.ID)
	env.takeOver(t, env.request.Children[0].ID, "Lina")

	assertParentPortalAccess(t, env.statusParentAccount(t), false, "contact_ogs")
}

// The token lookup runs cross-tenant; the profile lookup must not. An account
// the same address holds at another school does not let the family log in to
// this one.
func TestPublicStatus_ParentAccountFlagIgnoresOtherSchools(t *testing.T) {
	t.Parallel()
	env, cleanup := setupTakeoverLockTest(t)
	defer cleanup()
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, env.db, otherTenant)
	account := testpkg.CreateTestAccount(t, env.db, "status-parent-account-other")
	insertGuardianProfile(t, env.db, otherTenant, takeoverLockGuardianEmail, &account.ID)
	env.takeOver(t, env.request.Children[0].ID, "Lina")

	assertParentPortalAccess(t, env.statusParentAccount(t), false, "invitation")
}
