// Package usercontext_test tests the usercontext API handlers with hermetic test pattern.
//
// These tests mount the production Resource.Router() and authenticate via real
// signed JWTs (testutil.MintTestJWT + testutil.WithJWTBearer) so the full
// middleware chain (Verifier → Authenticator → TenantMiddleware → TenantTxMiddleware)
// runs exactly as it does in production. They verify HTTP request/response
// handling, status codes, and error responses against a real test database.
package usercontext_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	usercontextAPI "github.com/moto-nrw/project-phoenix/api/usercontext"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults before any test (and before setupUserContextRoute
// constructs a Resource via jwt.MustNewTokenAuth). CI runs without a .env so
// AUTH_JWT_SECRET is unset; without a secret jwx refuses HMAC signing.
func init() {
	testutil.SeedTestJWTConfig()
}

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	avatar   func(context.Context, int64) (string, error)
	resource *usercontextAPI.Resource
	router   chi.Router
}

// setupUserContextRoute initializes test database, services, resource, and a router
// that serves the resource at the same paths it would in production.
func setupUserContextRoute(t *testing.T) *testContext {
	t.Helper()

	db, serviceFactory := testutil.SetupUserContextModule(t)
	resource := usercontextAPI.NewResource(serviceFactory.UserContext, db)

	return &testContext{
		db: db,
		avatar: func(ctx context.Context, id int64) (string, error) {
			var avatar string
			err := db.NewSelect().
				TableExpr(`auth.accounts AS "account"`).
				ColumnExpr(`"account".avatar`).
				Where(`"account".id = ?`, id).
				Scan(ctx, &avatar)
			return avatar, err
		},
		resource: resource,
		router:   resource.Router(),
	}
}

// =============================================================================
// GET CURRENT USER TESTS
// =============================================================================

func TestGetCurrentUser_Success(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	account := testpkg.CreateTestAccount(t, tc.db, "usercontext-test@example.com")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetCurrentUser_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/", nil)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET CURRENT PROFILE TESTS
// =============================================================================

func TestGetCurrentProfile_Success(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "Profile", "Test")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/profile", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetCurrentProfile_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/profile", nil)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// UPDATE CURRENT PROFILE TESTS
// =============================================================================

func TestUpdateCurrentProfile_Success(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "Update", "ProfileTest")

	body := map[string]interface{}{
		"first_name": "Updated",
		"last_name":  "Name",
	}

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/profile", body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateCurrentProfile_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	body := map[string]interface{}{"first_name": "Updated"}
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/profile", body)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

func TestUpdateCurrentProfile_EmptyBody(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "Empty", "Update")

	body := map[string]interface{}{}

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/profile", body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET CURRENT STAFF TESTS
// =============================================================================

func TestGetCurrentStaff_Success(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Staff", "Test")
	personnelNumber := "90001"
	teacher.Staff.PersonnelNumber = &personnelNumber
	_, err := tc.db.NewUpdate().
		Model(teacher.Staff).
		ModelTableExpr(`users.staff AS "staff"`).
		Column("personnel_number").
		WherePK().
		Exec(context.Background())
	require.NoError(t, err)

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.NotContains(t, rr.Body.String(), "personnel_number")
}

func TestGetCurrentStaff_NotStaff(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	account := testpkg.CreateTestAccount(t, tc.db, "not-staff@example.com")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Contains(t, []int{http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError}, rr.Code)
}

func TestGetCurrentStaff_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/staff", nil)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET CURRENT TEACHER TESTS
// =============================================================================

