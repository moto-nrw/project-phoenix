package staff_test

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	staffAPI "github.com/moto-nrw/project-phoenix/api/staff"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults before any test (and before setupStaffRoute
// constructs a Resource via jwt.MustNewTokenAuth). CI runs without a .env so
// AUTH_JWT_SECRET is unset; without a secret jwx refuses HMAC signing.
func init() {
	testutil.SeedTestJWTConfig()
}

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	resource *staffAPI.Resource
	router   chi.Router
}

// setupStaffRoute initializes test database, services, and resource. The
// router serves the resource through the production middleware chain
// (Verifier → Authenticator → TenantMiddleware → RequiresPermission →
// TenantTxMiddleware) exactly as the real server does, mounted at /staff.
func setupStaffRoute(t *testing.T, clocks ...func() time.Time) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t, clocks...)

	resource := staffAPI.NewResource(svc.Users, svc.StaffDocuments, svc.StaffOffboarding, svc.Education, svc.Auth, svc.WorkSession, svc.StaffAbsence, svc.WorkTimeMonth, svc.StaffBalanceAdjust, svc.StaffMonthClose, svc.StaffOverview, svc.TimeTrackingAuditLog, svc.StaffTimeExport, db, slog.Default())

	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/staff", resource.Router())

	return &testContext{
		db:       db,
		resource: resource,
		router:   router,
	}
}

// authToken mints a bearer token from default admin claims narrowed to the
// given permissions. It replaces the old WithClaims(DefaultTestClaims) +
// WithPermissions(perms) pairing now that requests flow through Router() and
// the middleware chain reads permissions from the signed token.
func authToken(t *testing.T, perms ...string) string {
	t.Helper()
	claims := testutil.DefaultTestClaims()
	claims.Permissions = perms
	return testutil.MintTestJWT(t, claims)
}

// deleteStaffAuthToken creates the real account-to-staff chain required by
// deleteStaff to identify the actor for audit tombstones.
func deleteStaffAuthToken(t *testing.T, db *bun.DB) string {
	t.Helper()

	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Delete", "Actor")

	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.Username = account.Email
	claims.Permissions = []string{"users:delete"}
	return testutil.MintTestJWT(t, claims)
}

// pinToken mints a bearer token for a PIN-endpoint test. The /pin routes carry
// no permission guard, but the JWT middleware still requires id + sub + roles,
// so we start from DefaultTestClaims and only override the account id/username.
// An accountID of 0 yields a token whose id claim is omitted, which the
// middleware rejects with 401 (used by the InvalidToken cases).
func pinToken(t *testing.T, accountID int, username string) string {
	t.Helper()
	claims := testutil.DefaultTestClaims()
	claims.ID = accountID
	claims.Username = username
	claims.Permissions = nil
	return testutil.MintTestJWT(t, claims)
}

func assignSystemRoleToAccount(
	t *testing.T,
	db *bun.DB,
	accountID int64,
	tenantID int64,
	roleName string,
) {
	t.Helper()

	var role authModels.Role
	err := db.NewSelect().
		Model(&role).
		ModelTableExpr(`auth.roles AS "role"`).
		Where(`LOWER("role".name) = LOWER(?)`, roleName).
		Where(`"role".is_system = TRUE`).
		Where(`"role".tenant_id IS NULL`).
		OrderExpr(`"role".id ASC`).
		Limit(1).
		Scan(testpkg.TenantContext(tenantID))
	if err == sql.ErrNoRows {
		role = authModels.Role{
			Name:        roleName,
			Description: "test system role: " + roleName,
			IsSystem:    true,
		}
		err = db.NewInsert().
			Model(&role).
			ModelTableExpr(`auth.roles`).
			Scan(testpkg.TenantContext(tenantID))
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, repositories.NewFactory(db).Role.Delete(context.Background(), role.ID))
		})
	} else {
		require.NoError(t, err)
	}

	accountRole := &authModels.AccountRole{
		AccountID: accountID,
		RoleID:    role.ID,
	}
	accountRole.SetTenantID(tenantID)

	err = db.NewInsert().
		Model(accountRole).
		ModelTableExpr(`auth.account_roles`).
		Scan(testpkg.TenantContext(tenantID))
	require.NoError(t, err)
}

// =============================================================================
// LIST STAFF TESTS
// =============================================================================

func TestListStaff_Success(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// The response should be an array (even if empty)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, "success", response["status"])
}

func TestListStaff_WithTeachersOnlyFilter(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?teachers_only=true", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithTeachersOnlyFilter_IncludesLegacyTeacherAccounts(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Legacy", "TeacherOnly")

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?teachers_only=true", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "response should contain a data array")

	found := false
	for _, item := range data {
		teacherResp, ok := item.(map[string]interface{})
		require.True(t, ok, "teacher response should be an object")
		if teacherResp["id"] == float64(teacher.Staff.ID) {
			found = true
			break
		}
	}
	assert.True(t, found, "expected teachers_only response to include legacy teacher staff ID %d", teacher.Staff.ID)
}

