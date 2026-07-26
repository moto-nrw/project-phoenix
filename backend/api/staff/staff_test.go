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
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults before any test (and before setupTestContext
// constructs a Resource via jwt.MustNewTokenAuth). CI runs without a .env so
// AUTH_JWT_SECRET is unset; without a secret jwx refuses HMAC signing.
func init() {
	testutil.SeedTestJWTConfig()
}

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *staffAPI.Resource
	router   chi.Router
}

// setupTestContext initializes test database, services, and resource. The
// router serves the resource through the production middleware chain
// (Verifier → Authenticator → TenantMiddleware → RequiresPermission →
// TenantTxMiddleware) exactly as the real server does, mounted at /staff.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	resource := staffAPI.NewResource(svc.Users, svc.StaffOffboarding, svc.Education, svc.Auth, svc.WorkSession, svc.StaffAbsence, svc.WorkTimeMonth, svc.StaffBalanceAdjust, svc.StaffMonthClose, svc.StaffOverview, svc.TimeTrackingAuditLog, db, slog.Default())

	router := chi.NewRouter()
	router.Mount("/staff", resource.Router())

	return &testContext{
		db:       db,
		services: svc,
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

func cleanupStaffScheduleRows(t *testing.T, db *bun.DB, staffID int64) {
	t.Helper()
	_, err := db.NewDelete().
		Table("config.staff_work_schedules").
		Where("staff_id = ?", staffID).
		Exec(context.Background())
	require.NoError(t, err)
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
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID) })
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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// The response should be an array (even if empty)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, "success", response["status"])
}

func TestListStaff_WithTeachersOnlyFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?teachers_only=true", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithTeachersOnlyFilter_IncludesLegacyTeacherAccounts(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Legacy", "TeacherOnly")
	defer testpkg.CleanupAuthFixtures(t, ctx.db, account.ID)
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?first_name=Test", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithAccountEmail(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create staff with an account so the email-loading path is exercised
	staff, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "EmailList", "Staff")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)
	defer testpkg.CleanupAuthFixtures(t, ctx.db, account.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET STAFF TESTS
// =============================================================================

func TestGetStaff_WithAccountEmail(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create staff with an account so the email-loading path is exercised
	staff, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "EmailGet", "Staff")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)
	defer testpkg.CleanupAuthFixtures(t, ctx.db, account.ID)

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "GetStaff", "Test")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaff_AllowsTimeTrackingManage(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	staff := testpkg.CreateTestStaff(t, ctx.db, "TimeTrackingManager", "Profile")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "time_tracking:manage")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaff_RejectsUnrelatedPermission(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	staff := testpkg.CreateTestStaff(t, ctx.db, "UnrelatedPermission", "Profile")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "rooms:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
}

