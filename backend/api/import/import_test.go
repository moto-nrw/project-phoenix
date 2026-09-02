// Package importapi_test tests the import API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package importapi_test

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	importAPI "github.com/moto-nrw/project-phoenix/api/import"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults so jwt.MustNewTokenAuth (called by
// importAPI.Resource.Router) succeeds in CI environments without a populated
// .env. The student-import tests exercise the production Router() and mint real
// signed JWTs via testutil.MintTestJWT.
func init() {
	testutil.SeedTestJWTConfig()
}

// adminBearer returns a request option carrying a real signed admin JWT scoped
// to the given account. It replaces the old testutil.WithClaims context
// injection now that the student-import tests run through Resource.Router(),
// where the production JWT middleware chain runs and rejects unsigned requests.
// Admin claims carry admin:*, which satisfies the users:read / users:create
// permission gates on the import routes.
func adminBearer(t *testing.T, accountID int64) testutil.RequestOption {
	t.Helper()
	return testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.AdminTestClaims(int(accountID))))
}

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	student  func(context.Context, int64) (*users.Student, error)
	resource *importAPI.Resource
}

// setupImportRoute initializes test database, services, and resource.
func setupImportRoute(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	// Create import resource
	resource := importAPI.NewResource(svc.Import, svc.StaffImport, svc.ClassListImport, svc.Users, db)

	return &testContext{
		db:       db,
		student:  svc.Users.GetStudentByID,
		resource: resource,
	}
}

// =============================================================================
// DOWNLOAD TEMPLATE TESTS
// =============================================================================

func TestDownloadTemplate_NoAuth(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	// Use the full router which has JWT middleware
	router := ctx.resource.Router()

	// Request without JWT token should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students/template", nil)
	req.Header.Del("Authorization")

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing authentication")
}

func TestDownloadTemplate_CSV(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "Admin")

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/students/template?format=csv", nil,
		adminBearer(t, admin.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), ".csv")
}

func TestDownloadTemplate_XLSX(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "Admin2")

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/students/template?format=xlsx", nil,
		adminBearer(t, admin.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "spreadsheetml")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), ".xlsx")
}

func TestDownloadTemplate_DefaultFormat(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "Admin3")

	router := ctx.resource.Router()

	// No format parameter - should default to CSV
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students/template", nil,
		adminBearer(t, admin.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/csv")
}

// =============================================================================
// PREVIEW IMPORT TESTS
// =============================================================================

func TestPreviewImport_NoAuth(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	router := ctx.resource.Router()

	// Request without JWT token should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/students/preview", nil)
	req.Header.Del("Authorization")

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing authentication")
}

func TestPreviewImport_NoFile(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "Admin4")

	router := ctx.resource.Router()

	// Request without file upload
	req := testutil.NewAuthenticatedRequest(t, "POST", "/students/preview", nil,
		adminBearer(t, admin.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error for missing file
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusInternalServerError}, rr.Code)
}

// =============================================================================
// IMPORT STUDENTS TESTS
// =============================================================================

func TestImportStudents_NoAuth(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	router := ctx.resource.Router()

	// Request without JWT token should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/students/import", nil)
	req.Header.Del("Authorization")

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing authentication")
}

func TestImportStudents_NoFile(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "Admin5")

	router := ctx.resource.Router()

	// Request without file upload
	req := testutil.NewAuthenticatedRequest(t, "POST", "/students/import", nil,
		adminBearer(t, admin.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return error for missing file
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusInternalServerError}, rr.Code)
}

// =============================================================================
// TEMPLATE CONTENT TESTS
// =============================================================================

func TestDownloadTemplate_HasRequiredHeaders(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "Admin6")

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/students/template?format=csv", nil,
		adminBearer(t, admin.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Check that required headers are present
	body := rr.Body.String()
	assert.True(t, strings.Contains(body, "Vorname"), "Template should contain Vorname header")
	assert.True(t, strings.Contains(body, "Nachname"), "Template should contain Nachname header")
	assert.True(t, strings.Contains(body, "Klasse"), "Template should contain Klasse header")
}

func TestDownloadTemplate_CSVAdvertisesBirthdayFormats(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "AdminBirthday")

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/students/template?format=csv", nil,
		adminBearer(t, admin.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	body := rr.Body.String()
	assert.Contains(t, body, "15.08.2015")
	assert.Contains(t, body, "22.03.14")
}

// =============================================================================
// PREVIEW IMPORT WITH FILE TESTS
// =============================================================================

func TestPreviewImport_WithValidCSV(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Preview", "CSVTest")

	router := ctx.resource.Router()

	// Create CSV content with required headers
	csvContent := "Vorname,Nachname,Klasse\nMax,Mustermann,1a\nErika,Musterfrau,2b"

	// Create multipart form with file — use account ID (not teacher ID) since
	// the handler resolves account → person → staff for pickup schedule FK
	req := testutil.NewMultipartRequest(t, "POST", "/students/preview", "file", "students.csv", csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 200 with preview data or 400 for validation errors
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusBadRequest,
		"Expected 200 or 400, got %d: %s", rr.Code, rr.Body.String())
}