func TestListStaff_WithNameFilter(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?first_name=Test", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithAccountEmail(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create staff with an account so the email-loading path is exercised
	_, _ = testpkg.CreateTestStaffWithAccount(t, ctx.db, "EmailList", "Staff")

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET STAFF TESTS
// =============================================================================

func TestGetStaff_WithAccountEmail(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create staff with an account so the email-loading path is exercised
	staff, _ := testpkg.CreateTestStaffWithAccount(t, ctx.db, "EmailGet", "Staff")

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify the email is present in the response
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "response should contain data")
	person, ok := data["person"].(map[string]interface{})
	require.True(t, ok, "data should contain person")
	email, ok := person["email"].(string)
	require.True(t, ok, "person should contain email string")
	assert.NotEmpty(t, email)
}

func TestGetStaff_Success(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create test staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "GetStaff", "Test")

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaff_AllowsTimeTrackingManage(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "TimeTrackingManager", "Profile")

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "time_tracking:manage")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetFinancialProfile_AllowsStaffFinancial(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Payroll", "Profile")

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/staff/financial-profile/%d", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "staff:financial")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetDocumentProfile_AllowsHealthDocuments(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Health", "Documents")

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/staff/documents-profile/%d", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "staff_documents:health")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaff_RejectsUnrelatedPermission(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "UnrelatedPermission", "Profile")

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "rooms:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
}

func TestGetStaff_NotFound(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/999999", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetStaff_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/invalid", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// TestGetStaff_WorkStatusConsistentWithList covers the regression where the
// detail endpoint returned an empty work_status while the list endpoint
// resolved it correctly, making the location badge flip-flop between views.
func TestGetStaff_WorkStatusConsistentWithList(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Consistent", "Status")

	tenantID := testpkg.Tenant(t)
	tenantCtx := testpkg.TenantContext(tenantID)
	today := timezone.TodayDate()
	session := &active.WorkSession{
		StaffID:     staff.ID,
		Date:        today,
		Status:      active.WorkSessionStatusPresent,
		CheckInTime: time.Now(),
		CreatedBy:   staff.ID,
	}
	session.SetTenantID(tenantID)
	_, err := ctx.db.NewInsert().Model(session).Exec(tenantCtx)
	require.NoError(t, err)

	listReq := testutil.NewAuthenticatedRequest(t, "GET", "/staff", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))
	listRR := testutil.ExecuteRequest(ctx.router, listReq)
	testutil.AssertSuccessResponse(t, listRR, http.StatusOK)

	listResp := testutil.ParseJSONResponse(t, listRR.Body.Bytes())
	listData, ok := listResp["data"].([]interface{})
	require.True(t, ok, "list response should contain data array")

	var listWorkStatus string
	for _, item := range listData {
		entry, _ := item.(map[string]interface{})
		idFloat, _ := entry["id"].(float64)
		if int64(idFloat) == staff.ID {
			if ws, hasWS := entry["work_status"].(string); hasWS {
				listWorkStatus = ws
			}
			break
		}
	}
	require.Equal(t, string(active.WorkSessionStatusPresent), listWorkStatus,
		"list endpoint should expose the open work session as present")

	detailReq := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))
	detailRR := testutil.ExecuteRequest(ctx.router, detailReq)
	testutil.AssertSuccessResponse(t, detailRR, http.StatusOK)

	detailResp := testutil.ParseJSONResponse(t, detailRR.Body.Bytes())
	detailData, ok := detailResp["data"].(map[string]interface{})
	require.True(t, ok, "detail response should contain data object")

	detailWorkStatus, _ := detailData["work_status"].(string)
	assert.Equal(t, listWorkStatus, detailWorkStatus,
		"detail endpoint must report the same work_status as the list endpoint")
}

// =============================================================================
// CREATE STAFF TESTS
// =============================================================================

