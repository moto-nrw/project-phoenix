package enrollment_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Unread enrollments (#3778), pinned at the HTTP seam: the assembled router
// with the real middleware chain, the real owner and the real SQL. Read state
// is per account and per school.

type unreadEnv struct {
	db       *bun.DB
	router   http.Handler
	tenantID int64
	phaseID  int64
	repos    repositories.EnrollmentTestRepositories
	requests enrollmentAPI.RequestService
}

func setupUnreadTest(t *testing.T) *unreadEnv {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.Ctx(t)
	repos, err := repositories.NewEnrollmentTestRepositories(db, repositories.NewTestAuditStore(db))
	require.NoError(t, err)
	settings := stubTakeoverSettings{}

	_, author := testpkg.CreateTestPersonWithAccount(t, db, "Uta", "Ungelesen")
	schema, err := enrollmentAPI.NewTestFormSchemas(repos.Enrollment()).CreateSchema(ctx, "Testformular "+t.Name(), []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
	}, author.ID)
	require.NoError(t, err)
	phase := &capability.Phase{
		Name:             "unread-" + t.Name(),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: capability.Date(timezone.NewDate(2026, 9, 1)),
		ServiceEndDate:   capability.Date(timezone.NewDate(2027, 7, 31)),
		IsActive:         true,
		FormSchemaID:     &schema.ID,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	phase.TenantID = tenantID
	require.NoError(t, repos.Enrollment().InsertPhase(ctx, phase))

	offerings := testutil.NewEnrollmentCareOfferingRecords(repos.CarePlan)
	requests := enrollmentAPI.NewRequestService(testutil.NewEnrollmentIntake(testutil.EnrollmentIntakeSources{
		Requests:         repos.Enrollment(),
		ParentChanges:    repos.Enrollment(),
		Children:         repos.Enrollment(),
		Guardians:        repos.Enrollment(),
		CareOfferingRepo: offerings,
		Capacity:         testutil.NewEnrollmentOfferingCapacity(offerings, repos.Enrollment(), settings),
		Catalog:          repos.Enrollment(),
		SchoolRepo:       capabilitySchools{schools: repos.School},
		Notifications:    enrollmentAPI.NewTestNotifications(repos.Enrollment(), notifyModeSettings{settings: settings}, discardingOutbox{}, capabilitySchools{schools: repos.School}),
		RateLimitRepo:    repos.Enrollment(),
		OutboxEnqueuer:   discardingOutbox{},
		Settings:         settings,
		FrontendURL:      "http://localhost:3000",
		ParentsURL:       "http://parents.localhost:3000",
		Logger:           slog.Default(),
	}))
	decisions := enrollmentAPI.NewDecisionService(testutil.PublicEnrollmentDecisions(testutil.NewEnrollmentDecisions(testutil.EnrollmentDecisionSources{
		Requests: repos.Enrollment(), Children: repos.Enrollment(),
		Guardians: repos.Enrollment(), LateInvites: repos.Enrollment(), CareOfferings: offerings,
		Phases: repos.Enrollment(), Schemas: repos.Enrollment(), Settings: settings,
		ParentsURL: "http://parents.localhost:3000", Logger: slog.Default(),
	})))
	resource := enrollmentAPI.NewResource(
		nil, nil, requests, nil, nil, decisions, nil, nil, nil,
		nil, enrollmentAPI.GuardianInvitationRuntime{}, nil, nil, db,
	)
	return &unreadEnv{
		db: db, router: testpkg.TenantRuntimeMiddleware(t, db)(resource.Router()),
		tenantID: tenantID, phaseID: phase.ID, repos: repos, requests: requests,
	}
}

// submit files one enrollment with the given number of children.
func (env *unreadEnv) submit(t *testing.T, children int) *enrollmentAPI.SubmitResult {
	t.Helper()
	req := enrollmentAPI.SubmitRequest{
		TenantID:          env.tenantID,
		PhaseID:           env.phaseID,
		GuardianFirstName: "Ulla",
		GuardianLastName:  "Umbach",
		GuardianEmail:     "unread-" + strconv.Itoa(children) + "@example.com",
		ConsentFlags:      map[string]any{"agb": true, "data_processing": true, "email_contact": true, "photo": false},
	}
	for i := range children {
		req.Children = append(req.Children, enrollmentAPI.SubmitChild{
			FirstName: "Kind" + strconv.Itoa(i), LastName: "Umbach",
			DateOfBirth: timezone.NewDate(2018, 4, 15+i), TargetGradeLevel: testpkg.Int16Ptr(1),
		})
	}
	out, err := env.requests.Submit(testpkg.Ctx(t), req)
	require.NoError(t, err)
	require.Len(t, out.Children, children)
	return out
}

// reader mints a session for a new staff account of the school.
func (env *unreadEnv) reader(t *testing.T, tenantID int64, permissions ...string) string {
	t.Helper()
	_, account := testpkg.CreateTestPersonWithAccount(t, env.db, "Lea", "Leser")
	return testutil.MintTestJWT(t, jwt.AppClaims{
		ID: int(account.ID), Sub: account.Email, Username: "leser" + strconv.FormatInt(account.ID, 10),
		Roles: []string{"admin"}, Permissions: permissions, TenantID: tenantID,
	})
}

func (env *unreadEnv) do(t *testing.T, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

func (env *unreadEnv) count(t *testing.T, token string) int {
	t.Helper()
	rec := env.do(t, http.MethodGet, "/admin/requests/unread-count", token, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Data struct {
			UnreadCount int `json:"unread_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Data.UnreadCount
}

func (env *unreadEnv) listUnread(t *testing.T, token string) map[string]bool {
	t.Helper()
	rec := env.do(t, http.MethodGet, "/admin/requests/?phase_id="+strconv.FormatInt(env.phaseID, 10), token, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Data []struct {
			ID       string `json:"id"`
			IsUnread bool   `json:"is_unread"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	out := map[string]bool{}
	for _, row := range body.Data {
		out[row.ID] = row.IsUnread
	}
	return out
}

func idPath(id int64, suffix string) string {
	return "/admin/requests/" + strconv.FormatInt(id, 10) + suffix
}

var managePermissions = []string{"config:manage", "config:read"}

func TestUnreadEnrollments_CountPerRequestAndPerAccount(t *testing.T) {
	t.Parallel()
	env := setupUnreadTest(t)
	anna, ben := env.reader(t, env.tenantID, managePermissions...), env.reader(t, env.tenantID, managePermissions...)

	twoChildren := env.submit(t, 2)
	assert.Equal(t, 1, env.count(t, anna), "an enrollment with two children counts once")
	assert.Equal(t, 1, env.count(t, ben))

	rec := env.do(t, http.MethodGet, idPath(twoChildren.Request.ID, "/"), anna, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 0, env.count(t, anna), "opening the detail reads it")
	assert.Equal(t, 1, env.count(t, ben), "reading is personal")

	id := strconv.FormatInt(twoChildren.Request.ID, 10)
	assert.False(t, env.listUnread(t, anna)[id])
	assert.True(t, env.listUnread(t, ben)[id])
}

func TestUnreadEnrollments_MarkReadUnreadAndAll(t *testing.T) {
	t.Parallel()
	env := setupUnreadTest(t)
	anna := env.reader(t, env.tenantID, managePermissions...)
	first, second := env.submit(t, 1), env.submit(t, 1)
	require.Equal(t, 2, env.count(t, anna))

	require.Equal(t, http.StatusNoContent, env.do(t, http.MethodPut, idPath(first.Request.ID, "/read"), anna, "").Code)
	assert.Equal(t, 1, env.count(t, anna))
	require.Equal(t, http.StatusNoContent, env.do(t, http.MethodDelete, idPath(first.Request.ID, "/read"), anna, "").Code)
	assert.Equal(t, 2, env.count(t, anna))

	require.Equal(t, http.StatusNoContent, env.do(t, http.MethodPost, "/admin/requests/mark-all-read", anna, "").Code)
	assert.Equal(t, 0, env.count(t, anna))
	assert.False(t, env.listUnread(t, anna)[strconv.FormatInt(second.Request.ID, 10)])
}

func TestUnreadEnrollments_ParentEditResetsButDecisionDoesNot(t *testing.T) {
	t.Parallel()
	env := setupUnreadTest(t)
	anna := env.reader(t, env.tenantID, managePermissions...)
	submitted := env.submit(t, 2)

	require.Equal(t, http.StatusNoContent, env.do(t, http.MethodPut, idPath(submitted.Request.ID, "/read"), anna, "").Code)
	require.Equal(t, 0, env.count(t, anna))

	// An OGS decision on one child moves no parent-change stamp.
	require.NoError(t, env.repos.Enrollment().UpdateChildStatus(testpkg.Ctx(t), submitted.Children[0].ID,
		enrollmentModels.ChildStatusWaitlisted, nil, 0))
	assert.Equal(t, 0, env.count(t, anna), "an OGS decision keeps the enrollment read")

	// Parent edits are accepted only while every child is submitted.
	require.NoError(t, env.repos.Enrollment().UpdateChildStatus(testpkg.Ctx(t), submitted.Children[0].ID,
		enrollmentModels.ChildStatusSubmitted, nil, 0))
	rec := env.do(t, http.MethodPatch, "/requests/"+submitted.Request.StatusToken, "", `{"guardian_first_name":"Ulrike"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, env.count(t, anna), "a parent edit makes it unread again")
}

func TestUnreadEnrollments_RenewalConfirmationResets(t *testing.T) {
	t.Parallel()
	env := setupUnreadTest(t)
	anna := env.reader(t, env.tenantID, managePermissions...)
	submitted := env.submit(t, 1)
	require.NoError(t, env.repos.Enrollment().UpdateChildStatus(testpkg.Ctx(t), submitted.Children[0].ID,
		enrollmentModels.ChildStatusPendingRenewal, nil, 0))
	require.Equal(t, http.StatusNoContent, env.do(t, http.MethodPut, idPath(submitted.Request.ID, "/read"), anna, "").Code)
	require.Equal(t, 0, env.count(t, anna))

	rec := env.do(t, http.MethodPost, "/requests/"+submitted.Request.StatusToken+"/confirm-renewal", "", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, env.count(t, anna), "a confirmed renewal makes it unread again")
}

func TestUnreadEnrollments_TerminalChildrenAndInactivePhaseDoNotCount(t *testing.T) {
	t.Parallel()
	env := setupUnreadTest(t)
	anna := env.reader(t, env.tenantID, managePermissions...)
	closed, open := env.submit(t, 2), env.submit(t, 1)
	ctx := testpkg.Ctx(t)
	require.NoError(t, env.repos.Enrollment().UpdateChildStatus(ctx, closed.Children[0].ID, enrollmentModels.ChildStatusRejected, nil, 0))
	require.Equal(t, 2, env.count(t, anna), "one open child keeps the enrollment unread")
	require.NoError(t, env.repos.Enrollment().UpdateChildStatus(ctx, closed.Children[1].ID, enrollmentModels.ChildStatusWithdrawn, nil, 0))
	assert.Equal(t, 1, env.count(t, anna), "only terminal children: not counted")
	assert.False(t, env.listUnread(t, anna)[strconv.FormatInt(closed.Request.ID, 10)])

	_, err := env.db.NewUpdate().TableExpr("enrollment.phases").Set("is_active = false").
		Where("id = ?", env.phaseID).Exec(testpkg.TenantContext(env.tenantID))
	require.NoError(t, err)
	assert.Equal(t, 0, env.count(t, anna), "inactive phase: not counted")
	_ = open
}

func TestUnreadEnrollments_ReadOnlyPreviewDoesNotRead(t *testing.T) {
	t.Parallel()
	env := setupUnreadTest(t)
	submitted := env.submit(t, 1)
	_, account := testpkg.CreateTestPersonWithAccount(t, env.db, "Pia", "Preview")
	claims := jwt.AppClaims{
		ID: int(account.ID), Sub: account.Email, Username: "preview" + strconv.FormatInt(account.ID, 10),
		Roles: []string{"admin"}, Permissions: managePermissions, TenantID: env.tenantID,
	}
	owner := testutil.MintTestJWT(t, claims)
	claims.ReadOnly = true
	preview := testutil.MintTestJWT(t, claims)

	rec := env.do(t, http.MethodGet, idPath(submitted.Request.ID, "/"), preview, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, env.count(t, owner), "a read-only preview leaves the enrollment unread")
}

func TestUnreadEnrollments_RequiresConfigManage(t *testing.T) {
	t.Parallel()
	env := setupUnreadTest(t)
	submitted := env.submit(t, 1)
	readOnly := env.reader(t, env.tenantID, "config:read")

	for _, call := range []struct{ method, path string }{
		{http.MethodGet, "/admin/requests/unread-count"},
		{http.MethodPost, "/admin/requests/mark-all-read"},
		{http.MethodPut, idPath(submitted.Request.ID, "/read")},
		{http.MethodDelete, idPath(submitted.Request.ID, "/read")},
	} {
		rec := env.do(t, call.method, call.path, readOnly, "")
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s", call.method, call.path)
	}
}

func TestUnreadEnrollments_TenantIsolation(t *testing.T) {
	t.Parallel()
	env := setupUnreadTest(t)
	submitted := env.submit(t, 1)
	otherTenant, _ := testpkg.CreateTestTenant(t, env.db)
	outsider := env.reader(t, otherTenant, managePermissions...)

	assert.Equal(t, 0, env.count(t, outsider), "another school's enrollments never count")
	require.Equal(t, http.StatusNoContent, env.do(t, http.MethodPut, idPath(submitted.Request.ID, "/read"), outsider, "").Code)

	rows, err := env.db.NewSelect().TableExpr("enrollment.request_reads").
		Where("request_id = ?", submitted.Request.ID).Count(testpkg.TenantContext(env.tenantID))
	require.NoError(t, err)
	assert.Zero(t, rows, "marking a foreign id writes no read row")
}

func TestSubmit_AdminNotificationLinksToAdminDetail(t *testing.T) {
	t.Parallel()
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	env.settings.stringValues[configModel.KeyEnrollmentNotificationEmails] = "admin@example.com"

	result, err := env.svc.Submit(testpkg.Ctx(t), validSubmission(t, env.phaseID))
	require.NoError(t, err)

	admins := env.outbox.ByKind("enrollment_admin_notification")
	require.Len(t, admins, 1)
	assert.Equal(t, "http://localhost:3000/admin/enrollments/"+strconv.FormatInt(result.Request.ID, 10),
		admins[0].Payload[capability.EnrollmentPayloadAdminURL])
}

// The admin list reads the caller's read state in one batched statement, no
// matter how many enrollments it shows (#2940, #3778). The rest of the list
// assembly loads per enrollment and is outside this budget.
func TestAdminRequestsList_QueryBudget(t *testing.T) {
	t.Parallel()
	env := setupUnreadTest(t)
	anna := env.reader(t, env.tenantID, managePermissions...)
	counter := testpkg.CaptureQueriesForContext(t, env.db)
	run := func() []string {
		counter.Reset()
		req := httptest.NewRequest(http.MethodGet, "/admin/requests/?phase_id="+strconv.FormatInt(env.phaseID, 10), nil)
		req = req.WithContext(counter.Context(req.Context()))
		req.Header.Set("Authorization", "Bearer "+anna)
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Matching(func(query string) bool { return strings.Contains(query, "enrollment.request_reads") })
	}

	env.submit(t, 1)
	small := run()
	for range 3 {
		env.submit(t, 2)
	}
	large := run()

	require.Len(t, small, 1, "one read-state statement for one enrollment")
	require.Len(t, large, 1, "one read-state statement for four enrollments")
	testpkg.AssertQueryBudget(t, "api.enrollment.admin_requests.read_state", large)
}