func TestPreviewImport_WithEmptyCSV(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Preview", "EmptyCSV")

	router := ctx.resource.Router()

	// Create empty CSV with headers only
	csvContent := "Vorname,Nachname,Klasse"

	req := testutil.NewMultipartRequest(t, "POST", "/students/preview", "file", "empty.csv", csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 200 with empty preview or 400 for no data
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusBadRequest,
		"Expected 200 or 400, got %d: %s", rr.Code, rr.Body.String())
}

func TestPreviewImport_WithMissingHeaders(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Preview", "MissingHeaders")

	router := ctx.resource.Router()

	// CSV missing required headers
	csvContent := "Name,Class\nMax,1a"

	req := testutil.NewMultipartRequest(t, "POST", "/students/preview", "file", "invalid.csv", csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Service accepts files without standard headers (maps by position)
	// So this may return 200 with validation errors in response body
	t.Logf("Missing headers response: %d - %s", rr.Code, rr.Body.String())
}

// =============================================================================
// IMPORT STUDENTS WITH FILE TESTS
// =============================================================================

func TestImportStudents_WithValidCSV(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "CSVTest")

	router := ctx.resource.Router()

	// Create CSV content with required headers
	csvContent := "Vorname,Nachname,Klasse\nImport,Student1,1a"

	req := testutil.NewMultipartRequest(t, "POST", "/students/import", "file", "students.csv", csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return 200 with import results or 400/500 for errors
	t.Logf("Import response: %d - %s", rr.Code, rr.Body.String())
}

func TestImportStudents_WithDuplicateData(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Import", "DupeTest")

	router := ctx.resource.Router()

	// CSV with duplicate entries
	csvContent := "Vorname,Nachname,Klasse\nDupe,Student,1a\nDupe,Student,1a"

	req := testutil.NewMultipartRequest(t, "POST", "/students/import", "file", "dupes.csv", csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Log the result - may succeed with warnings or fail
	t.Logf("Duplicate import response: %d - %s", rr.Code, rr.Body.String())
}

// TestImportStudents_PersistsBusPermission is a regression test for issue #1460:
// the "Bus" column was parsed and validated but never written to the student
// record, silently dropping the bus permission on import.
func TestImportStudents_PersistsBusPermission(t *testing.T) {
	t.Parallel()

	tc := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Import", "BusTest")

	router := tc.resource.Router()

	// CSV with Bus=Ja — the imported student must end up with bus = true.
	csvContent := "Vorname,Nachname,Klasse,Bus\nBuskind,Phase1Regression,1a,Ja"

	req := testutil.NewMultipartRequest(t, "POST", "/students/import", "file", "bus.csv", csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, rr.Code, "import should succeed: %s", rr.Body.String())

	// Read the imported student back and assert the bus permission was persisted.
	var student users.Student
	err := tc.db.NewSelect().
		Model(&student).
		ModelTableExpr(`users.students AS "student"`).
		Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Where(`"person".first_name = ?`, "Buskind").
		Where(`"person".last_name = ?`, "Phase1Regression").
		Scan(context.Background())
	require.NoError(t, err, "imported student should exist in the database")
	hydrated, err := tc.student(testpkg.Ctx(t), student.ID)
	require.NoError(t, err, "imported student should be readable through the repository")
	for _, day := range users.BusDayOrder {
		assert.True(t, hydrated.BusDays[day], "Bus permission from CSV (Ja) must enable %s", day)
	}
}

// TestImportStudents_PersistsDepartureFromGehweise verifies the unified per-day
// Gehweise columns (#1610) are parsed and persisted as departure_days, with the
// legacy bus_days/pickup_days mirrors derived from them.
func TestImportStudents_PersistsDepartureFromGehweise(t *testing.T) {
	t.Parallel()

	tc := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Import", "DepTest")

	router := tc.resource.Router()

	csvContent := "Vorname,Nachname,Klasse,Gehweise.Mo,Gehweise.Mi\nDeparture,GehweiseImport,1a,bus,abholung"
	req := testutil.NewMultipartRequest(t, "POST", "/students/import", "file", "dep.csv", csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, rr.Code, "import should succeed: %s", rr.Body.String())

	var student users.Student
	require.NoError(t, tc.db.NewSelect().
		Model(&student).
		ModelTableExpr(`users.students AS "student"`).
		Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Where(`"person".first_name = ?`, "Departure").
		Where(`"person".last_name = ?`, "GehweiseImport").
		Scan(context.Background()))
	hydrated, err := tc.student(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	assert.Equal(t, users.DepartureBus, hydrated.DepartureDays.ModeFor(users.PickupDayMonday))
	assert.Equal(t, users.DeparturePickup, hydrated.DepartureDays.ModeFor(users.PickupDayWednesday))
	assert.True(t, hydrated.BusDays[users.BusDayMonday], "derived bus_days mirror")
	assert.True(t, hydrated.PickupDays[users.PickupDayWednesday], "derived pickup_days mirror")
}

// TestImportStudents_LegacyTemplateStillImports verifies an OLD template (with
// the legacy "Bus" + "Abholstatus" columns and no Gehweise) is still processed
// into departure_days (#1610) — the documented backward-compatible path.
func TestImportStudents_LegacyTemplateStillImports(t *testing.T) {
	t.Parallel()

	tc := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Import", "LegacyTest")

	router := tc.resource.Router()

	// Bus=Ja with "Geht alleine" pickup folds to bus on every weekday.
	csvContent := "Vorname,Nachname,Klasse,Bus,Abholstatus\nLegacy,GehweiseFallback,1a,Ja,Geht alleine nach Hause"
	req := testutil.NewMultipartRequest(t, "POST", "/students/import", "file", "legacy.csv", csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, rr.Code, "legacy import should succeed: %s", rr.Body.String())

	var student users.Student
	require.NoError(t, tc.db.NewSelect().
		Model(&student).
		ModelTableExpr(`users.students AS "student"`).
		Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Where(`"person".first_name = ?`, "Legacy").
		Where(`"person".last_name = ?`, "GehweiseFallback").
		Scan(context.Background()))
	hydrated, err := tc.student(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	for _, day := range users.PickupDayOrder {
		assert.Equal(t, users.DepartureBus, hydrated.DepartureDays.ModeFor(day),
			"legacy Bus=Ja must fold to bus on %s", day)
	}
}

// TestImportStudents_PersistsEnrollmentDatesAndStatus verifies that the
// enrollment date range is imported and that a future enrollment start marks
// the student as pending (so the activate-students scheduler activates them
// later), while a past/current start stays active (issue #1460, phase 2a).
func TestImportStudents_PersistsEnrollmentDatesAndStatus(t *testing.T) {
	t.Parallel()

	tc := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Import", "EnrollTest")

	router := tc.resource.Router()

	csvContent := "Vorname,Nachname,Klasse,Einschreibung von,Einschreibung bis\n" +
		"Zukunft,EnrollRegression,1a,01.08.2099,31.07.2100\n" +
		"Aktiv,EnrollRegression,1a,01.08.2020,01.08.2099"

	req := testutil.NewMultipartRequest(t, "POST", "/students/import", "file", "enroll.csv", csvContent,
		adminBearer(t, account.ID),
	)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, rr.Code, "import should succeed: %s", rr.Body.String())

	read := func(firstName string) users.Student {
		var s users.Student
		err := tc.db.NewSelect().
			Model(&s).
			ModelTableExpr(`users.students AS "student"`).
			Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id`).
			Where(`"person".first_name = ?`, firstName).
			Where(`"person".last_name = ?`, "EnrollRegression").
			Scan(context.Background())
		require.NoError(t, err, "imported student %q should exist", firstName)
		return s
	}

	future := read("Zukunft")
	require.NotNil(t, future.EnrolledFrom, "enrolled_from must be persisted")
	assert.Equal(t, "2099-08-01", future.EnrolledFrom.Format("2006-01-02"))
	require.NotNil(t, future.EnrolledUntil, "enrolled_until must be persisted")
	assert.Equal(t, "2100-07-31", future.EnrolledUntil.Format("2006-01-02"))
	assert.Equal(t, users.StudentStatusPending, future.Status,
		"a future enrollment start must be imported as pending")

	active := read("Aktiv")
	require.NotNil(t, active.EnrolledFrom, "enrolled_from must be persisted")
	assert.Equal(t, "2020-08-01", active.EnrolledFrom.Format("2006-01-02"))
	assert.Equal(t, users.StudentStatusActive, active.Status,
		"a past/current enrollment start must stay active")
}

// TestImportStudents_PersistsConsentDates verifies that the explicit consent
// date columns (AGB, data processing, email contact, photo) are imported and
// that photo_consent_given_by is left NULL on import (issue #1460, phase 2b).
func TestImportStudents_PersistsConsentDates(t *testing.T) {
	t.Parallel()

	tc := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Import", "ConsentTest")

	router := tc.resource.Router()

	csvContent := "Vorname,Nachname,Klasse,AGB akzeptiert am,Datenverarbeitung akzeptiert am,E-Mail-Kontakt akzeptiert am,Foto-Einwilligung am\n" +
		"Consent,Phase2bRegression,1a,01.08.2024,02.08.2024,03.08.2024,04.08.2024"

	req := testutil.NewMultipartRequest(t, "POST", "/students/import", "file", "consent.csv", csvContent,
		adminBearer(t, account.ID),
	)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, rr.Code, "import should succeed: %s", rr.Body.String())

	var student users.Student
	err := tc.db.NewSelect().
		Model(&student).
		ModelTableExpr(`users.students AS "student"`).
		Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Where(`"person".first_name = ?`, "Consent").
		Where(`"person".last_name = ?`, "Phase2bRegression").
		Scan(context.Background())
	require.NoError(t, err, "imported student should exist")

	require.NotNil(t, student.AGBAcceptedAt, "AGB consent date must be persisted")
	assert.Equal(t, "2024-08-01", student.AGBAcceptedAt.Format("2006-01-02"))
	require.NotNil(t, student.DataProcessingAcceptedAt)
	assert.Equal(t, "2024-08-02", student.DataProcessingAcceptedAt.Format("2006-01-02"))
	require.NotNil(t, student.EmailContactAcceptedAt)
	assert.Equal(t, "2024-08-03", student.EmailContactAcceptedAt.Format("2006-01-02"))
	require.NotNil(t, student.PhotoConsentGivenAt)
	assert.Equal(t, "2024-08-04", student.PhotoConsentGivenAt.Format("2006-01-02"))
	assert.Nil(t, student.PhotoConsentGivenBy, "photo_consent_given_by must be left NULL on import")
}

// =============================================================================
// STAFF ID RESOLUTION TESTS
// =============================================================================

func TestPreviewImport_NoClaims(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	router := ctx.resource.Router()

	csvContent := "Vorname,Nachname,Klasse\nMax,Mustermann,1a"

	// Request without claims — getAccountIDFromContext should fail
	req := testutil.NewMultipartRequest(t, "POST", "/students/preview", "file", "students.csv", csvContent)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"Expected 401 when no JWT claims in context")
}

func TestPreviewImport_AccountWithoutPerson(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	// Create account without person/staff chain
	account := testpkg.CreateTestAccount(t, ctx.db, "noperson")

	router := ctx.resource.Router()

	csvContent := "Vorname,Nachname,Klasse\nMax,Mustermann,1a"

	req := testutil.NewMultipartRequest(t, "POST", "/students/preview", "file", "students.csv", csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"Expected 401 when account has no person record")
}

func TestPreviewImport_PersonWithoutStaff(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	// Create account + person but no staff record
	_, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "NoStaff", "User")

	router := ctx.resource.Router()

	csvContent := "Vorname,Nachname,Klasse\nMax,Mustermann,1a"

	req := testutil.NewMultipartRequest(t, "POST", "/students/preview", "file", "students.csv", csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"Expected 401 when person has no staff record")
}

func TestImportStudents_AccountWithoutPerson(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	// Create account without person/staff chain
	account := testpkg.CreateTestAccount(t, ctx.db, "noperson-import")

	router := ctx.resource.Router()

	csvContent := "Vorname,Nachname,Klasse\nMax,Mustermann,1a"

	req := testutil.NewMultipartRequest(t, "POST", "/students/import", "file", "students.csv", csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Import handler wraps staff resolution in WithTenantTx, so failure returns 500
	assert.Equal(t, http.StatusInternalServerError, rr.Code,
		"Expected 500 when account has no person record in import")
}

func TestImportStudents_PersonWithoutStaff(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	// Create account + person but no staff record
	_, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "NoStaff", "Import")

	router := ctx.resource.Router()

	csvContent := "Vorname,Nachname,Klasse\nMax,Mustermann,1a"

	req := testutil.NewMultipartRequest(t, "POST", "/students/import", "file", "students.csv", csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Import handler wraps staff resolution in WithTenantTx, so failure returns 500
	assert.Equal(t, http.StatusInternalServerError, rr.Code,
		"Expected 500 when person has no staff record in import")
}

// =============================================================================
// ROUTER TESTS
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	t.Parallel()

	ctx := setupImportRoute(t)

	router := ctx.resource.Router()
	assert.NotNil(t, router, "Router should return a valid chi.Router")
}

// =============================================================================
// STAFF (MITARBEITER) IMPORT TESTS (issue #1460, phase 3)
// =============================================================================

func TestDownloadStaffTemplate_CSV(t *testing.T) {
	t.Parallel()

	tc := setupImportRoute(t)

	router := chi.NewRouter()
	router.Get("/template", tc.resource.DownloadStaffTemplate)

	req, _ := http.NewRequest("GET", "/template?format=csv", nil)
	rr := testutil.ExecuteRequest(router, req)

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	for _, col := range []string{"Vorname", "Nachname", "Email", "Rolle"} {
		assert.Contains(t, body, col, "staff CSV template must advertise the %q column", col)
	}
	assert.Contains(t, body, "Betreuer", "staff CSV template should use a default role that exists")
	assert.NotContains(t, body, "Lehrer", "staff CSV template must not suggest a non-default role")
}

func TestDownloadStaffTemplate_XLSX(t *testing.T) {
	t.Parallel()

	tc := setupImportRoute(t)

	router := chi.NewRouter()
	router.Get("/template", tc.resource.DownloadStaffTemplate)

	req, _ := http.NewRequest("GET", "/template?format=xlsx", nil)
	rr := testutil.ExecuteRequest(router, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		rr.Header().Get("Content-Type"))
}

// TestImportStaff_CreatesInvitationForValidRole verifies that a staff row with a
// valid role creates an invitation, while a row with an unknown role is rejected
// (and never produces an invitation).
func TestImportStaff_CreatesInvitationForValidRole(t *testing.T) {
	t.Parallel()

	tc := setupImportRoute(t)

	// Role visible to the caller: claims are rebased onto this test's tenant.
	role := testpkg.CreateTestRoleForTenant(t, tc.db, "ImportRolle", testpkg.Tenant(t))
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "StaffImport", "Admin")

	unique := time.Now().UnixNano()
	validEmail := fmt.Sprintf("valid.staff.%d@example.com", unique)
	invalidEmail := fmt.Sprintf("invalid.staff.%d@example.com", unique)

	router := chi.NewRouter()
	router.Post("/import", tc.resource.ImportStaff)

	csvContent := "Vorname,Nachname,Email,Rolle,Position\n" +
		fmt.Sprintf("Valide,Person,%s,%s,Lehrkraft\n", validEmail, role.Name) +
		fmt.Sprintf("Falsche,Rolle,%s,GibtEsNicht,", invalidEmail)

	req := testutil.NewMultipartRequest(t, "POST", "/import", "file", "staff.csv", csvContent,
		testutil.WithClaims(t, testutil.AdminTestClaims(int(account.ID))),
	)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, rr.Code, "import should succeed: %s", rr.Body.String())

	// The valid row created exactly one invitation with the resolved role.
	validCount, err := tc.db.NewSelect().
		Table("auth.invitation_tokens").
		Where("LOWER(email) = LOWER(?)", validEmail).
		Where("role_id = ?", role.ID).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, validCount, "valid staff row must create an invitation with the resolved role")

	// The unknown-role row must NOT create an invitation.
	invalidCount, err := tc.db.NewSelect().
		Table("auth.invitation_tokens").
		Where("LOWER(email) = LOWER(?)", invalidEmail).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, invalidCount, "row with an unknown role must not create an invitation")
}

// TestImportStaff_AcceptsRoleDisplayName verifies that the German display name
// shown on the roles page (e.g. "Betreuer") is accepted in the CSV and resolves
// to the underlying system role ("user").
func TestImportStaff_AcceptsRoleDisplayName(t *testing.T) {
	t.Parallel()

	tc := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "StaffImport", "DisplayRole")

	email := fmt.Sprintf("display.role.%d@example.com", time.Now().UnixNano())

	router := chi.NewRouter()
	router.Post("/import", tc.resource.ImportStaff)

	// "Betreuer" is the German display name of the system role "user".
	csvContent := "Vorname,Nachname,Email,Rolle,Position\n" +
		fmt.Sprintf("Bea,Betreuerin,%s,Betreuer,", email)

	req := testutil.NewMultipartRequest(t, "POST", "/import", "file", "staff.csv", csvContent,
		testutil.WithClaims(t, testutil.AdminTestClaims(int(account.ID))),
	)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, rr.Code, "import should succeed: %s", rr.Body.String())

	var userRoleID int64
	err := tc.db.NewSelect().
		Table("auth.roles").
		Column("id").
		Where("LOWER(name) = 'user'").
		Where("tenant_id IS NULL").
		Limit(1).
		Scan(context.Background(), &userRoleID)
	require.NoError(t, err, "system 'user' role must exist")

	count, err := tc.db.NewSelect().
		Table("auth.invitation_tokens").
		Where("LOWER(email) = LOWER(?)", email).
		Where("role_id = ?", userRoleID).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count, "display name 'Betreuer' must resolve to the system 'user' role")
}

// TestPreviewStaffImport_ValidatesRows exercises the staff preview (dry-run)
// handler: a valid row, an unknown-role row (suggestions path) and a row with
// a missing name (required-field path). The preview persists nothing.
func TestPreviewStaffImport_ValidatesRows(t *testing.T) {
	t.Parallel()

	tc := setupImportRoute(t)

	role := testpkg.CreateTestRoleForTenant(t, tc.db, "PreviewRolle", testpkg.Tenant(t))
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "StaffPreview", "Admin")

	router := chi.NewRouter()
	router.Post("/preview", tc.resource.PreviewStaffImport)

	unique := time.Now().UnixNano()
	csvContent := "Vorname,Nachname,Email,Rolle,Position\n" +
		fmt.Sprintf("Gut,Person,preview.valid.%d@example.com,%s,Lehrkraft\n", unique, role.Name) +
		fmt.Sprintf("Schlecht,Rolle,preview.badrole.%d@example.com,GibtEsNicht,\n", unique) +
		fmt.Sprintf(",Ohne,preview.noname.%d@example.com,%s,", unique, role.Name)

	req := testutil.NewMultipartRequest(t, "POST", "/preview", "file", "staff.csv", csvContent,
		testutil.WithClaims(t, testutil.AdminTestClaims(int(account.ID))),
	)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, rr.Code, "preview should succeed: %s", rr.Body.String())

	// Dry-run must not create any invitations.
	count, err := tc.db.NewSelect().
		Table("auth.invitation_tokens").
		Where("LOWER(email) LIKE ?", fmt.Sprintf("preview.%%.%d@example.com", unique)).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count, "preview (dry-run) must not create invitations")
}

// TestStaffImport_UploadValidation covers the shared upload-validation error
// paths (missing file, wrong file type, unparseable content) on the staff
// endpoints.
func TestStaffImport_UploadValidation(t *testing.T) {
	t.Parallel()

	tc := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "StaffUpload", "Admin")
	claims := testutil.WithClaims(t, testutil.AdminTestClaims(int(account.ID)))

	t.Run("missing file is rejected", func(t *testing.T) {
		router := chi.NewRouter()
		router.Post("/preview", tc.resource.PreviewStaffImport)
		req := testutil.NewAuthenticatedRequest(t, "POST", "/preview", nil, claims)
		rr := testutil.ExecuteRequest(router, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "body: %s", rr.Body.String())
	})

	t.Run("invalid file type is rejected", func(t *testing.T) {
		router := chi.NewRouter()
		router.Post("/preview", tc.resource.PreviewStaffImport)
		req := testutil.NewMultipartRequest(t, "POST", "/preview", "file", "evil.png", "\x89PNG\r\n\x1a\n", claims)
		rr := testutil.ExecuteRequest(router, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "body: %s", rr.Body.String())
	})

	t.Run("missing required columns is rejected", func(t *testing.T) {
		router := chi.NewRouter()
		router.Post("/preview", tc.resource.PreviewStaffImport)
		// Valid CSV file, but the "Rolle" column is missing.
		csv := "Vorname,Nachname,Email\nAnna,Lehmann,anna@example.com"
		req := testutil.NewMultipartRequest(t, "POST", "/preview", "file", "staff.csv", csv, claims)
		rr := testutil.ExecuteRequest(router, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "body: %s", rr.Body.String())
	})

	t.Run("missing file on import is rejected", func(t *testing.T) {
		router := chi.NewRouter()
		router.Post("/import", tc.resource.ImportStaff)
		req := testutil.NewAuthenticatedRequest(t, "POST", "/import", nil, claims)
		rr := testutil.ExecuteRequest(router, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "body: %s", rr.Body.String())
	})
}

// =============================================================================
// IMPORT AUDIT REGRESSION TESTS (#2141)
// =============================================================================
// The detached RecordAudit goroutine used to run on context.Background(),
// outside the tenant transaction, so the INSERT into audit.data_imports failed
// with 42501 on the NOINHERIT phoenix_auth connection and every import left no
// GDPR trace. These tests pin the fix: a successful preview/import response
// implies a persisted audit row.

// findImportAuditRecords loads audit.data_imports rows for a filename.
func findImportAuditRecords(t *testing.T, db *bun.DB, filename string) []*auditModels.DataImport {
	t.Helper()
	var records []*auditModels.DataImport
	require.NoError(t, db.NewSelect().
		Model(&records).
		ModelTableExpr(`audit.data_imports AS "data_import"`).
		Where(`"data_import".filename = ?`, filename).
		Scan(context.Background()))
	return records
}

func TestPreviewImport_PersistsAuditRecord(t *testing.T) {
	t.Parallel()

	tc := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "AuditPrev", "Regression")

	filename := fmt.Sprintf("audit-preview-%d.csv", account.ID)

	csvContent := "Vorname,Nachname,Klasse\nMax,Mustermann,1a"
	req := testutil.NewMultipartRequest(t, "POST", "/students/preview", "file", filename, csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	records := findImportAuditRecords(t, tc.db, filename)
	require.Len(t, records, 1, "preview must persist exactly one audit.data_imports row")
	assert.Equal(t, "student", records[0].EntityType)
	assert.True(t, records[0].DryRun, "preview audit row must be marked dry_run")
	assert.Equal(t, account.ID, records[0].ImportedBy)
}

func TestImportStudents_PersistsAuditRecord(t *testing.T) {
	t.Parallel()

	tc := setupImportRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "AuditImp", "Regression")

	filename := fmt.Sprintf("audit-import-%d.csv", account.ID)

	firstName := fmt.Sprintf("AuditKind%d", account.ID)
	csvContent := fmt.Sprintf("Vorname,Nachname,Klasse\n%s,Regression,1a", firstName)
	req := testutil.NewMultipartRequest(t, "POST", "/students/import", "file", filename, csvContent,
		adminBearer(t, account.ID),
	)

	rr := testutil.ExecuteRequest(tc.resource.Router(), req)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	records := findImportAuditRecords(t, tc.db, filename)
	require.Len(t, records, 1, "import must persist exactly one audit.data_imports row")
	assert.Equal(t, "student", records[0].EntityType)
	assert.False(t, records[0].DryRun, "actual import audit row must not be dry_run")
	assert.Equal(t, account.ID, records[0].ImportedBy)
	assert.Equal(t, 1, records[0].CreatedCount)
}

// newMultipartRequestWithMode builds an upload request that also carries the
// "mode" form field of the import UI (#2600).
func newMultipartRequestWithMode(t *testing.T, target, fileName, content, mode string, opts ...testutil.RequestOption) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("mode", mode))
	fw, err := writer.CreateFormFile("file", fileName)
	require.NoError(t, err)
	_, err = fw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, target, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for _, opt := range opts {
		opt(req)
	}
	return req
}

// TestImportStudents_UpsertModeUpdatesExisting verifies the update path of the
// child import: a second upload in upsert mode changes the stored record
// instead of reporting a duplicate, and empty cells keep their values.
func TestImportStudents_UpsertModeUpdatesExisting(t *testing.T) {
	t.Parallel()
	tc := setupImportRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Upsert", "Admin")
	claims := adminBearer(t, account.ID)
	router := tc.resource.Router()

	unique := time.Now().UnixNano()
	// The birthday is the second key that survives the class change below.
	first := fmt.Sprintf("Vorname,Nachname,Klasse,Geburtstag,Straße,Ort,Datenschutz\nUps,Ert%d,1A,03.02.2018,Kinderweg 1,Köln,Ja\n", unique)
	rr := testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/students/import", "k.csv", first, "create", claims))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	// Create mode again: duplicate.
	rr = testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/students/preview", "k.csv", first, "create", claims))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "already_exists")

	// Upsert: class and city change, street stays.
	second := fmt.Sprintf("Vorname,Nachname,Klasse,Geburtstag,Ort,Datenschutz\nUps,Ert%d,2A,03.02.2018,Bonn,Ja\n", unique)
	rr = testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/students/preview", "k.csv", second, "upsert", claims))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"UpdatedCount":1`)
	rr = testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/students/import", "k.csv", second, "upsert", claims))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"UpdatedCount":1`)

	var stored struct {
		SchoolClass   string  `bun:"school_class"`
		AddressStreet *string `bun:"address_street"`
		AddressCity   *string `bun:"address_city"`
	}
	err := tc.db.NewSelect().Table("users.students").
		ColumnExpr("students.school_class, students.address_street, students.address_city").
		Join("JOIN users.persons AS p ON p.id = students.person_id").
		Where("p.last_name = ?", fmt.Sprintf("Ert%d", unique)).
		Scan(context.Background(), &stored)
	require.NoError(t, err)
	assert.Equal(t, "2A", stored.SchoolClass)
	require.NotNil(t, stored.AddressCity)
	assert.Equal(t, "Bonn", *stored.AddressCity)
	require.NotNil(t, stored.AddressStreet)
	assert.Equal(t, "Kinderweg 1", *stored.AddressStreet)

	// Unknown mode is a 400, never a silent create.
	rr = testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/students/preview", "k.csv", second, "merge", claims))
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

// TestImportStaff_FilesStammdatensatz verifies that the staff import creates
// the person and staff rows immediately (#2600): with an e-mail the invitation
// points at that person, without one the row is a directory entry only.
func TestImportStaff_FilesStammdatensatz(t *testing.T) {
	t.Parallel()
	tc := setupImportRoute(t)
	role := testpkg.CreateTestRoleForTenant(t, tc.db, "ImportStammdaten", testpkg.Tenant(t))
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "StaffImport", "Admin")
	router := chi.NewRouter()
	router.Post("/import", tc.resource.ImportStaff)

	unique := time.Now().UnixNano()
	email := fmt.Sprintf("stamm.%d@example.com", unique)
	csvContent := "Vorname,Nachname,Rolle,Email,Personalnummer,Wochenstunden,Ort\n" +
		fmt.Sprintf("Mit,Konto%d,%s,%s,PN-%d,39,Köln\n", unique, role.Name, email, unique) +
		fmt.Sprintf("Ohne,Konto%d,%s,,,8,Bonn\n", unique, role.Name)

	req := testutil.NewMultipartRequest(t, "POST", "/import", "file", "staff.csv", csvContent,
		testutil.WithClaims(t, testutil.AdminTestClaims(int(account.ID))))
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"CreatedCount":2`)

	type staffRow struct {
		FirstName       string   `bun:"first_name"`
		PersonID        int64    `bun:"person_id"`
		AccountID       *int64   `bun:"account_id"`
		PersonnelNumber *string  `bun:"personnel_number"`
		City            *string  `bun:"address_city"`
		WeeklyHours     *float64 `bun:"weekly_hours"`
	}
	var rows []staffRow
	err := tc.db.NewSelect().Table("users.staff").
		ColumnExpr("p.first_name, p.id AS person_id, p.account_id, staff.personnel_number, m.address_city, m.weekly_hours").
		Join("JOIN users.persons AS p ON p.id = staff.person_id").
		Join("LEFT JOIN users.staff_master_data AS m ON m.staff_id = staff.id").
		Where("p.last_name = ?", fmt.Sprintf("Konto%d", unique)).
		OrderExpr("p.first_name").
		Scan(context.Background(), &rows)
	require.NoError(t, err)
	require.Len(t, rows, 2, "both rows become staff records immediately")
	assert.Equal(t, "Mit", rows[0].FirstName)
	assert.Nil(t, rows[0].AccountID, "no account until the invitation is accepted")
	require.NotNil(t, rows[0].PersonnelNumber)
	assert.Equal(t, fmt.Sprintf("PN-%d", unique), *rows[0].PersonnelNumber)
	assert.Equal(t, "Köln", *rows[0].City)
	assert.InDelta(t, 39, *rows[0].WeeklyHours, 0.001)
	assert.Equal(t, "Ohne", rows[1].FirstName)
	assert.Equal(t, "Bonn", *rows[1].City)

	var invitedPersonID *int64
	err = tc.db.NewSelect().Table("auth.invitation_tokens").Column("person_id").
		Where("LOWER(email) = LOWER(?)", email).Limit(1).Scan(context.Background(), &invitedPersonID)
	require.NoError(t, err)
	require.NotNil(t, invitedPersonID, "the invitation remembers the imported person")
	assert.Equal(t, rows[0].PersonID, *invitedPersonID)

	invitations, err := tc.db.NewSelect().Table("auth.invitation_tokens").
		Where("LOWER(email) = ''").Where("created_at > now() - interval '1 minute'").Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, invitations, "no invitation without an e-mail")
}

// createOnlyBearer mints a JWT for a custom role that may file new staff
// (users:create) but holds none of the personnel permissions (#2906).
func createOnlyBearer(t *testing.T, accountID int64, extra ...string) testutil.RequestOption {
	t.Helper()
	claims := testutil.AdminTestClaims(int(accountID))
	claims.Roles = []string{"user"}
	claims.IsAdmin = false
	claims.Permissions = append([]string{"users:create"}, extra...)
	return testutil.WithJWTBearer(testutil.MintTestJWT(t, claims))
}

// TestStaffImport_UpdateModesRequirePersonnelPermissions pins that the staff
// importer changes existing records only for callers holding staff:manage and
// staff:stammdaten (#2906). users:create alone may preview and run create
// mode; update and upsert are refused before a row is looked at.
func TestStaffImport_UpdateModesRequirePersonnelPermissions(t *testing.T) {
	t.Parallel()
	tc := setupImportRoute(t)
	role := testpkg.CreateTestRoleForTenant(t, tc.db, "ImportModeRolle", testpkg.Tenant(t))
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "ImportMode", "Actor")
	router := tc.resource.Router()

	unique := time.Now().UnixNano()
	filename := fmt.Sprintf("modus-%d.csv", unique)
	csvContent := "Vorname,Nachname,Rolle\n" + fmt.Sprintf("Modus,Test%d,%s\n", unique, role.Name)
	createOnly := createOnlyBearer(t, account.ID)

	for _, mode := range []string{"update", "upsert"} {
		for _, path := range []string{"/teachers/preview", "/teachers/import"} {
			rr := testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, path, filename, csvContent, mode, createOnly))
			assert.Equal(t, http.StatusForbidden, rr.Code, "%s in %s mode with users:create only: %s", path, mode, rr.Body.String())
			assert.Contains(t, rr.Body.String(), "Nur neue anlegen", "the refusal tells the person what still works")
		}
	}

	rr := testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/teachers/preview", filename, csvContent, "create", createOnly))
	assert.Equal(t, http.StatusOK, rr.Code, "create mode stays open for users:create: %s", rr.Body.String())

	for _, partial := range [][]string{{"staff:manage"}, {"staff:stammdaten"}} {
		rr = testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/teachers/preview", filename, csvContent, "update", createOnlyBearer(t, account.ID, partial...)))
		assert.Equal(t, http.StatusForbidden, rr.Code, "one personnel permission is not enough (%v): %s", partial, rr.Body.String())
	}

	personnel := createOnlyBearer(t, account.ID, "staff:manage", "staff:stammdaten")
	rr = testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/teachers/preview", filename, csvContent, "update", personnel))
	assert.Equal(t, http.StatusOK, rr.Code, "both personnel permissions open update mode: %s", rr.Body.String())

	// A refused request is rolled back before its audit row: only the two
	// permitted previews leave a trace.
	assert.Len(t, findImportAuditRecords(t, tc.db, filename), 2)
}

// TestDownloadStaffTemplate_OpenToUsersCreate pins that the import page's
// only prerequisite, users:create, also fetches the template (#2906).
func TestDownloadStaffTemplate_OpenToUsersCreate(t *testing.T) {
	t.Parallel()
	tc := setupImportRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Template", "Creator")
	router := tc.resource.Router()

	req := httptest.NewRequest(http.MethodGet, "/teachers/template?format=csv", nil)
	createOnlyBearer(t, account.ID)(req)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Vorname")
}