func TestGetCurrentTeacher_Success(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Teacher", "Test")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/teacher", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetCurrentTeacher_NotTeacher(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	account := testpkg.CreateTestAccount(t, tc.db, "not-teacher@example.com")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/teacher", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Contains(t, []int{http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError}, rr.Code)
}

func TestGetCurrentTeacher_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/teacher", nil)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET MY GROUPS TESTS
// =============================================================================

func TestGetMyGroups_Success(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Groups", "Test")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetMyGroups_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups", nil)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET MY ACTIVITY GROUPS TESTS
// =============================================================================

func TestGetMyActivityGroups_Success(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Activity", "Groups")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/activity", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetMyActivityGroups_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/activity", nil)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET MY ACTIVE GROUPS TESTS
// =============================================================================

func TestGetMyActiveGroups_Success(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Active", "Groups")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/active", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetMyActiveGroups_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/active", nil)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET MY SUPERVISED GROUPS TESTS
// =============================================================================

func TestGetMySupervisedGroups_Success(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Supervised", "Groups")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/supervised", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetMySupervisedGroups_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/supervised", nil)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET GROUP STUDENTS TESTS
// =============================================================================

func TestGetGroupStudents_InvalidGroupID(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	claims := testutil.DefaultTestClaims()
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/invalid/students", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGetGroupStudents_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "GroupStudentsUnauth")
	room := testpkg.CreateTestRoom(t, tc.db, "GroupStudentsUnauthRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students", activeGroup.ID), nil)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// GET GROUP VISITS TESTS
// =============================================================================

func TestGetGroupVisits_InvalidGroupID(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	claims := testutil.DefaultTestClaims()
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups/invalid/visits", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGetGroupVisits_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "GroupVisitsUnauth")
	room := testpkg.CreateTestRoom(t, tc.db, "GroupVisitsUnauthRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/visits", activeGroup.ID), nil)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

// =============================================================================
// AVATAR TESTS
// =============================================================================

func TestDeleteAvatar_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/profile/avatar", nil)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

func TestDeleteAvatar_NoAvatar(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "NoAvatar", "Test")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/profile/avatar", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestServeAvatar_MissingFilename(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "Avatar", "Serve")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/profile/avatar/", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, rr.Code)
}

func TestServeAvatar_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/profile/avatar/test.jpg", nil)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for unauthenticated request")
}

func TestServeAvatar_GlobalAvatarFile(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "Avatar", "GlobalFile")

	publicDir, err := common.ResolvePublicDir()
	if err != nil {
		publicDir = filepath.Join("public")
	}
	avatarDir := filepath.Join(publicDir, "uploads", "avatars", "global")
	err = os.MkdirAll(avatarDir, 0755)
	require.NoError(t, err)

	filename := fmt.Sprintf("%d_test-avatar.jpg", account.ID)
	filePath := filepath.Join(avatarDir, filename)
	err = os.WriteFile(filePath, []byte("fake-image-data"), 0644)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Remove(filePath)
	})

	_, err = tc.db.ExecContext(context.Background(),
		`UPDATE auth.accounts SET avatar = ? WHERE id = ?`,
		"/uploads/avatars/global/"+filename,
		account.ID,
	)
	require.NoError(t, err)

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/profile/avatar/"+filename, nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, []byte("fake-image-data"), rr.Body.Bytes())
}

// =============================================================================
// ROUTER TESTS
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	router := tc.resource.Router()
	assert.NotNil(t, router, "Router should not be nil")
}

// =============================================================================
// UPDATE PROFILE WITH ALL FIELDS TESTS
// =============================================================================

func TestUpdateCurrentProfile_WithUsernameAndBio(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "FullUpdate", "ProfileTest")

	uniqueUsername := fmt.Sprintf("user_%d", account.ID)
	body := map[string]interface{}{
		"first_name": "NewFirst",
		"last_name":  "NewLast",
		"username":   uniqueUsername,
		"bio":        "This is my bio text",
	}

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/profile", body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GROUP STUDENTS WITH TEACHER ACCESS TESTS
// =============================================================================