func TestGetStaff_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/999999", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetStaff_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/invalid", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// TestGetStaff_WorkStatusConsistentWithList covers the regression where the
// detail endpoint returned an empty work_status while the list endpoint
// resolved it correctly, making the location badge flip-flop between views.
func TestGetStaff_WorkStatusConsistentWithList(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	staff := testpkg.CreateTestStaff(t, ctx.db, "Consistent", "Status")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	tenantID := int64(testutil.DefaultTestClaims().TenantID)
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
	defer testpkg.CleanupTableRecords(t, ctx.db, "active.work_sessions", session.ID)

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a person without staff record
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	person := testpkg.CreateTestPerson(t, ctx.db, "NewStaff"+uniqueSuffix, "Person")
	defer testpkg.CleanupPerson(t, ctx.db, person.ID)

	body := map[string]interface{}{
		"person_id":   person.ID,
		"staff_notes": "Test staff notes",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithJWTBearer(authToken(t, "users:create")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Parse response to get staff ID for cleanup
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if data, ok := response["data"].(map[string]interface{}); ok {
		if staffID, ok := data["id"].(float64); ok {
			defer testpkg.CleanupStaffFixtures(t, ctx.db, int64(staffID))
		}
	}
}

func TestCreateStaff_AsTeacher(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a person without staff record
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	person := testpkg.CreateTestPerson(t, ctx.db, "NewTeacher"+uniqueSuffix, "Person")
	defer testpkg.CleanupPerson(t, ctx.db, person.ID)

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
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if data, ok := response["data"].(map[string]interface{}); ok {
		if staffID, ok := data["id"].(float64); ok {
			defer testpkg.CleanupStaffFixtures(t, ctx.db, int64(staffID))
		}
	}
}

func TestCreateStaff_PersonNotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	body := map[string]interface{}{
		"person_id": 999999,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/staff", body,
		testutil.WithJWTBearer(authToken(t, "users:create")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

func TestCreateStaff_MissingPersonID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "UpdateStaff", "Test")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	body := map[string]interface{}{
		"person_id":   staff.PersonID,
		"staff_notes": "Updated notes",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithJWTBearer(authToken(t, "users:update")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateStaff_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	body := map[string]interface{}{
		"person_id":   1,
		"staff_notes": "Should fail",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/999999", body,
		testutil.WithJWTBearer(authToken(t, "users:update")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdateStaff_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	body := map[string]interface{}{
		"person_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/staff/invalid", body,
		testutil.WithJWTBearer(authToken(t, "users:update")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// DELETE STAFF TESTS
// =============================================================================

func TestDeleteStaff_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "DeleteStaff", "Test")
	t.Cleanup(func() {
		// Offboarding soft-deletes; remove the row for real.
		_, _ = ctx.db.ExecContext(context.Background(), `DELETE FROM users.staff WHERE id = ?`, staff.ID)
		_, _ = ctx.db.ExecContext(context.Background(), `DELETE FROM audit.data_deletions WHERE staff_id = ?`, staff.ID)
		_, _ = ctx.db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, staff.PersonID)
	})

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "users:delete")))

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/staff/999999", nil,
		testutil.WithJWTBearer(authToken(t, "users:delete")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	// NOTE: The delete operation returns success even for non-existent IDs.
	// This is idempotent behavior - deleting something that doesn't exist is still "successful".
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteStaff_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/staff/invalid", nil,
		testutil.WithJWTBearer(authToken(t, "users:delete")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET STAFF GROUPS TESTS
// =============================================================================

func TestGetStaffGroups_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a teacher with the full chain (person -> staff -> teacher)
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "GroupsTest", "Teacher")
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/staff/%d/groups", teacher.StaffID), nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaffGroups_NonTeacher(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create regular staff (not a teacher)
	staff := testpkg.CreateTestStaff(t, ctx.db, "NonTeacher", "Staff")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/staff/%d/groups", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	// Should return success with empty groups array for non-teachers
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaffGroups_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/999999/groups", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// GET STAFF SUBSTITUTIONS TESTS
// =============================================================================

func TestGetStaffSubstitutions_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test staff
	staff := testpkg.CreateTestStaff(t, ctx.db, "SubstitutionsTest", "Staff")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/staff/%d/substitutions", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaffSubstitutions_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/999999/substitutions", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// GET AVAILABLE STAFF TESTS
// =============================================================================

func TestGetAvailableStaff_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/available", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET AVAILABLE FOR SUBSTITUTION TESTS
// =============================================================================

func TestGetAvailableForSubstitution_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/available-for-substitution", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetAvailableForSubstitution_WithDateFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/available-for-substitution?date=2024-01-15", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetAvailableForSubstitution_WithSearchFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/available-for-substitution?search=Test", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET STAFF BY ROLE TESTS
// =============================================================================

func TestGetStaffByRole_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/by-role?role=user", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetStaffByRole_Teacher_IncludesLegacyTeacherRoleAccounts(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Legacy", "RoleTeacher")
	defer testpkg.CleanupAuthFixtures(t, ctx.db, account.ID)
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)
	testpkg.EnsureAccountTenant(t, ctx.db, account.ID, 1)
	assignSystemRoleToAccount(t, ctx.db, account.ID, 1, "teacher")

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Legacy", "Caregiver")
	defer testpkg.CleanupAuthFixtures(t, ctx.db, account.ID)
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)
	testpkg.EnsureAccountTenant(t, ctx.db, account.ID, 1)
	assignSystemRoleToAccount(t, ctx.db, account.ID, 1, "teacher")

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/by-role", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// PIN STATUS TESTS
// =============================================================================

func TestGetPINStatus_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a staff with account
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "PINTest", "Staff")
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a staff with account
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "UpdatePIN", "Staff")
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a staff with account
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "NonDigit", "PIN")
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a staff with account
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "MissingPIN", "Test")
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a staff with account (no existing PIN)
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "FirstPIN", "Setup")
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Test that the anwesend filter works (even if no staff are currently present)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?anwesend=true", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithRoleFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Test filtering by role (the role filter branch)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?role=admin", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithLastNameFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Test filtering by last name
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff?last_name=Test", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListStaff_WithCombinedFilters(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create regular staff (not a teacher)
	staff := testpkg.CreateTestStaff(t, ctx.db, "ConvertTo", "Teacher")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	// Update to convert to teacher
	body := map[string]interface{}{
		"person_id":      staff.PersonID,
		"staff_notes":    "Converting to teacher",
		"is_teacher":     true,
		"specialization": "Mathematics",
		"role":           "Teacher",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithJWTBearer(authToken(t, "users:update")))

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a teacher with the full chain
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "UpdateTeacher", "Test")
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

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
		testutil.WithJWTBearer(authToken(t, "users:update")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateStaff_KeepExistingTeacherWithoutIsTeacher(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a teacher
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "KeepTeacher", "Test")
	defer testpkg.CleanupTeacherFixtures(t, ctx.db, teacher.ID)

	// Update without is_teacher flag - should still return teacher response
	body := map[string]interface{}{
		"person_id":   teacher.Staff.PersonID,
		"staff_notes": "Updated notes only",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", teacher.StaffID), body,
		testutil.WithJWTBearer(authToken(t, "users:update")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateStaff_InvalidRequest(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	staff := testpkg.CreateTestStaff(t, ctx.db, "InvalidReq", "Staff")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	// Invalid request - missing person_id
	body := map[string]interface{}{
		"staff_notes": "Missing person ID",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithJWTBearer(authToken(t, "users:update")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdateStaff_ChangePersonID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create two persons
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	person1 := testpkg.CreateTestPerson(t, ctx.db, "Original"+uniqueSuffix, "Person")
	person2 := testpkg.CreateTestPerson(t, ctx.db, "New"+uniqueSuffix, "Person")
	defer testpkg.CleanupPerson(t, ctx.db, person1.ID)
	defer testpkg.CleanupPerson(t, ctx.db, person2.ID)

	// Create staff with person1
	staff := testpkg.CreateTestStaffForPerson(t, ctx.db, person1.ID)
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	// Update to point to person2
	body := map[string]interface{}{
		"person_id":   person2.ID,
		"staff_notes": "Changed person",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithJWTBearer(authToken(t, "users:update")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateStaff_ChangeToNonExistentPerson(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	staff := testpkg.CreateTestStaff(t, ctx.db, "InvalidPerson", "Staff")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	// Try to update to non-existent person
	body := map[string]interface{}{
		"person_id":   999999,
		"staff_notes": "Should fail",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/staff/%d", staff.ID), body,
		testutil.WithJWTBearer(authToken(t, "users:update")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// DELETE TEACHER TESTS (Delete Staff Who Is Also Teacher)
// =============================================================================

func TestDeleteStaff_WhoIsTeacher(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a teacher (this creates person -> staff -> teacher chain)
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "DeleteTeacher", "Test")
	// Note: No defer cleanup as we're deleting

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/staff/%d", teacher.StaffID), nil,
		testutil.WithJWTBearer(authToken(t, "users:delete")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	// Should successfully delete both teacher and staff records
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteStaff_ConflictWithSupervision(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create staff with active supervision
	staff := testpkg.CreateTestStaff(t, ctx.db, "SupervisorDelete", "Test")
	room := testpkg.CreateTestRoom(t, ctx.db, "SupervisionRoom")
	activityGroup := testpkg.CreateTestActivityGroup(t, ctx.db, "SupervisionActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activityGroup.ID, room.ID)
	testpkg.CreateTestGroupSupervisor(t, ctx.db, staff.ID, activeGroup.ID, "supervisor")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID, activityGroup.ID, room.ID)
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/staff/%d", staff.ID), nil,
		testutil.WithJWTBearer(authToken(t, "users:delete")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusConflict)
}

func TestUpdateSchedule_SaveAsTemplateMaterializesAssignedSnapshot(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	staff := testpkg.CreateTestStaff(t, ctx.db, "ScheduleTemplate", "Clean")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)
	defer cleanupStaffScheduleRows(t, ctx.db, staff.ID)

	require.NoError(t, repositories.NewFactory(ctx.db).StaffWorkSchedule.ReplaceSchedule(testpkg.TenantContext(1), staff.ID, []*configModels.StaffWorkSchedule{
		{
			WeekIndex:      0,
			RotationLength: 1,
			DayOfWeek:      configModels.DayMonday,
			TargetMinutes:  300,
		},
	}, timezone.Date{}))

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

	activeRows, err := repositories.NewFactory(ctx.db).StaffWorkSchedule.GetCurrentByStaffID(testpkg.TenantContext(1), staff.ID)
	require.NoError(t, err)
	require.Len(t, activeRows, 1)
	assert.Equal(t, configModels.DayTuesday, activeRows[0].DayOfWeek)
	assert.Equal(t, 360, activeRows[0].TargetMinutes)

	reloadedStaff, err := repositories.NewFactory(ctx.db).Staff.FindByID(testpkg.TenantContext(1), staff.ID)
	require.NoError(t, err)
	require.NotNil(t, reloadedStaff.WorkTimeModelID)
	t.Cleanup(func() {
		_ = repositories.NewFactory(ctx.db).WorkTimeModel.Delete(testpkg.TenantContext(1), *reloadedStaff.WorkTimeModelID)
	})

	model, err := repositories.NewFactory(ctx.db).WorkTimeModel.FindByID(testpkg.TenantContext(1), *reloadedStaff.WorkTimeModelID)
	require.NoError(t, err)
	require.Len(t, model.Entries, 1)
	assert.Equal(t, configModels.DayTuesday, model.Entries[0].DayOfWeek)
	assert.Equal(t, 360, model.Entries[0].TargetMinutes)
}

func TestGetSchedule_AllowsOwnStaffWithTimeTrackingOwn(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	staff, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "ScheduleOwn", "Read")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, staff.ID)
	defer cleanupStaffScheduleRows(t, ctx.db, staff.ID)

	require.NoError(t, repositories.NewFactory(ctx.db).StaffWorkSchedule.ReplaceSchedule(testpkg.TenantContext(1), staff.ID, []*configModels.StaffWorkSchedule{
		{
			WeekIndex:      0,
			RotationLength: 1,
			DayOfWeek:      configModels.DayMonday,
			TargetMinutes:  300,
		},
	}, timezone.Date{}))

	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.Permissions = []string{"time_tracking:own"}
	token := testutil.MintTestJWT(t, claims)

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/staff/%d/schedule", staff.ID), nil, testutil.WithJWTBearer(token))
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetSchedule_RejectsOtherStaffWithTimeTrackingOwn(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	ownStaff, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "ScheduleOwn", "Only")
	otherStaff := testpkg.CreateTestStaff(t, ctx.db, "ScheduleOther", "Denied")
	defer testpkg.CleanupStaffFixtures(t, ctx.db, otherStaff.ID, ownStaff.ID)

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
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/invalid/groups", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET STAFF SUBSTITUTIONS - INVALID ID TEST
// =============================================================================

func TestGetStaffSubstitutions_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/invalid/substitutions", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// GET AVAILABLE FOR SUBSTITUTION - INVALID DATE TEST
// =============================================================================

func TestGetAvailableForSubstitution_WithInvalidDate(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Invalid date format should fall back to current date
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff/available-for-substitution?date=invalid", nil,
		testutil.WithJWTBearer(authToken(t, "users:read")))

	rr := testutil.ExecuteRequest(ctx.router, req)

	// Should still succeed with fallback to current date
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}