func TestCreateStaff_Success(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create a person without staff record
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	person := testpkg.CreateTestPerson(t, ctx.db, "NewStaff"+uniqueSuffix, "Person")

	body := map[string]interface{}{
		"person_id":   person.ID,
		"staff_notes": "Test staff notes",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithJWTBearer(authToken(t, "users:create")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Parse response to get staff ID for cleanup
}

func TestCreateStaff_AsTeacher(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create a person without staff record
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	person := testpkg.CreateTestPerson(t, ctx.db, "NewTeacher"+uniqueSuffix, "Person")

	body := map[string]interface{}{
		"person_id":      person.ID,
		"staff_notes":    "Teacher notes",
		"is_teacher":     true,
		"specialization": "Mathematics",
		"role":           "Senior Teacher",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithJWTBearer(authToken(t, "users:create")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Parse response to get staff ID for cleanup
}

// createStaffForAccountPerson posts a staff record for a person that already
// carries an account. It returns the direct (non-role) permissions the account
// holds afterwards plus the created staff ID for the caller to clean up.
func createStaffForAccountPerson(t *testing.T, ctx *testContext, personID, accountID int64) ([]string, int64) {
	t.Helper()

	body := map[string]interface{}{
		"person_id":   personID,
		"staff_notes": "Übergabe nach Unterricht",
		"is_teacher":  false,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithJWTBearer(authToken(t, "users:create")))

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "create staff response should contain a data object")
	rawStaffID, ok := data["id"].(float64)
	require.True(t, ok, "create staff response should contain the staff ID")

	granted, err := ctx.resource.AuthService.GetAccountDirectPermissions(testpkg.Ctx(t), int(accountID))
	require.NoError(t, err)

	names := make([]string, 0, len(granted))
	for _, perm := range granted {
		names = append(names, perm.Name)
	}
	return names, int64(rawStaffID)
}

// A Lehrkraft (#1772) holds class_day:read and nothing else. Granting the staff
// default groups:read would open the tenant-wide group list and every group's
// student names, far beyond the classes assigned to that Lehrkraft.
func TestCreateStaff_LehrkraftDoesNotGetGroupsRead(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	ctx := setupStaffRoute(t)

	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	person, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "Lehrkraft"+uniqueSuffix, "Person")
	testpkg.EnsureAccountTenant(t, ctx.db, account.ID, testpkg.Tenant(t))
	assignSystemRoleToAccount(t, ctx.db, account.ID, testpkg.Tenant(t), "lehrkraft")

	granted, _ := createStaffForAccountPerson(t, ctx, person.ID, account.ID)

	assert.NotContains(t, granted, permissions.GroupsRead,
		"a Lehrkraft account must not be granted groups:read on staff creation")
}

// The mirror image: a staff account without the Lehrkraft role keeps the
// existing default, so the guard above cannot quietly disable the grant.
func TestCreateStaff_PlainStaffKeepsGroupsRead(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	ctx := setupStaffRoute(t)

	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	person, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "PlainStaff"+uniqueSuffix, "Person")
	testpkg.EnsureAccountTenant(t, ctx.db, account.ID, testpkg.Tenant(t))
	assignSystemRoleToAccount(t, ctx.db, account.ID, testpkg.Tenant(t), "user")

	granted, _ := createStaffForAccountPerson(t, ctx, person.ID, account.ID)

	assert.Contains(t, granted, permissions.GroupsRead,
		"a regular staff account keeps the groups:read default")
}

// POST /api/staff is gated on users:create alone, but a person that already
// carries a staff record adopts it — an edit of someone who is already in the
// directory. That write belongs to staff:manage (#2906), so a create-only
// caller is refused (#2222 review).
func TestCreateStaff_AdoptionRequiresUpdatePermission(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Vorhandene", "Kraft")

	body := map[string]interface{}{
		"person_id":   staff.PersonID,
		"staff_notes": "Fremde Notiz",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithJWTBearer(authToken(t, "users:create")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertForbidden(t, rr)

	// And the same request goes through once the caller may update.
	req = testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithJWTBearer(authToken(t, "users:create", "staff:manage")))

	rr = testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

// A Lehrkraft account (#1772) is provisioned without a caregiver profile on
// purpose. The role-assignment paths refuse the combination from the other
// direction; staff creation must not be the way around it (#2222 review).
func TestCreateStaff_LehrkraftRefusesCaregiverProfile(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	ctx := setupStaffRoute(t)

	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	person, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "LehrkraftProfil"+uniqueSuffix, "Person")
	testpkg.EnsureAccountTenant(t, ctx.db, account.ID, testpkg.Tenant(t))
	assignSystemRoleToAccount(t, ctx.db, account.ID, testpkg.Tenant(t), "lehrkraft")

	body := map[string]interface{}{
		"person_id":      person.ID,
		"is_teacher":     true,
		"specialization": "Betreuung",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithJWTBearer(authToken(t, "users:create", "staff:manage")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	assert.Equal(t, http.StatusConflict, rr.Code,
		"a Lehrkraft account may not be given a caregiver profile")

	count, err := ctx.db.NewSelect().
		TableExpr(`users.staff AS s`).
		Join(`JOIN users.teachers AS t ON t.staff_id = s.id`).
		Where(`s.person_id = ?`, person.ID).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count, "nothing may be written for a refused request")
}

func TestCreateStaff_PersonNotFound(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	body := map[string]interface{}{
		"person_id": 999999,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithJWTBearer(authToken(t, "users:create")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

func TestCreateStaff_MissingPersonID(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	body := map[string]interface{}{
		"staff_notes": "Missing person ID",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithJWTBearer(authToken(t, "users:create")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// UPDATE STAFF TESTS
// =============================================================================

func TestUpdateStaff_Success(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create test staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "UpdateStaff", "Test")

	body := map[string]interface{}{
		"person_id":   staff.PersonID,
		"staff_notes": "Updated notes",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithJWTBearer(authToken(t, "staff:manage")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateStaff_NotFound(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	body := map[string]interface{}{
		"person_id":   1,
		"staff_notes": "Should fail",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/999999", body,
		testutil.WithJWTBearer(authToken(t, "staff:manage")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdateStaff_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	body := map[string]interface{}{
		"person_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/invalid", body,
		testutil.WithJWTBearer(authToken(t, "staff:manage")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DELETE STAFF TESTS
// =============================================================================

func TestDeleteStaff_Success(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create test staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "DeleteStaff", "Test")
	t.Cleanup(func() {
		// Offboarding soft-deletes; remove the row for real.
		_, _ = ctx.db.ExecContext(context.Background(), `DELETE FROM users.staff WHERE id = ?`, staff.ID)
		_, _ = ctx.db.ExecContext(context.Background(), `DELETE FROM audit.data_deletions WHERE staff_id = ?`, staff.ID)
		_, _ = ctx.db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, staff.PersonID)
	})

	token := deleteStaffAuthToken(t, ctx.db)
	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithJWTBearer(token))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// The staff row must be soft-deleted (kept for history), not hard-deleted.
	var deletedAt *time.Time
	err := ctx.db.NewSelect().
		TableExpr(`users.staff`).
		ColumnExpr(`deleted_at`).
		Where(`id = ?`, staff.ID).
		Scan(context.Background(), &deletedAt)
	require.NoError(t, err)
	assert.NotNil(t, deletedAt, "delete must soft-delete the staff row")
}

func TestDeleteStaff_NotFound(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	token := deleteStaffAuthToken(t, ctx.db)
	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/staff/999999", nil,
		testutil.WithJWTBearer(token))

	rr := testutil.ExecuteRequest(ctx.router, req)

	// NOTE: The delete operation returns success even for non-existent IDs.
	// This is idempotent behavior - deleting something that doesn't exist is still "successful".
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteStaff_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/staff/invalid", nil,
		testutil.WithJWTBearer(authToken(t, "users:delete")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET STAFF GROUPS TESTS
// =============================================================================

func TestGetStaffGroups_Success(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create a teacher with the full chain (person -> staff -> teacher)
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "GroupsTest", "Teacher")

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/staff/%d/groups", teacher.StaffID), nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaffGroups_NonTeacher(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create regular staff (not a teacher)
	staff := testpkg.CreateTestStaff(t, ctx.db, "NonTeacher", "Staff")

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/staff/%d/groups", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	// Should return success with empty groups array for non-teachers
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaffGroups_NotFound(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/999999/groups", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// GET STAFF SUBSTITUTIONS TESTS
// =============================================================================

func TestGetAvailableStaff_Success(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/available", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET AVAILABLE FOR SUBSTITUTION TESTS
// =============================================================================

func TestGetStaffByRole_Success(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/by-role?role=user", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaffByRole_Teacher_IncludesLegacyTeacherRoleAccounts(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	ctx := setupStaffRoute(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Legacy", "RoleTeacher")
	testpkg.EnsureAccountTenant(t, ctx.db, account.ID, testpkg.Tenant(t))
	assignSystemRoleToAccount(t, ctx.db, account.ID, testpkg.Tenant(t), "teacher")

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/by-role?role=teacher", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "response should contain a data array")

	found := false
	for _, item := range data {
		staffResp, ok := item.(map[string]interface{})
		require.True(t, ok, "staff response should be an object")
		if staffResp["id"] == float64(teacher.Staff.ID) {
			found = true
			break
		}
	}
	assert.True(t, found, "expected role=teacher response to include legacy teacher staff ID %d", teacher.Staff.ID)
}

func TestGetStaffByRole_User_IncludesLegacyTeacherRoleAccounts(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	ctx := setupStaffRoute(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Legacy", "Caregiver")
	testpkg.EnsureAccountTenant(t, ctx.db, account.ID, testpkg.Tenant(t))
	assignSystemRoleToAccount(t, ctx.db, account.ID, testpkg.Tenant(t), "teacher")

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/by-role?role=user", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "response should contain a data array")

	found := false
	for _, item := range data {
		staffResp, ok := item.(map[string]interface{})
		require.True(t, ok, "staff response should be an object")
		if staffResp["id"] == float64(teacher.Staff.ID) {
			found = true
			assert.Equal(t, float64(teacher.ID), staffResp["teacher_id"])
			assert.Equal(t, true, staffResp["is_active_caregiver"])
			break
		}
	}
	assert.True(t, found, "expected role=user caregiver response to include legacy teacher staff ID %d", teacher.Staff.ID)
}

func TestGetStaffByRole_MissingRoleParam(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/by-role", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// PIN STATUS TESTS
// =============================================================================

func TestGetPINStatus_Success(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create a staff with account
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "PINTest", "Staff")

	// Get the person to access the account ID
	person := teacher.Staff.Person
	require.NotNil(t, person)
	require.NotNil(t, person.AccountID)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/pin", nil,
		testutil.WithJWTBearer(pinToken(t, int(*person.AccountID), "pintest")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetPINStatus_InvalidToken(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Request with invalid (zero) user ID
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/pin", nil,
		testutil.WithJWTBearer(pinToken(t, 0, "")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertUnauthorized(t, rr)
}

// =============================================================================
// UPDATE PIN TESTS
// =============================================================================

func TestUpdatePIN_InvalidPINFormat(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create a staff with account
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "UpdatePIN", "Staff")

	person := teacher.Staff.Person
	require.NotNil(t, person)
	require.NotNil(t, person.AccountID)

	// PIN must be exactly 4 digits
	body := map[string]interface{}{
		"new_pin": "123", // Invalid - only 3 digits
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithJWTBearer(pinToken(t, int(*person.AccountID), "updatepin")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePIN_NonDigitPIN(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create a staff with account
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "NonDigit", "PIN")

	person := teacher.Staff.Person
	require.NotNil(t, person)
	require.NotNil(t, person.AccountID)

	// PIN must contain only digits
	body := map[string]interface{}{
		"new_pin": "12ab", // Invalid - contains letters
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithJWTBearer(pinToken(t, int(*person.AccountID), "nondigitpin")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePIN_MissingNewPIN(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create a staff with account
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "MissingPIN", "Test")

	person := teacher.Staff.Person
	require.NotNil(t, person)
	require.NotNil(t, person.AccountID)

	body := map[string]interface{}{}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithJWTBearer(pinToken(t, int(*person.AccountID), "missingpin")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePIN_InvalidToken(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	body := map[string]interface{}{
		"new_pin": "1234",
	}

	// Request with invalid (zero) user ID
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithJWTBearer(pinToken(t, 0, "")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertUnauthorized(t, rr)
}

func TestUpdatePIN_Success_FirstTime(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create a staff with account (no existing PIN)
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "FirstPIN", "Setup")

	person := teacher.Staff.Person
	require.NotNil(t, person)
	require.NotNil(t, person.AccountID)

	body := map[string]interface{}{
		"new_pin": "1234",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/pin", body,
		testutil.WithJWTBearer(pinToken(t, int(*person.AccountID), "firstpinsetup")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// ANWESEND FILTER TESTS (Staff Presence Tracking)
// =============================================================================

func TestListStaff_WithAnwesendFilter(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Test that the anwesend filter works (even if no staff are currently present)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?anwesend=true", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithRoleFilter(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Test filtering by role (the role filter branch)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?role=admin", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithLastNameFilter(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Test filtering by last name
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?last_name=Test", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithCombinedFilters(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Test multiple filters combined
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?first_name=Test&last_name=Staff&teachers_only=true", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// UPDATE STAFF TO TEACHER TESTS (Teacher Record Update Paths)
// =============================================================================

func TestUpdateStaff_ConvertToTeacher(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create regular staff (not a teacher)
	staff := testpkg.CreateTestStaff(t, ctx.db, "ConvertTo", "Teacher")

	// Update to convert to teacher
	body := map[string]interface{}{
		"person_id":      staff.PersonID,
		"staff_notes":    "Converting to teacher",
		"is_teacher":     true,
		"specialization": "Mathematics",
		"role":           "Teacher",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithJWTBearer(authToken(t, "staff:manage")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify it's now a teacher response
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if data, ok := response["data"].(map[string]interface{}); ok {
		// Check for teacher_id field which indicates it's a teacher response
		_, hasTeacherID := data["teacher_id"]
		assert.True(t, hasTeacherID || data["is_teacher"] == true)
	}
}

func TestUpdateStaff_UpdateExistingTeacher(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create a teacher with the full chain
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "UpdateTeacher", "Test")

	// Update teacher fields
	body := map[string]interface{}{
		"person_id":      teacher.Staff.PersonID,
		"staff_notes":    "Updated teacher notes",
		"is_teacher":     true,
		"specialization": "Physics", // Changed specialization
		"role":           "Senior Teacher",
		"qualifications": "Ph.D. Physics",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", teacher.StaffID), body,
		testutil.WithJWTBearer(authToken(t, "staff:manage")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateStaff_KeepExistingTeacherWithoutIsTeacher(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create a teacher
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "KeepTeacher", "Test")

	// Update without is_teacher flag - should still return teacher response
	body := map[string]interface{}{
		"person_id":   teacher.Staff.PersonID,
		"staff_notes": "Updated notes only",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", teacher.StaffID), body,
		testutil.WithJWTBearer(authToken(t, "staff:manage")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateStaff_InvalidRequest(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "InvalidReq", "Staff")

	// Invalid request - missing person_id
	body := map[string]interface{}{
		"staff_notes": "Missing person ID",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithJWTBearer(authToken(t, "staff:manage")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdateStaff_ChangePersonID(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create two persons
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	person1 := testpkg.CreateTestPerson(t, ctx.db, "Original"+uniqueSuffix, "Person")
	person2 := testpkg.CreateTestPerson(t, ctx.db, "New"+uniqueSuffix, "Person")

	// Create staff with person1
	staff := testpkg.CreateTestStaffForPerson(t, ctx.db, person1.ID)

	// Update to point to person2
	body := map[string]interface{}{
		"person_id":   person2.ID,
		"staff_notes": "Changed person",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithJWTBearer(authToken(t, "staff:manage")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateStaff_ChangeToNonExistentPerson(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "InvalidPerson", "Staff")

	// Try to update to non-existent person
	body := map[string]interface{}{
		"person_id":   999999,
		"staff_notes": "Should fail",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithJWTBearer(authToken(t, "staff:manage")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// DELETE TEACHER TESTS (Delete Staff Who Is Also Teacher)
// =============================================================================

func TestDeleteStaff_WhoIsTeacher(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create a teacher (this creates person -> staff -> teacher chain)
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "DeleteTeacher", "Test")
	// Note: No defer cleanup as we're deleting

	token := deleteStaffAuthToken(t, ctx.db)
	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/staff/%d", teacher.StaffID), nil,
		testutil.WithJWTBearer(token))

	rr := testutil.ExecuteRequest(ctx.router, req)

	// Should successfully delete both teacher and staff records
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteStaff_ConflictWithSupervision(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	// Create staff with active supervision
	staff := testpkg.CreateTestStaff(t, ctx.db, "SupervisorDelete", "Test")
	room := testpkg.CreateTestRoom(t, ctx.db, "SupervisionRoom")
	activityGroup := testpkg.CreateTestActivityGroup(t, ctx.db, "SupervisionActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activityGroup.ID, room.ID)
	testpkg.CreateTestGroupSupervisor(t, ctx.db, staff.ID, activeGroup.ID, "supervisor")

	token := deleteStaffAuthToken(t, ctx.db)
	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithJWTBearer(token))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusConflict)
}

func TestUpdateSchedule_SaveAsTemplateMaterializesAssignedSnapshot(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "ScheduleTemplate", "Clean")
	repos := repositories.NewFactory(ctx.db)
	repos.SetConfigRuntime(testpkg.ConfigRuntime(ctx.db))

	require.NoError(t, repos.StaffWorkSchedule.ReplaceSchedule(testpkg.Ctx(t), staff.ID, []*configModels.StaffWorkSchedule{
		{
			WeekIndex:      0,
			RotationLength: 1,
			DayOfWeek:      configModels.DayMonday,
			TargetMinutes:  300,
		},
	}, configModels.CalendarDate("")))

	claims := testutil.DefaultTestClaims()
	claims.Permissions = []string{"time_tracking:manage"}
	token := testutil.MintTestJWT(t, claims)
	body := map[string]any{
		"mode":                 "custom",
		"rotation_length":      1,
		"rotation_anchor_date": "2026-06-01",
		"save_as_template":     fmt.Sprintf("Saved schedule template clean %d", staff.ID),
		"entries": []map[string]any{
			{
				"week_index":     0,
				"day_of_week":    configModels.DayTuesday,
				"target_minutes": 360,
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/staff/%d/schedule", staff.ID), body, testutil.WithJWTBearer(token))
	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	activeRows, err := repos.StaffWorkSchedule.GetCurrentByStaffID(testpkg.Ctx(t), staff.ID)
	require.NoError(t, err)
	require.Len(t, activeRows, 1)
	assert.Equal(t, configModels.DayTuesday, activeRows[0].DayOfWeek)
	assert.Equal(t, 360, activeRows[0].TargetMinutes)

	reloadedStaff, err := repositories.NewFactory(ctx.db).Staff.FindByID(testpkg.Ctx(t), staff.ID)
	require.NoError(t, err)
	require.NotNil(t, reloadedStaff.WorkTimeModelID)
	// config.work_time_model_entries has no tenant of its own; it only goes
	// away with its model, so the delete has to actually happen (#2419).
	t.Cleanup(func() {
		_, err := ctx.db.NewUpdate().TableExpr("users.staff").
			Set("work_time_model_id = NULL").
			Where("work_time_model_id = ?", *reloadedStaff.WorkTimeModelID).
			Exec(context.Background())
		require.NoError(t, err)
		_, err = ctx.db.NewDelete().TableExpr("config.work_time_models").
			Where("id = ?", *reloadedStaff.WorkTimeModelID).
			Exec(context.Background())
		require.NoError(t, err)
	})

	model, err := repos.WorkTimeModel.FindByID(testpkg.Ctx(t), *reloadedStaff.WorkTimeModelID)
	require.NoError(t, err)
	require.Len(t, model.Entries, 1)
	assert.Equal(t, configModels.DayTuesday, model.Entries[0].DayOfWeek)
	assert.Equal(t, 360, model.Entries[0].TargetMinutes)
}

func TestGetSchedule_AllowsOwnStaffWithTimeTrackingOwn(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "ScheduleOwn", "Read")
	repos := repositories.NewFactory(ctx.db)
	repos.SetConfigRuntime(testpkg.ConfigRuntime(ctx.db))

	require.NoError(t, repos.StaffWorkSchedule.ReplaceSchedule(testpkg.Ctx(t), staff.ID, []*configModels.StaffWorkSchedule{
		{
			WeekIndex:      0,
			RotationLength: 1,
			DayOfWeek:      configModels.DayMonday,
			TargetMinutes:  300,
		},
	}, configModels.CalendarDate("")))

	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.Permissions = []string{"time_tracking:own"}
	token := testutil.MintTestJWT(t, claims)

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/staff/%d/schedule", staff.ID), nil, testutil.WithJWTBearer(token))
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetSchedule_RejectsOtherStaffWithTimeTrackingOwn(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "ScheduleOwn", "Only")
	otherStaff := testpkg.CreateTestStaff(t, ctx.db, "ScheduleOther", "Denied")

	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.Permissions = []string{"time_tracking:own"}
	token := testutil.MintTestJWT(t, claims)

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/staff/%d/schedule", otherStaff.ID), nil, testutil.WithJWTBearer(token))
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
}

// =============================================================================
// GET STAFF GROUPS - INVALID ID TEST
// =============================================================================

func TestGetStaffGroups_InvalidID(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/invalid/groups", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET STAFF SUBSTITUTIONS - INVALID ID TEST
// =============================================================================

// =============================================================================
// #2906 — THE BETREUER DEFAULT ROLE HAS NO AUTHORITY OVER COLLEAGUE HR DATA
// =============================================================================

// betreuerPermissions is the effective permission set of the unmodified `user`
// (Betreuer) system role for the staff surfaces: it may read the directory and
// write child data, and that is all. Migration 1.9.4 grants users:update to
// this role, which is exactly why users:update must not gate any personnel
// surface (#2906).
var betreuerPermissions = []string{
	"users:read", "users:list", "users:create", "users:update", "users:absence",
	"groups:read", "visits:read", "time_tracking:own",
}

// TestBetreuerRoleCannotReachColleaguePersonnelData pins acceptance criteria
// 1-3 of #2906: with nothing but the Betreuer permissions, every personnel
// read, personnel write and staff-record write on another person is refused.
func TestBetreuerRoleCannotReachColleaguePersonnelData(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)
	colleague := testpkg.CreateTestStaff(t, ctx.db, "Kollegin", "Personalakte")
	token := authToken(t, betreuerPermissions...)

	forbiddenReads := []string{
		fmt.Sprintf("/staff/%d/stammdaten", colleague.ID),
		fmt.Sprintf("/staff/%d/documents", colleague.ID),
		fmt.Sprintf("/staff/documents-profile/%d", colleague.ID),
		"/staff/documents-directory",
		fmt.Sprintf("/staff/%d/stammdaten/bank-steuer", colleague.ID),
	}
	for _, path := range forbiddenReads {
		req := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(token))
		rr := testutil.ExecuteRequest(ctx.router, req)
		assert.Equal(t, http.StatusForbidden, rr.Code, "GET %s must be refused: %s", path, rr.Body.String())
	}

	forbiddenWrites := []struct {
		method string
		path   string
		body   map[string]interface{}
	}{
		{http.MethodPut, fmt.Sprintf("/staff/%d", colleague.ID),
			map[string]interface{}{"person_id": colleague.PersonID, "staff_notes": "Fremde Notiz"}},
		{http.MethodPut, fmt.Sprintf("/staff/%d/stammdaten/person", colleague.ID),
			map[string]interface{}{"first_name": "Neu", "last_name": "Name"}},
		{http.MethodPut, fmt.Sprintf("/staff/%d/stammdaten/kontakt", colleague.ID),
			map[string]interface{}{"phone": "+49 170 1"}},
		{http.MethodPut, fmt.Sprintf("/staff/%d/stammdaten/arbeitsvertrag", colleague.ID),
			map[string]interface{}{"weekly_hours": 20}},
		{http.MethodPut, fmt.Sprintf("/staff/%d/stammdaten/qualifikationen", colleague.ID),
			map[string]interface{}{"qualifikationen": []any{}}},
		{http.MethodPut, fmt.Sprintf("/staff/%d/vacation/quota", colleague.ID),
			map[string]interface{}{"year": 2026, "days": 30}},
	}
	for _, tc := range forbiddenWrites {
		req := testutil.NewAuthenticatedRequest(t, tc.method, tc.path, tc.body, testutil.WithJWTBearer(token))
		rr := testutil.ExecuteRequest(ctx.router, req)
		assert.Equal(t, http.StatusForbidden, rr.Code, "%s %s must be refused: %s", tc.method, tc.path, rr.Body.String())
	}
}

// TestBetreuerRoleSeesOnlyTheMinimalColleagueView pins acceptance criterion 4:
// the directory a Betreuer may read carries names, work e-mail, account role
// and today's presence — never notes, employment type, absence reason or NFC
// tag (#2906).
func TestBetreuerRoleSeesOnlyTheMinimalColleagueView(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)
	colleague := testpkg.CreateTestStaff(t, ctx.db, "Sichtbare", "Kollegin")

	// Give the record something to leak.
	body := map[string]interface{}{
		"person_id":   colleague.PersonID,
		"staff_notes": "Geheime Personalnotiz",
	}
	req := testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/staff/%d", colleague.ID), body,
		testutil.WithJWTBearer(authToken(t, "staff:manage")))
	testutil.AssertSuccessResponse(t, testutil.ExecuteRequest(ctx.router, req), http.StatusOK)

	betreuer := testutil.WithJWTBearer(authToken(t, betreuerPermissions...))
	for _, path := range []string{"/staff", fmt.Sprintf("/staff/%d", colleague.ID)} {
		rr := testutil.ExecuteRequest(ctx.router,
			testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, betreuer))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.NotContains(t, rr.Body.String(), "Geheime Personalnotiz",
			"%s must not carry staff notes for the Betreuer tier", path)
		assert.NotContains(t, rr.Body.String(), `"staff_notes"`, path)
		assert.NotContains(t, rr.Body.String(), `"employment_type"`, path)
		assert.NotContains(t, rr.Body.String(), `"absence_type"`, path)
		assert.NotContains(t, rr.Body.String(), `"tag_id"`, path)
	}

	// The personnel tier still sees it.
	rr := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(t, http.MethodGet,
		fmt.Sprintf("/staff/%d", colleague.ID), nil, testutil.WithJWTBearer(authToken(t, "staff:stammdaten"))))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Geheime Personalnotiz")
}

// TestBetreuerRoleDoesNotSeeTheSchoolsAbsenceWording pins the second half of
// acceptance criterion 4: the school's own Abwesenheitsart wording (#2403)
// names today's absence reason just as absence_type does, so it follows the
// same personnel gate on BOTH staff read paths — the list and the single
// record (#2906).
func TestBetreuerRoleDoesNotSeeTheSchoolsAbsenceWording(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)
	colleague := testpkg.CreateTestStaff(t, ctx.db, "Abwesende", "Kollegin")
	tenantCtx := testpkg.Ctx(t)
	repos := repositories.NewFactory(ctx.db)

	absenceType := &active.StaffAbsenceType{
		Name:     "Regenerationstag",
		BaseType: active.AbsenceTypeOther,
		IsActive: true,
	}
	absenceType.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.StaffAbsenceType.Create(tenantCtx, absenceType))

	today := timezone.TodayDate()
	absence := &active.StaffAbsence{
		StaffID:       colleague.ID,
		AbsenceType:   active.AbsenceTypeOther,
		AbsenceTypeID: &absenceType.ID,
		DateStart:     today,
		DateEnd:       today,
		Status:        active.AbsenceStatusApproved,
		CreatedBy:     colleague.ID,
	}
	absence.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.StaffAbsence.Create(tenantCtx, absence))

	betreuer := testutil.WithJWTBearer(authToken(t, betreuerPermissions...))
	for _, path := range []string{"/staff", fmt.Sprintf("/staff/%d", colleague.ID)} {
		rr := testutil.ExecuteRequest(ctx.router,
			testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, betreuer))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.NotContains(t, rr.Body.String(), "Regenerationstag",
			"%s must not carry the school's absence wording for the Betreuer tier", path)
		assert.NotContains(t, rr.Body.String(), `"absence_type_label"`, path)
	}

	// The personnel tier still sees it on both paths.
	personnel := testutil.WithJWTBearer(authToken(t, "users:read", "staff:stammdaten"))
	for _, path := range []string{"/staff", fmt.Sprintf("/staff/%d", colleague.ID)} {
		rr := testutil.ExecuteRequest(ctx.router,
			testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, personnel))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Regenerationstag", path)
	}
}

// TestVacationQuotaWriteStaysOnTheTimeTrackingTier pins the gate on
// PUT /{id}/vacation/quota (#2906): the quota belongs to the time-tracking
// tier, which also owns reading it back and the Abwesenheiten tab that shows
// it. staff:manage — the personnel-record authority — must not be able to
// write a value it can never see.
func TestVacationQuotaWriteStaysOnTheTimeTrackingTier(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)
	colleague := testpkg.CreateTestStaff(t, ctx.db, "Urlaubs", "Kontingent")
	path := fmt.Sprintf("/staff/%d/vacation/quota", colleague.ID)
	body := map[string]interface{}{"year": timezone.TodayDate().Year(), "days": 30}

	refused := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(
		t, http.MethodPut, path, body, testutil.WithJWTBearer(authToken(t, "staff:manage"))))
	assert.Equal(t, http.StatusForbidden, refused.Code,
		"staff:manage must not write a quota it cannot read: %s", refused.Body.String())

	allowed := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(
		t, http.MethodPut, path, body, testutil.WithJWTBearer(authToken(t, "time_tracking:manage"))))
	assert.Equal(t, http.StatusOK, allowed.Code, allowed.Body.String())
}