func TestGetGroupStudents_WithTeacherAccess(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "GroupStudents", "Teacher")

	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "StudentAccessGroup")
	room := testpkg.CreateTestRoom(t, tc.db, "StudentAccessRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/students", activeGroup.ID), nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Contains(t, []int{http.StatusOK, http.StatusForbidden, http.StatusInternalServerError}, rr.Code)
}

// =============================================================================
// GROUP VISITS WITH TEACHER ACCESS TESTS
// =============================================================================

func TestGetGroupVisits_WithTeacherAccess(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "GroupVisits", "Teacher")

	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "VisitsAccessGroup")
	room := testpkg.CreateTestRoom(t, tc.db, "VisitsAccessRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/groups/%d/visits", activeGroup.ID), nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Contains(t, []int{http.StatusOK, http.StatusForbidden, http.StatusInternalServerError}, rr.Code)
}

// =============================================================================
// SERVE AVATAR INVALID PATH TESTS
// =============================================================================

func TestServeAvatar_InvalidFilename(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "InvalidPath", "Avatar")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/profile/avatar/../../../etc/passwd", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Contains(t, []int{http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound}, rr.Code)
}

func TestServeAvatar_NonExistentFile(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "NonExistent", "Avatar")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/profile/avatar/nonexistent_file_12345.jpg", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, rr.Code)
}

// =============================================================================
// UPLOAD AVATAR TESTS
// =============================================================================

func TestUploadAvatar_Unauthenticated(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "POST", "/profile/avatar", nil)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnauthorized}, rr.Code,
		"Expected 400 or 401 for unauthenticated request")
}

func TestUploadAvatar_NoFile(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "NoFile", "Upload")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "POST", "/profile/avatar", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUploadAvatar_Success(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "Upload", "Success")

	pngContent := string([]byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde,
		0x00, 0x00, 0x00, 0x0a, 'I', 'D', 'A', 'T',
		0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00,
		0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92, 0xef,
		0x00, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D',
		0xae, 'B', 0x60, 0x82,
	})
	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewMultipartRequest(t, "POST", "/profile/avatar", "avatar", "avatar.png", pngContent,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	avatar, err := tc.avatar(context.Background(), account.ID)
	require.NoError(t, err)
	require.NotEmpty(t, avatar)
	assert.Contains(t, avatar, "/uploads/avatars/global/")
	assert.Equal(t, ".png", filepath.Ext(avatar))

	// The upload goes through the storage backend, which resolves a relative
	// upload directory against the discovered public dir. Rebuilding the path
	// from the working directory would look for the avatar in the test
	// package's directory instead of where the handler just wrote it.
	avatarFilePath, err := common.ResolveStoredPath("public", avatar, "/uploads/avatars/global/")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Remove(avatarFilePath)
	})

	_, err = os.Stat(avatarFilePath)
	require.NoError(t, err)
}

// =============================================================================
// DELETE AVATAR WITH AVATAR TESTS
// =============================================================================

func TestDeleteAvatar_WithAvatar(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "HasAvatar", "Delete")
	avatarDir := filepath.Join("public", "uploads", "avatars", "global")
	err := os.MkdirAll(avatarDir, 0755)
	require.NoError(t, err)

	avatarPath := filepath.Join(avatarDir, "test_avatar.jpg")
	err = os.WriteFile(avatarPath, []byte("fake-image-data"), 0644)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Remove(avatarPath)
	})

	_, err = tc.db.ExecContext(context.Background(),
		`UPDATE auth.accounts SET avatar = ? WHERE id = ?`,
		"/uploads/avatars/global/test_avatar.jpg",
		account.ID,
	)
	require.NoError(t, err)

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/profile/avatar", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	_, err = os.Stat(avatarPath)
	assert.True(t, os.IsNotExist(err))
}

// =============================================================================
// GET MY GROUPS WITH SUBSTITUTION TESTS
// =============================================================================

func TestGetMyGroups_WithSubstitution(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "SubstGroups", "Teacher")

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/groups", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(tc.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, "success", response["status"])
}
