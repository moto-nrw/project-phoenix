package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests in the guardians_composition_* files drive the composed
// /guardians surface (#2663): the People Directory guardian adapter bound to
// the shared renderer, the JWT identity and the legacy-service runtime. They
// are the public HTTP contract goldens of the former api/guardians package
// and run through the real middleware chain (Verifier -> Authenticator ->
// TenantMiddleware -> RequiresPermission -> TenantTxMiddleware).

func init() {
	// Ensure a JWT secret exists so MintTestJWT and Router()'s MustNewTokenAuth
	// agree (no-op when AUTH_JWT_SECRET is already in the environment).
	testutil.SeedTestJWTConfig()
}

// guardianCompositionContext keeps the composed router plus fixture closures;
// the closures capture the database so the test names no persistence type.
type guardianCompositionContext struct {
	router chi.Router
	// createGuardian creates a guardian profile and returns its id and e-mail.
	createGuardian func(emailSeed string) (int64, string)
	// createNamedGuardian creates a guardian profile with the given names.
	createNamedGuardian func(firstName, lastName, emailSeed string) (int64, string)
	// createStudent creates an active student and returns (studentID, personID).
	createStudent func(firstName, lastName, class string) (int64, int64)
	// link inserts a students_guardians row with the role preset and returns
	// the link id.
	link func(studentID, guardianID int64, role string) int64
	// linkGrantsPortalAccess reads the stored link's parent_portal.access.
	linkGrantsPortalAccess func(linkID int64) bool
	// staffAccount creates a verified staff member and returns the account id.
	staffAccount func(firstName, lastName string) int64
	// teacherAccount creates a teacher with an account and returns
	// (teacherID, accountID).
	teacherAccount func(firstName, lastName string) (int64, int64)
	// account creates a bare account (no staff record) and returns its id.
	account func(emailSeed string) int64
	// assignGroup puts the student into a fresh group supervised by the teacher.
	assignGroup func(studentID, teacherID int64, groupName string)
	// graduate flips the student to alumnus.
	graduate func(studentID int64)
	// softDeletePerson marks the person row deleted.
	softDeletePerson func(personID int64)
	// guardianEmailCount counts the tenant's profiles with that e-mail.
	guardianEmailCount func(email string) int
	// accessLogs counts data-access rows of one resource type in the tenant.
	accessLogs func(resourceType string) int
	// financialChanges returns the audit rows of one guardian as
	// (field_name, new_value, student_id set) triples.
	financialChanges func(guardianID int64) []financialChangeRow
	// foreignTenant creates a second tenant with a student, a guardian and a
	// primary-guardian link and returns (studentID, guardianID, linkID).
	foreignTenant func() (int64, int64, int64)
	// persistedLink reads the stored link's role and flags.
	persistedLink func(linkID int64) persistedLinkState
}

type financialChangeRow struct {
	Field        string
	NewValue     string
	HasStudentID bool
}

type persistedLinkState struct {
	Role               string
	CanPickup          bool
	IsEmergencyContact bool
	PortalAccess       bool
}

func setupGuardiansCompositionRoute(t *testing.T, appEnvs ...string) *guardianCompositionContext {
	t.Helper()
	appEnv := ""
	if len(appEnvs) > 0 {
		appEnv = appEnvs[0]
	}
	db, svc := testutil.SetupAPITest(t)
	resource := newGuardiansResource(svc.PeopleDirectory, svc, db, appEnv, slog.Default())

	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/guardians", resource.Router())

	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	return &guardianCompositionContext{
		router: router,
		createGuardian: func(emailSeed string) (int64, string) {
			profile := testpkg.CreateTestGuardianProfile(t, db, emailSeed)
			return profile.ID, *profile.Email
		},
		createNamedGuardian: func(firstName, lastName, emailSeed string) (int64, string) {
			profile := testpkg.CreateTestGuardianProfileNamed(t, db, firstName, lastName, emailSeed)
			return profile.ID, *profile.Email
		},
		createStudent: func(firstName, lastName, class string) (int64, int64) {
			student := testpkg.CreateTestStudent(t, db, firstName, lastName, class)
			return student.ID, student.PersonID
		},
		link: func(studentID, guardianID int64, role string) int64 {
			return testpkg.CreateTestStudentGuardianLink(t, db, studentID, guardianID, role).ID
		},
		linkGrantsPortalAccess: func(linkID int64) bool {
			return testpkg.StudentGuardianLinkGrantsPortalAccess(t, db, linkID)
		},
		staffAccount: func(firstName, lastName string) int64 {
			_, account := testpkg.CreateTestStaffWithAccount(t, db, firstName, lastName)
			return account.ID
		},
		teacherAccount: func(firstName, lastName string) (int64, int64) {
			teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, firstName, lastName)
			return teacher.ID, account.ID
		},
		account: func(emailSeed string) int64 {
			return testpkg.CreateTestAccount(t, db, emailSeed).ID
		},
		assignGroup: func(studentID, teacherID int64, groupName string) {
			group := testpkg.CreateTestEducationGroup(t, db, groupName)
			testpkg.CreateTestGroupTeacher(t, db, group.ID, teacherID)
			_, err := db.NewUpdate().TableExpr("users.students").
				Set("group_id = ?", group.ID).
				Where("id = ?", studentID).
				Where("tenant_id = ?", tenantID).
				Exec(ctx)
			require.NoError(t, err)
		},
		graduate: func(studentID int64) {
			_, err := db.NewUpdate().TableExpr("users.students").
				Set("status = ?", "alumnus").
				Where("id = ?", studentID).
				Where("tenant_id = ?", tenantID).
				Exec(ctx)
			require.NoError(t, err)
		},
		softDeletePerson: func(personID int64) {
			_, err := db.NewUpdate().TableExpr(`users.persons AS "person"`).
				Set("deleted_at = NOW()").
				Where(`"person".id = ?`, personID).
				Exec(ctx)
			require.NoError(t, err)
		},
		guardianEmailCount: func(email string) int {
			count, err := db.NewSelect().
				TableExpr(`users.guardian_profiles AS "guardian_profile"`).
				Where(`"guardian_profile".tenant_id = ?`, tenantID).
				Where(`"guardian_profile".email = ?`, email).
				Count(ctx)
			require.NoError(t, err)
			return count
		},
		accessLogs: func(resourceType string) int {
			count, err := db.NewSelect().
				TableExpr(`audit.data_access_log AS "log"`).
				Where(`"log".tenant_id = ?`, tenantID).
				Where(`"log".resource_type = ?`, resourceType).
				Count(ctx)
			require.NoError(t, err)
			return count
		},
		financialChanges: func(guardianID int64) []financialChangeRow {
			var rows []struct {
				FieldName string `bun:"field_name"`
				NewValue  string `bun:"new_value"`
				StudentID *int64 `bun:"student_id"`
			}
			err := db.NewSelect().
				TableExpr(`audit.guardian_financial_changes AS "guardian_financial_change"`).
				ColumnExpr(`"guardian_financial_change".field_name, "guardian_financial_change".new_value, "guardian_financial_change".student_id`).
				Where(`"guardian_financial_change".tenant_id = ?`, tenantID).
				Where(`"guardian_financial_change".guardian_profile_id = ?`, guardianID).
				Scan(ctx, &rows)
			require.NoError(t, err)
			result := make([]financialChangeRow, 0, len(rows))
			for _, row := range rows {
				result = append(result, financialChangeRow{Field: row.FieldName, NewValue: row.NewValue, HasStudentID: row.StudentID != nil})
			}
			return result
		},
		foreignTenant: func() (int64, int64, int64) {
			foreignTenantID, _ := testpkg.CreateTestTenant(t, db)
			student := testpkg.CreateTestStudentForTenant(t, db, foreignTenantID, "Foreign", "Child", "1a")
			guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, foreignTenantID, "Foreign", "Guardian", "foreign-guardian")
			link := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, foreignTenantID, student.ID, guardian.ID, "primary_guardian")
			return student.ID, guardian.ID, link.ID
		},
		persistedLink: func(linkID int64) persistedLinkState {
			var row struct {
				Role      string `bun:"guardian_role"`
				CanPickup bool   `bun:"can_pickup"`
				Emergency bool   `bun:"is_emergency_contact"`
			}
			err := db.NewSelect().
				TableExpr(`users.students_guardians AS "student_guardian"`).
				ColumnExpr(`"student_guardian".guardian_role, "student_guardian".can_pickup, "student_guardian".is_emergency_contact`).
				Where(`"student_guardian".id = ?`, linkID).
				Scan(ctx, &row)
			require.NoError(t, err)
			return persistedLinkState{
				Role: row.Role, CanPickup: row.CanPickup, IsEmergencyContact: row.Emergency,
				PortalAccess: testpkg.StudentGuardianLinkGrantsPortalAccess(t, db, linkID),
			}
		},
	}
}

// bearer mints a signed JWT for claims and returns it as an Authorization
// bearer option.
func bearer(t *testing.T, claims jwt.AppClaims) testutil.RequestOption {
	t.Helper()
	return testutil.WithJWTBearer(testutil.MintTestJWT(t, claims))
}

// nonStaffClaims builds an authenticated but non-admin principal for account 1
// (which has no staff record in the test DB). It carries exactly perms so the
// request clears the route's RequiresPermission gate and is then denied by
// the handler's own staff check.
func nonStaffClaims(tb testing.TB, perms ...string) jwt.AppClaims {
	return jwt.AppClaims{
		ID:          1,
		Sub:         "nonstaff@example.com",
		TenantID:    testpkg.Tenant(tb),
		Roles:       []string{"user"},
		Permissions: perms,
	}
}

// withPerms returns a copy of claims narrowed to exactly perms.
func withPerms(claims jwt.AppClaims, perms ...string) jwt.AppClaims {
	claims.Permissions = perms
	return claims
}

// guardianStaffClaims are teacher claims for accountID carrying exactly the
// given permissions.
func guardianStaffClaims(accountID int64, accountPermissions ...string) jwt.AppClaims {
	claims := testutil.TeacherTestClaims(int(accountID))
	claims.Permissions = accountPermissions
	return claims
}

func guardiansPath(format string, args ...any) string {
	return "/guardians" + fmt.Sprintf(format, args...)
}

func (c *guardianCompositionContext) do(t *testing.T, claims jwt.AppClaims, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, method, guardiansPath("%s", path), body, bearer(t, claims))
	return testutil.ExecuteRequest(c.router, req)
}

func (c *guardianCompositionContext) status(t *testing.T, claims jwt.AppClaims, method, path string, body any) int {
	t.Helper()
	return c.do(t, claims, method, path, body).Code
}

func errorText(t *testing.T, body string) string {
	t.Helper()
	response := testutil.ParseJSONResponse(t, []byte(body))
	text, ok := response["error"].(string)
	require.True(t, ok, "expected error text in response: %s", body)
	return text
}

func dataObject(t *testing.T, body []byte) map[string]any {
	t.Helper()
	response := testutil.ParseJSONResponse(t, body)
	data, ok := response["data"].(map[string]any)
	require.True(t, ok, "expected data object in response: %s", string(body))
	return data
}

func dataArray(t *testing.T, body []byte) []any {
	t.Helper()
	response := testutil.ParseJSONResponse(t, body)
	data, ok := response["data"].([]any)
	require.True(t, ok, "expected data array in response: %s", string(body))
	return data
}

// studentGuardianRow is one entry of GET /students/{id}/guardians.
type studentGuardianRow struct {
	Guardian struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
	} `json:"guardian"`
	RelationshipID     int64  `json:"relationship_id"`
	GuardianRole       string `json:"guardian_role"`
	CanPickup          bool   `json:"can_pickup"`
	IsEmergencyContact bool   `json:"is_emergency_contact"`
	IsPayer            bool   `json:"is_payer"`
	AccountStatus      string `json:"account_status"`
}

// studentGuardians reads a child's guardians as an admin holding the
// financial permission so the payer mark is visible.
func (c *guardianCompositionContext) studentGuardians(t *testing.T, studentID int64) []studentGuardianRow {
	t.Helper()
	claims := testutil.AdminTestClaims(999)
	claims.Permissions = append(claims.Permissions, "guardians:financial")
	rr := c.do(t, claims, http.MethodGet, fmt.Sprintf("/students/%d/guardians", studentID), nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var response struct {
		Data []studentGuardianRow `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	return response.Data
}

// guardianExists reports whether GET /{id} still finds the guardian.
func (c *guardianCompositionContext) guardianExists(t *testing.T, guardianID int64) bool {
	t.Helper()
	code := c.status(t, testutil.AdminTestClaims(999), http.MethodGet, fmt.Sprintf("/%d", guardianID), nil)
	require.Contains(t, []int{http.StatusOK, http.StatusNotFound}, code)
	return code == http.StatusOK
}

// createLinkedGuardian creates a guardian profile linked to a fresh student
// and returns both IDs plus the link id.
func (c *guardianCompositionContext) createLinkedGuardian(t *testing.T, emailSeed string) (guardianID, studentID, linkID int64) {
	t.Helper()
	guardianID, _ = c.createGuardian(emailSeed)
	studentID, _ = c.createStudent("Linked", "Child", "1a")
	linkID = c.link(studentID, guardianID, "custom")
	return guardianID, studentID, linkID
}

// ===========================================================================
// LIST / GET / CREATE / UPDATE
// ===========================================================================

func TestGuardianComposition_ListGuardiansReturnsArray(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)

	rr := ctx.do(t, testutil.DefaultTestClaims(), http.MethodGet, "/", nil)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	dataArray(t, rr.Body.Bytes())

	rr = ctx.do(t, testutil.DefaultTestClaims(), http.MethodGet, "/?search=test", nil)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGuardianComposition_GetGuardianNotFoundAndInvalidID(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)

	testutil.AssertNotFound(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodGet, "/99999", nil))
	testutil.AssertBadRequest(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodGet, "/invalid", nil))
}

func TestGuardianComposition_CreateGuardian(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	admin := testutil.AdminTestClaims(999)

	t.Run("admin creates a full profile", func(t *testing.T) {
		rr := ctx.do(t, admin, http.MethodPost, "/", map[string]any{
			"first_name":               fmt.Sprintf("TestGuardian-%d", testpkg.UniqueSuffix()),
			"last_name":                "Test",
			"email":                    fmt.Sprintf("guardian-%d@test.com", testpkg.UniqueSuffix()),
			"preferred_contact_method": "email",
			"language_preference":      "de",
		})
		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
		assert.NotZero(t, dataObject(t, rr.Body.Bytes())["id"])
	})

	t.Run("non-staff with users:create is forbidden", func(t *testing.T) {
		rr := ctx.do(t, nonStaffClaims(t, "users:create"), http.MethodPost, "/", map[string]any{
			"first_name": "Test", "last_name": "Guardian", "email": "test@test.com",
			"preferred_contact_method": "email", "language_preference": "de",
		})
		testutil.AssertForbidden(t, rr)
	})

	// Guardian names are optional (CSV imports may only carry a relationship
	// type) and phone numbers are added separately.
	for name, body := range map[string]map[string]any{
		"missing first name": {"last_name": "Test", "email": fmt.Sprintf("no-firstname-%d@test.com", testpkg.UniqueSuffix()), "preferred_contact_method": "email", "language_preference": "de"},
		"missing last name":  {"first_name": "Test", "email": fmt.Sprintf("no-lastname-%d@test.com", testpkg.UniqueSuffix()), "preferred_contact_method": "email", "language_preference": "de"},
		"without contact":    {"first_name": fmt.Sprintf("NoContact-%d", testpkg.UniqueSuffix()), "last_name": "Guardian", "preferred_contact_method": "email", "language_preference": "de"},
	} {
		t.Run(name, func(t *testing.T) {
			testutil.AssertSuccessResponse(t, ctx.do(t, admin, http.MethodPost, "/", body), http.StatusCreated)
		})
	}
}

func TestGuardianComposition_CreateGuardianDuplicateEmailIsBadRequest(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	_, email := ctx.createGuardian("duplicate")

	rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodPost, "/", map[string]any{
		"first_name": "Second", "last_name": "Person", "email": email,
	})
	testutil.AssertBadRequest(t, rr)
	assert.Contains(t, errorText(t, rr.Body.String()), "bereits vergeben")
	assert.Equal(t, 1, ctx.guardianEmailCount(email))
}

func TestGuardianComposition_UpdateGuardian(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	body := map[string]any{"first_name": "Updated"}

	testutil.AssertForbidden(t, ctx.do(t, nonStaffClaims(t, "users:update"), http.MethodPut, "/99999", body))
	testutil.AssertNotFound(t, ctx.do(t, testutil.AdminTestClaims(999), http.MethodPut, "/99999", body))
	testutil.AssertBadRequest(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodPut, "/invalid", body))

	guardianID, _ := ctx.createGuardian("update-me")
	rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodPut, fmt.Sprintf("/%d", guardianID), body)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	data := dataObject(t, rr.Body.Bytes())
	assert.Equal(t, "Updated", data["first_name"])
	assert.Equal(t, "Test", data["last_name"], "fields left out of the partial update keep their value")
}

// ===========================================================================
// DELETE
// ===========================================================================

func TestGuardianComposition_DeleteGuardianGates(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)

	testutil.AssertForbidden(t, ctx.do(t, nonStaffClaims(t, "users:delete"), http.MethodDelete, "/99999", nil))
	testutil.AssertNotFound(t, ctx.do(t, testutil.AdminTestClaims(999), http.MethodDelete, "/99999", nil))
	testutil.AssertBadRequest(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodDelete, "/invalid", nil))
	testutil.AssertNotFound(t, ctx.do(t, testutil.AdminTestClaims(999), http.MethodGet, "/99999/delete-preview", nil))
}

func TestGuardianComposition_DeletePreviewIncludesAffectedLinkIDs(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	guardianID, _, linkID := ctx.createLinkedGuardian(t, "delete-preview")

	rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodGet, fmt.Sprintf("/%d/delete-preview", guardianID), nil)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	data := dataObject(t, rr.Body.Bytes())
	assert.Equal(t, float64(1), data["linked_count"])
	assert.Len(t, data["affected_names"], 1)
	assert.Equal(t, []any{fmt.Sprintf("%d", linkID)}, data["affected_link_ids"], "link ids travel as strings")
	assert.Contains(t, data["warning"], "nur mit diesem Kind")

	t.Run("non-admin may not preview", func(t *testing.T) {
		accountID := ctx.staffAccount("Preview", "Staff")
		testutil.AssertForbidden(t, ctx.do(t, guardianStaffClaims(accountID, "users:delete"), http.MethodGet, fmt.Sprintf("/%d/delete-preview", guardianID), nil))
	})
}

// A guardian that is still linked to a student is refused without
// ?force=true (409) rather than silently unlinking siblings (#819).
func TestGuardianComposition_DeleteLinkedGuardianConflict(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	guardianID, _, _ := ctx.createLinkedGuardian(t, "delete-conflict")
	siblingID, _ := ctx.createStudent("Linked", "Sibling", "1a")
	ctx.link(siblingID, guardianID, "custom")

	rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodDelete, fmt.Sprintf("/%d", guardianID), nil)
	testutil.AssertErrorResponse(t, rr, http.StatusConflict)
	assert.Contains(t, errorText(t, rr.Body.String()), "Linked Child")
	assert.True(t, ctx.guardianExists(t, guardianID), "guardian must survive the refused delete")
}

func TestGuardianComposition_DeleteLinkedGuardianNonAdminConflictHidesNames(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	teacherID, accountID := ctx.teacherAccount("Delete", "Supervisor")
	guardianID, _ := ctx.createGuardian("delete-privacy")
	studentID, _ := ctx.createStudent("Private", "Child", "1a")
	ctx.assignGroup(studentID, teacherID, "DeletePrivacy")
	ctx.link(studentID, guardianID, "custom")

	rr := ctx.do(t, withPerms(testutil.TeacherTestClaims(int(accountID)), "users:delete"), http.MethodDelete, fmt.Sprintf("/%d", guardianID), nil)
	testutil.AssertErrorResponse(t, rr, http.StatusConflict)
	text := errorText(t, rr.Body.String())
	assert.Contains(t, text, "Noch mit Kindern verknüpft")
	assert.NotContains(t, text, "Private")
	assert.NotContains(t, text, "Child")
}

func TestGuardianComposition_ForceDeleteByAdminRemovesGuardianAndLinks(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	guardianID, studentID, linkID := ctx.createLinkedGuardian(t, "delete-force")

	rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodDelete, fmt.Sprintf("/%d?force=true&expected_link_ids=%d", guardianID, linkID), nil)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.False(t, ctx.guardianExists(t, guardianID), "guardian is gone")
	assert.Empty(t, ctx.studentGuardians(t, studentID), "the link is gone with it")
}

func TestGuardianComposition_ForceDeleteRejectsStalePreview(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	guardianID, _, _ := ctx.createLinkedGuardian(t, "delete-force-stale")

	rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodDelete, fmt.Sprintf("/%d?force=true&expected_link_ids=999999", guardianID), nil)
	testutil.AssertErrorResponse(t, rr, http.StatusConflict)
	assert.Contains(t, errorText(t, rr.Body.String()), "Vorschau")
	assert.True(t, ctx.guardianExists(t, guardianID))

	t.Run("malformed expected ids are a bad request", func(t *testing.T) {
		rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodDelete, fmt.Sprintf("/%d?force=true&expected_link_ids=abc", guardianID), nil)
		testutil.AssertBadRequest(t, rr)
	})
}

// A non-admin supervisor cannot full-delete a linked guardian even with
// ?force=true: the blast radius reaches siblings they may not supervise.
func TestGuardianComposition_ForceDeleteByNonAdminIsForbidden(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	teacherID, accountID := ctx.teacherAccount("Force", "Supervisor")
	guardianID, _ := ctx.createGuardian("force-forbidden")
	studentID, _ := ctx.createStudent("Force", "Child", "1a")
	ctx.assignGroup(studentID, teacherID, "ForceForbidden")
	ctx.link(studentID, guardianID, "custom")

	rr := ctx.do(t, withPerms(testutil.TeacherTestClaims(int(accountID)), "users:delete"), http.MethodDelete, fmt.Sprintf("/%d?force=true", guardianID), nil)
	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
	assert.True(t, ctx.guardianExists(t, guardianID))
}

// ===========================================================================
// SPECIAL LISTS AND INVITATIONS
// ===========================================================================

func TestGuardianComposition_SpecialListsReturnArrays(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)

	for _, path := range []string{"/without-account", "/invitable", "/invitations/pending"} {
		rr := ctx.do(t, testutil.DefaultTestClaims(), http.MethodGet, path, nil)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
		dataArray(t, rr.Body.Bytes())
	}
}

func TestGuardianComposition_SendInvitationGates(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)

	testutil.AssertBadRequest(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodPost, "/invalid/invite", nil))

	// Without an Authorization header the Verifier/Authenticator rejects the
	// request before the handler runs.
	req := testutil.NewJSONRequest(t, http.MethodPost, guardiansPath("/1/invite"), nil)
	testutil.AssertUnauthorized(t, testutil.ExecuteRequest(ctx.router, req))
}

func TestGuardianComposition_SeedTokenHeaderDoesNotExposeTokenOutsideLocalDev(t *testing.T) {
	t.Parallel()
	for _, appEnv := range []string{"production", "staging", "preview", "developement", ""} {
		t.Run(fmt.Sprintf("app_env=%q", appEnv), func(t *testing.T) {
			t.Parallel()
			ctx := setupGuardiansCompositionRoute(t, appEnv)
			guardianID, _ := ctx.createGuardian("invite-" + appEnv)

			req := testutil.NewAuthenticatedRequest(t, http.MethodPost, guardiansPath("/%d/invite", guardianID), nil,
				bearer(t, withPerms(testutil.DefaultTestClaims(), "users:create")))
			req.Header.Set("X-Phoenix-Seed-Token", "true")
			rr := testutil.ExecuteRequest(ctx.router, req)

			testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
			data := dataObject(t, rr.Body.Bytes())
			assert.NotZero(t, data["id"])
			assert.NotContains(t, data, "token")
		})
	}
}

func TestGuardianComposition_SeedTokenExposedForLocalSeeder(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t, "development")
	guardianID, _ := ctx.createGuardian("invite-local")

	req := testutil.NewAuthenticatedRequest(t, http.MethodPost, guardiansPath("/%d/invite", guardianID), nil,
		bearer(t, withPerms(testutil.DefaultTestClaims(), "users:create")))
	req.Header.Set("X-Phoenix-Seed-Token", "true")
	req.Host = "localhost:8080"
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
	assert.NotEmpty(t, dataObject(t, rr.Body.Bytes())["token"], "the local seeder receives the raw token")
}

// ===========================================================================
// RELATIONSHIPS
// ===========================================================================

func TestGuardianComposition_StudentGuardianReads(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Guardian", "TestStudent", "1a")

	rr := ctx.do(t, testutil.DefaultTestClaims(), http.MethodGet, fmt.Sprintf("/students/%d/guardians", studentID), nil)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.Empty(t, dataArray(t, rr.Body.Bytes()))

	testutil.AssertBadRequest(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodGet, "/students/invalid/guardians", nil))
	testutil.AssertBadRequest(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodGet, "/invalid/students", nil))

	rr = ctx.do(t, testutil.DefaultTestClaims(), http.MethodGet, "/99999/students", nil)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.Empty(t, dataArray(t, rr.Body.Bytes()), "an unknown guardian has no children")
}

func TestGuardianComposition_GuardianStudentsCarryNamesAndLink(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	guardianID, _ := ctx.createGuardian("children")
	studentID, _ := ctx.createStudent("Mia", "Directory", "2b")
	linkID := ctx.link(studentID, guardianID, "primary_guardian")

	rr := ctx.do(t, testutil.DefaultTestClaims(), http.MethodGet, fmt.Sprintf("/%d/students", guardianID), nil)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	rows := dataArray(t, rr.Body.Bytes())
	require.Len(t, rows, 1)
	row := rows[0].(map[string]any)
	assert.Equal(t, "Mia", row["first_name"])
	assert.Equal(t, "Directory", row["last_name"])
	assert.Equal(t, "2b", row["school_class"])
	assert.Equal(t, float64(linkID), row["relationship_id"])
	assert.Equal(t, "primary_guardian", row["guardian_role"])
}

func TestGuardianComposition_AccountStatusReflectsPortalAccess(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Status", "Child", "1a")
	contactID, _ := ctx.createGuardian("status-contact")
	ctx.link(studentID, contactID, "pickup_only")

	rows := ctx.studentGuardians(t, studentID)
	require.Len(t, rows, 1)
	assert.Equal(t, "none", rows[0].AccountStatus, "no account, no invitation")
	assert.Equal(t, "pickup_only", rows[0].GuardianRole)
}

func TestGuardianComposition_AlumnusIsRejected(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Former", "Student", "4a")
	ctx.graduate(studentID)

	testutil.AssertNotFound(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodGet, fmt.Sprintf("/students/%d/guardians", studentID), nil))
	testutil.AssertForbidden(t, ctx.do(t, testutil.AdminTestClaims(999), http.MethodPost, fmt.Sprintf("/students/%d/guardians", studentID), map[string]any{}))
}

func TestGuardianComposition_LinkGuardianValidation(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	admin := testutil.AdminTestClaims(999)

	testutil.AssertForbidden(t, ctx.do(t, nonStaffClaims(t, "users:update"), http.MethodPost, "/students/1/guardians", map[string]any{
		"guardian_profile_id": 1, "relationship_type": "parent", "is_primary": true,
		"is_emergency_contact": true, "can_pickup": true, "emergency_priority": 1,
	}))
	testutil.AssertBadRequest(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodPost, "/students/invalid/guardians", map[string]any{
		"guardian_profile_id": 1, "relationship_type": "parent", "emergency_priority": 1,
	}))

	studentID, _ := ctx.createStudent("Link", "TestStudent", "1a")
	for name, body := range map[string]map[string]any{
		"missing guardian id":        {"relationship_type": "parent", "emergency_priority": 1},
		"missing relationship type":  {"guardian_profile_id": 1, "emergency_priority": 1},
		"invalid emergency priority": {"guardian_profile_id": 1, "relationship_type": "parent", "emergency_priority": 0},
	} {
		t.Run(name, func(t *testing.T) {
			testutil.AssertBadRequest(t, ctx.do(t, admin, http.MethodPost, fmt.Sprintf("/students/%d/guardians", studentID), body))
		})
	}
}

func TestGuardianComposition_LinkGuardianEchoesTheStoredLink(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	guardianID, _ := ctx.createGuardian("link-echo")
	studentID, _ := ctx.createStudent("Link", "Echo", "1a")

	rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodPost, fmt.Sprintf("/students/%d/guardians", studentID), map[string]any{
		"guardian_profile_id": guardianID, "relationship_type": "parent", "guardian_role": "primary_guardian",
		"is_primary": true, "can_pickup": true, "emergency_priority": 1,
	})
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
	data := dataObject(t, rr.Body.Bytes())
	assert.Equal(t, float64(studentID), data["student_id"])
	assert.Equal(t, float64(guardianID), data["guardian_profile_id"])
	assert.Equal(t, "primary_guardian", data["guardian_role"])
	assert.Equal(t, true, data["can_pickup"])
	permissions, ok := data["permissions"].(map[string]any)
	require.True(t, ok, "the stored link carries its portal permissions: %s", rr.Body.String())
	assert.Equal(t, true, permissions["parent_portal.access"])

	// Re-linking is idempotent and returns the existing link unchanged.
	rr = ctx.do(t, testutil.AdminTestClaims(999), http.MethodPost, fmt.Sprintf("/students/%d/guardians", studentID), map[string]any{
		"guardian_profile_id": guardianID, "relationship_type": "parent", "guardian_role": "pickup_only", "emergency_priority": 1,
	})
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
	assert.Equal(t, "primary_guardian", dataObject(t, rr.Body.Bytes())["guardian_role"])
}

func TestGuardianComposition_RelationshipUpdateGates(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	body := map[string]any{"is_primary": true}

	testutil.AssertBadRequest(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodPut, "/relationships/invalid", body))
	testutil.AssertNotFound(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodPut, "/relationships/99999", body))
}

func TestGuardianComposition_RemoveGuardianGates(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)

	testutil.AssertForbidden(t, ctx.do(t, nonStaffClaims(t, "users:update"), http.MethodDelete, "/students/1/guardians/1", nil))
	testutil.AssertBadRequest(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodDelete, "/students/invalid/guardians/1", nil))
	testutil.AssertBadRequest(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodDelete, "/students/1/guardians/invalid", nil))
}

// ===========================================================================
// PHONE NUMBERS
// ===========================================================================

// createGuardianWithPhones creates a guardian through the API with a primary
// and a secondary phone number.
func (c *guardianCompositionContext) createGuardianWithPhones(t *testing.T) (guardianID, phone1ID, phone2ID int64) {
	t.Helper()
	admin := testutil.AdminTestClaims(999)
	rr := c.do(t, admin, http.MethodPost, "/", map[string]any{
		"first_name":               fmt.Sprintf("TestGuardian-%d", testpkg.UniqueSuffix()),
		"last_name":                "PhoneTest",
		"email":                    fmt.Sprintf("phone-%d@test.com", testpkg.UniqueSuffix()),
		"preferred_contact_method": "phone",
		"language_preference":      "de",
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	guardianID = int64(dataObject(t, rr.Body.Bytes())["id"].(float64))

	rr = c.do(t, admin, http.MethodPost, fmt.Sprintf("/%d/phone-numbers", guardianID), map[string]any{
		"phone_number": "+49 123 456789", "phone_type": "mobile", "is_primary": true,
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	phone1ID = int64(dataObject(t, rr.Body.Bytes())["id"].(float64))

	rr = c.do(t, admin, http.MethodPost, fmt.Sprintf("/%d/phone-numbers", guardianID), map[string]any{
		"phone_number": "+49 987 654321", "phone_type": "work", "is_primary": false,
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	phone2ID = int64(dataObject(t, rr.Body.Bytes())["id"].(float64))
	return guardianID, phone1ID, phone2ID
}

func TestGuardianComposition_ListPhoneNumbers(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	guardianID, _, _ := ctx.createGuardianWithPhones(t)
	reader := withPerms(testutil.DefaultTestClaims(), "users:read")

	rr := ctx.do(t, reader, http.MethodGet, fmt.Sprintf("/%d/phone-numbers", guardianID), nil)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.Len(t, dataArray(t, rr.Body.Bytes()), 2)

	testutil.AssertBadRequest(t, ctx.do(t, reader, http.MethodGet, "/invalid/phone-numbers", nil))

	emptyID, _ := ctx.createGuardian("no-phone")
	rr = ctx.do(t, reader, http.MethodGet, fmt.Sprintf("/%d/phone-numbers", emptyID), nil)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.Empty(t, dataArray(t, rr.Body.Bytes()))
}

func TestGuardianComposition_AddPhoneNumber(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	admin := testutil.AdminTestClaims(999)
	guardianID, _ := ctx.createGuardian("add-phone")

	rr := ctx.do(t, admin, http.MethodPost, fmt.Sprintf("/%d/phone-numbers", guardianID), map[string]any{
		"phone_number": "+49 123 456789", "phone_type": "mobile", "label": "Personal", "is_primary": true,
	})
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
	data := dataObject(t, rr.Body.Bytes())
	assert.Equal(t, "+49 123 456789", data["phone_number"])
	assert.Equal(t, "mobile", data["phone_type"])
	assert.Equal(t, "Personal", data["label"])
	assert.Equal(t, true, data["is_primary"])

	t.Run("defaults the type to mobile", func(t *testing.T) {
		rr := ctx.do(t, admin, http.MethodPost, fmt.Sprintf("/%d/phone-numbers", guardianID), map[string]any{
			"phone_number": "+49 123 456780", "is_primary": false,
		})
		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
		assert.Equal(t, "mobile", dataObject(t, rr.Body.Bytes())["phone_type"])
	})

	testutil.AssertForbidden(t, ctx.do(t, nonStaffClaims(t, "users:update"), http.MethodPost, "/1/phone-numbers", map[string]any{"phone_number": "+49 123 456789"}))
	testutil.AssertBadRequest(t, ctx.do(t, admin, http.MethodPost, "/invalid/phone-numbers", map[string]any{"phone_number": "+49 123 456789"}))
	testutil.AssertBadRequest(t, ctx.do(t, admin, http.MethodPost, fmt.Sprintf("/%d/phone-numbers", guardianID), map[string]any{"phone_type": "mobile"}))
	testutil.AssertBadRequest(t, ctx.do(t, admin, http.MethodPost, fmt.Sprintf("/%d/phone-numbers", guardianID), map[string]any{"phone_number": "+49 1", "phone_type": "invalid_type"}))
}

func TestGuardianComposition_UpdatePhoneNumber(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	admin := testutil.AdminTestClaims(999)
	guardianID, phone1ID, _ := ctx.createGuardianWithPhones(t)

	rr := ctx.do(t, admin, http.MethodPut, fmt.Sprintf("/%d/phone-numbers/%d", guardianID, phone1ID), map[string]any{
		"phone_number": "+49 111 222333", "phone_type": "home", "label": "Updated Label",
	})
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	data := dataObject(t, rr.Body.Bytes())
	assert.Equal(t, "+49 111 222333", data["phone_number"])
	assert.Equal(t, "home", data["phone_type"])
	assert.Equal(t, "Updated Label", data["label"])

	update := map[string]any{"phone_number": "+49 111 222333"}
	testutil.AssertBadRequest(t, ctx.do(t, admin, http.MethodPut, "/invalid/phone-numbers/1", update))
	testutil.AssertBadRequest(t, ctx.do(t, admin, http.MethodPut, fmt.Sprintf("/%d/phone-numbers/invalid", guardianID), update))
	testutil.AssertNotFound(t, ctx.do(t, admin, http.MethodPut, fmt.Sprintf("/%d/phone-numbers/99999", guardianID), update))
	testutil.AssertForbidden(t, ctx.do(t, nonStaffClaims(t, "users:update"), http.MethodPut, "/1/phone-numbers/1", update))
	testutil.AssertBadRequest(t, ctx.do(t, admin, http.MethodPut, fmt.Sprintf("/%d/phone-numbers/%d", guardianID, phone1ID), map[string]any{"phone_number": ""}))
	testutil.AssertBadRequest(t, ctx.do(t, admin, http.MethodPut, fmt.Sprintf("/%d/phone-numbers/%d", guardianID, phone1ID), map[string]any{"phone_type": "invalid_type"}))

	t.Run("a phone of another guardian is forbidden", func(t *testing.T) {
		otherID, _ := ctx.createGuardian("other-phone-owner")
		testutil.AssertForbidden(t, ctx.do(t, admin, http.MethodPut, fmt.Sprintf("/%d/phone-numbers/%d", otherID, phone1ID), update))
	})
}

func TestGuardianComposition_DeletePhoneNumber(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	admin := testutil.AdminTestClaims(999)
	guardianID, phone1ID, phone2ID := ctx.createGuardianWithPhones(t)

	testutil.AssertSuccessResponse(t, ctx.do(t, admin, http.MethodDelete, fmt.Sprintf("/%d/phone-numbers/%d", guardianID, phone2ID), nil), http.StatusOK)
	testutil.AssertBadRequest(t, ctx.do(t, admin, http.MethodDelete, "/invalid/phone-numbers/1", nil))
	testutil.AssertBadRequest(t, ctx.do(t, admin, http.MethodDelete, fmt.Sprintf("/%d/phone-numbers/invalid", guardianID), nil))
	testutil.AssertNotFound(t, ctx.do(t, admin, http.MethodDelete, fmt.Sprintf("/%d/phone-numbers/99999", guardianID), nil))
	testutil.AssertForbidden(t, ctx.do(t, nonStaffClaims(t, "users:update"), http.MethodDelete, "/1/phone-numbers/1", nil))

	otherID, _ := ctx.createGuardian("other-phone-delete")
	testutil.AssertForbidden(t, ctx.do(t, admin, http.MethodDelete, fmt.Sprintf("/%d/phone-numbers/%d", otherID, phone1ID), nil))
}

func TestGuardianComposition_SetPrimaryPhone(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	admin := testutil.AdminTestClaims(999)
	guardianID, phone1ID, phone2ID := ctx.createGuardianWithPhones(t)

	rr := ctx.do(t, admin, http.MethodPost, fmt.Sprintf("/%d/phone-numbers/%d/set-primary", guardianID, phone2ID), nil)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.Equal(t, true, dataObject(t, rr.Body.Bytes())["is_primary"])

	testutil.AssertBadRequest(t, ctx.do(t, admin, http.MethodPost, "/invalid/phone-numbers/1/set-primary", nil))
	testutil.AssertBadRequest(t, ctx.do(t, admin, http.MethodPost, fmt.Sprintf("/%d/phone-numbers/invalid/set-primary", guardianID), nil))
	testutil.AssertNotFound(t, ctx.do(t, admin, http.MethodPost, fmt.Sprintf("/%d/phone-numbers/99999/set-primary", guardianID), nil))
	testutil.AssertForbidden(t, ctx.do(t, nonStaffClaims(t, "users:update"), http.MethodPost, "/1/phone-numbers/1/set-primary", nil))

	otherID, _ := ctx.createGuardian("other-phone-primary")
	testutil.AssertForbidden(t, ctx.do(t, admin, http.MethodPost, fmt.Sprintf("/%d/phone-numbers/%d/set-primary", otherID, phone1ID), nil))
}

// ===========================================================================
// BATCH CREATE
// ===========================================================================

func TestGuardianComposition_BatchCreatesAndLinksGuardian(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Batch", "Child", "1a")

	rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodPost, fmt.Sprintf("/students/%d/guardians/batch", studentID), map[string]any{
		"guardians": []map[string]any{{
			"first_name": "Atomic", "last_name": "Guardian",
			"email":             "batch-guardian-" + time.Now().Format("150405.000000") + "@test.com",
			"relationship_type": "parent", "emergency_priority": 1,
			"phone_numbers": []map[string]any{{"phone_number": "+49 123 456", "phone_type": "mobile", "is_primary": true}},
		}},
	})
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	linked := ctx.studentGuardians(t, studentID)
	require.Len(t, linked, 1)
	assert.Equal(t, "Atomic", linked[0].Guardian.FirstName)
}

func TestGuardianComposition_BatchRejectsEmptyAndNonStaff(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Empty", "Batch", "1a")

	testutil.AssertBadRequest(t, ctx.do(t, testutil.AdminTestClaims(999), http.MethodPost, fmt.Sprintf("/students/%d/guardians/batch", studentID), map[string]any{"guardians": []map[string]any{}}))

	nonStaff := jwt.AppClaims{ID: 555, Sub: "nonstaff@test.com", TenantID: testpkg.Tenant(t), Roles: []string{"user"}, Permissions: []string{"users:read", "users:update"}}
	testutil.AssertForbidden(t, ctx.do(t, nonStaff, http.MethodPost, fmt.Sprintf("/students/%d/guardians/batch", studentID), map[string]any{
		"guardians": []map[string]any{{"first_name": "X", "last_name": "Y", "relationship_type": "parent", "emergency_priority": 1}},
	}))
}

func TestGuardianComposition_BatchValidationIsBadRequestAndWritesNothing(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Invalid", "Batch", "1a")
	email := fmt.Sprintf("invalid-batch-%d@test.com", testpkg.UniqueSuffix())

	rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodPost, fmt.Sprintf("/students/%d/guardians/batch", studentID), map[string]any{
		"guardians": []map[string]any{{
			"first_name": "Bad", "last_name": "Priority", "email": email,
			"relationship_type": "parent", "emergency_priority": 0,
		}},
	})
	testutil.AssertBadRequest(t, rr)
	assert.Contains(t, errorText(t, rr.Body.String()), "Notfall-Priorität")
	assert.Equal(t, 0, ctx.guardianEmailCount(email))
	assert.Empty(t, ctx.studentGuardians(t, studentID))
}
