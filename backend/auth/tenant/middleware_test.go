package tenant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/betterauth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// =============================================================================
// parseFullName Tests
// =============================================================================

func TestParseFullName_EmptyString(t *testing.T) {
	firstName, lastName := parseFullName("")
	assert.Equal(t, "New", firstName)
	assert.Equal(t, "User", lastName)
}

func TestParseFullName_SingleWord(t *testing.T) {
	firstName, lastName := parseFullName("John")
	assert.Equal(t, "John", firstName)
	assert.Equal(t, "User", lastName)
}

func TestParseFullName_TwoWords(t *testing.T) {
	firstName, lastName := parseFullName("John Doe")
	assert.Equal(t, "John", firstName)
	assert.Equal(t, "Doe", lastName)
}

func TestParseFullName_MultipleWords(t *testing.T) {
	firstName, lastName := parseFullName("John Paul Smith")
	assert.Equal(t, "John", firstName)
	assert.Equal(t, "Paul Smith", lastName)
}

func TestParseFullName_LeadingTrailingWhitespace(t *testing.T) {
	firstName, lastName := parseFullName("  John Doe  ")
	assert.Equal(t, "John", firstName)
	assert.Equal(t, "Doe", lastName)
}

func TestParseFullName_OnlyWhitespace(t *testing.T) {
	firstName, lastName := parseFullName("   ")
	assert.Equal(t, "New", firstName)
	assert.Equal(t, "User", lastName)
}

func TestParseFullName_GermanName(t *testing.T) {
	firstName, lastName := parseFullName("Hans Müller")
	assert.Equal(t, "Hans", firstName)
	assert.Equal(t, "Müller", lastName)
}

func TestParseFullName_DoubleLastName(t *testing.T) {
	firstName, lastName := parseFullName("Maria Garcia-López")
	assert.Equal(t, "Maria", firstName)
	assert.Equal(t, "Garcia-López", lastName)
}

func TestParseFullName_SpaceAfterFirstName(t *testing.T) {
	firstName, lastName := parseFullName("John ")
	assert.Equal(t, "John", firstName)
	assert.Equal(t, "User", lastName)
}

func TestParseFullName_SpaceBeforeFirstName(t *testing.T) {
	firstName, lastName := parseFullName(" John")
	assert.Equal(t, "John", firstName)
	assert.Equal(t, "User", lastName)
}

// =============================================================================
// handleAuthError Tests
// =============================================================================

func TestHandleAuthError_NoSession(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	handleAuthError(w, r, betterauth.ErrNoSession, "test context")

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHandleAuthError_NoActiveOrg(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	handleAuthError(w, r, betterauth.ErrNoActiveOrg, "test context")

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	// ErrNoOrganization returns 401 Unauthorized (user needs to select an org)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHandleAuthError_MemberNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	handleAuthError(w, r, betterauth.ErrMemberNotFound, "test context")

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestHandleAuthError_UnknownError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	handleAuthError(w, r, assert.AnError, "test context")

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	// Unknown errors should return 401 Unauthorized
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// =============================================================================
// organizationWithHierarchy Tests
// =============================================================================

func TestOrganizationWithHierarchy_Fields(t *testing.T) {
	bueroID := "buero-123"
	bueroName := "Test Büro"

	org := &organizationWithHierarchy{
		ID:          "org-123",
		Name:        "Test OGS",
		Slug:        "test-ogs",
		TraegerID:   "traeger-456",
		TraegerName: "Test Träger",
		BueroID:     &bueroID,
		BueroName:   &bueroName,
	}

	assert.Equal(t, "org-123", org.ID)
	assert.Equal(t, "Test OGS", org.Name)
	assert.Equal(t, "test-ogs", org.Slug)
	assert.Equal(t, "traeger-456", org.TraegerID)
	assert.Equal(t, "Test Träger", org.TraegerName)
	assert.NotNil(t, org.BueroID)
	assert.Equal(t, "buero-123", *org.BueroID)
	assert.NotNil(t, org.BueroName)
	assert.Equal(t, "Test Büro", *org.BueroName)
}

func TestOrganizationWithHierarchy_NoBuero(t *testing.T) {
	org := &organizationWithHierarchy{
		ID:          "org-123",
		Name:        "Test OGS",
		Slug:        "test-ogs",
		TraegerID:   "traeger-456",
		TraegerName: "Test Träger",
		BueroID:     nil,
		BueroName:   nil,
	}

	assert.Nil(t, org.BueroID)
	assert.Nil(t, org.BueroName)
}

// =============================================================================
// TenantContext Tests
// =============================================================================

func TestTenantContext_SetAndGet(t *testing.T) {
	tc := &TenantContext{
		UserID:      "user-123",
		UserEmail:   "teacher@school.de",
		UserName:    "Maria Schmidt",
		OrgID:       "org-456",
		OrgName:     "OGS Musterstadt",
		OrgSlug:     "ogs-musterstadt",
		Role:        "admin",
		Permissions: []string{"groups:read", "groups:write"},
		TraegerID:   "traeger-789",
		TraegerName: "Musterträger",
	}

	assert.Equal(t, "user-123", tc.UserID)
	assert.Equal(t, "teacher@school.de", tc.UserEmail)
	assert.Equal(t, "Maria Schmidt", tc.UserName)
	assert.Equal(t, "org-456", tc.OrgID)
	assert.Equal(t, "OGS Musterstadt", tc.OrgName)
	assert.Equal(t, "ogs-musterstadt", tc.OrgSlug)
	assert.Equal(t, "admin", tc.Role)
	assert.Contains(t, tc.Permissions, "groups:read")
	assert.Contains(t, tc.Permissions, "groups:write")
	assert.Equal(t, "traeger-789", tc.TraegerID)
	assert.Equal(t, "Musterträger", tc.TraegerName)
}

func TestTenantContext_OptionalFields(t *testing.T) {
	staffID := int64(42)
	accountID := int64(100)
	bueroID := "buero-999"
	bueroName := "Test Büro"

	tc := &TenantContext{
		UserID:    "user-123",
		UserEmail: "test@test.de",
		StaffID:   &staffID,
		AccountID: &accountID,
		BueroID:   &bueroID,
		BueroName: &bueroName,
	}

	assert.NotNil(t, tc.StaffID)
	assert.Equal(t, int64(42), *tc.StaffID)
	assert.NotNil(t, tc.AccountID)
	assert.Equal(t, int64(100), *tc.AccountID)
	assert.NotNil(t, tc.BueroID)
	assert.Equal(t, "buero-999", *tc.BueroID)
	assert.NotNil(t, tc.BueroName)
	assert.Equal(t, "Test Büro", *tc.BueroName)
}

func TestTenantContext_NilOptionalFields(t *testing.T) {
	tc := &TenantContext{
		UserID:    "user-123",
		UserEmail: "test@test.de",
		StaffID:   nil,
		AccountID: nil,
		BueroID:   nil,
		BueroName: nil,
	}

	assert.Nil(t, tc.StaffID)
	assert.Nil(t, tc.AccountID)
	assert.Nil(t, tc.BueroID)
	assert.Nil(t, tc.BueroName)
}

// =============================================================================
// Error Response Tests
// =============================================================================

func TestErrUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	err := render.Render(w, r, ErrUnauthorized)
	assert.NoError(t, err)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestErrNoOrganization(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	err := render.Render(w, r, ErrNoOrganization)
	assert.NoError(t, err)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	// ErrNoOrganization returns 401 Unauthorized (user needs to select an org)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestErrInternalServer(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	err := render.Render(w, r, ErrInternalServer)
	assert.NoError(t, err)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestNewErrForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	errResp := NewErrForbidden("custom error message")
	err := render.Render(w, r, errResp)
	assert.NoError(t, err)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestNewErrUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	errResp := NewErrUnauthorized("custom unauthorized message")
	err := render.Render(w, r, errResp)
	assert.NoError(t, err)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestNewErrInternal(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	errResp := NewErrInternal("custom internal error")
	err := render.Render(w, r, errResp)
	assert.NoError(t, err)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// =============================================================================
// Mock BetterAuth Server
// =============================================================================

// mockBetterAuthServer creates a mock BetterAuth server that returns configured responses.
type mockBetterAuthServer struct {
	server         *httptest.Server
	sessionResp    *betterauth.SessionResponse
	sessionErr     int // HTTP status code for error
	memberResp     *betterauth.MemberInfo
	memberErr      int // HTTP status code for error
	sessionHandler func(w http.ResponseWriter, r *http.Request)
	memberHandler  func(w http.ResponseWriter, r *http.Request)
}

func newMockBetterAuthServer() *mockBetterAuthServer {
	mock := &mockBetterAuthServer{}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/get-session", func(w http.ResponseWriter, r *http.Request) {
		if mock.sessionHandler != nil {
			mock.sessionHandler(w, r)
			return
		}
		if mock.sessionErr != 0 {
			w.WriteHeader(mock.sessionErr)
			return
		}
		if mock.sessionResp != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mock.sessionResp)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	})

	mux.HandleFunc("/api/auth/organization/get-active-member", func(w http.ResponseWriter, r *http.Request) {
		if mock.memberHandler != nil {
			mock.memberHandler(w, r)
			return
		}
		if mock.memberErr != 0 {
			w.WriteHeader(mock.memberErr)
			return
		}
		if mock.memberResp != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mock.memberResp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	mock.server = httptest.NewServer(mux)
	return mock
}

func (m *mockBetterAuthServer) close() {
	m.server.Close()
}

func (m *mockBetterAuthServer) client() *betterauth.Client {
	return betterauth.NewClientWithURL(m.server.URL)
}

// =============================================================================
// Tenant Test DB Setup
// =============================================================================

// setupTenantTestDB sets up the test database with required tenant/organization data.
func setupTenantTestDB(t *testing.T) (*bun.DB, string, func()) {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a unique test Träger
	traegerID := testpkg.GenerateTestOGSID("traeger")
	_, err := db.NewRaw(`
		INSERT INTO tenant.traeger (id, name, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, traegerID, "Test Träger").Exec(ctx)
	require.NoError(t, err, "Failed to create test Träger")

	// Create a unique test OGS organization
	orgID := testpkg.GenerateTestOGSID("org")
	_, err = db.NewRaw(`
		INSERT INTO public.organization (id, name, slug, status, "createdAt", "traegerId")
		VALUES (?, ?, ?, 'active', NOW(), ?)
		ON CONFLICT (id) DO NOTHING
	`, orgID, "Test OGS", "test-ogs-"+orgID, traegerID).Exec(ctx)
	require.NoError(t, err, "Failed to create test organization")

	cleanup := func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()

		_, _ = db.NewRaw(`DELETE FROM public.organization WHERE id = ?`, orgID).Exec(cleanCtx)
		_, _ = db.NewRaw(`DELETE FROM tenant.traeger WHERE id = ?`, traegerID).Exec(cleanCtx)
		_ = db.Close()
	}

	return db, orgID, cleanup
}

// setupTenantTestDBWithBuero sets up tenant test data including a Büro.
func setupTenantTestDBWithBuero(t *testing.T) (*bun.DB, string, func()) {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a unique test Träger
	traegerID := testpkg.GenerateTestOGSID("traeger")
	_, err := db.NewRaw(`
		INSERT INTO tenant.traeger (id, name, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, traegerID, "Test Träger").Exec(ctx)
	require.NoError(t, err)

	// Create a unique test Büro
	bueroID := testpkg.GenerateTestOGSID("buero")
	_, err = db.NewRaw(`
		INSERT INTO tenant.buero (id, name, traeger_id, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, bueroID, "Test Büro", traegerID).Exec(ctx)
	require.NoError(t, err)

	// Create a unique test OGS organization with Büro
	orgID := testpkg.GenerateTestOGSID("org")
	_, err = db.NewRaw(`
		INSERT INTO public.organization (id, name, slug, status, "createdAt", "traegerId", "bueroId")
		VALUES (?, ?, ?, 'active', NOW(), ?, ?)
		ON CONFLICT (id) DO NOTHING
	`, orgID, "Test OGS", "test-ogs-"+orgID, traegerID, bueroID).Exec(ctx)
	require.NoError(t, err)

	cleanup := func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()

		_, _ = db.NewRaw(`DELETE FROM public.organization WHERE id = ?`, orgID).Exec(cleanCtx)
		_, _ = db.NewRaw(`DELETE FROM tenant.buero WHERE id = ?`, bueroID).Exec(cleanCtx)
		_, _ = db.NewRaw(`DELETE FROM tenant.traeger WHERE id = ?`, traegerID).Exec(cleanCtx)
		_ = db.Close()
	}

	return db, orgID, cleanup
}

// =============================================================================
// loadOrganization Integration Tests
// =============================================================================

func TestLoadOrganization_Success(t *testing.T) {
	db, orgID, cleanup := setupTenantTestDB(t)
	defer cleanup()

	ctx := context.Background()
	org, err := loadOrganization(ctx, db, orgID)

	require.NoError(t, err)
	assert.Equal(t, orgID, org.ID)
	assert.Equal(t, "Test OGS", org.Name)
	assert.NotEmpty(t, org.TraegerID)
	assert.Equal(t, "Test Träger", org.TraegerName)
	assert.Nil(t, org.BueroID)
	assert.Nil(t, org.BueroName)
}

func TestLoadOrganization_WithBuero(t *testing.T) {
	db, orgID, cleanup := setupTenantTestDBWithBuero(t)
	defer cleanup()

	ctx := context.Background()
	org, err := loadOrganization(ctx, db, orgID)

	require.NoError(t, err)
	assert.Equal(t, orgID, org.ID)
	assert.NotNil(t, org.BueroID)
	assert.NotNil(t, org.BueroName)
	assert.Equal(t, "Test Büro", *org.BueroName)
}

func TestLoadOrganization_NotFound(t *testing.T) {
	db, _, cleanup := setupTenantTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, err := loadOrganization(ctx, db, "nonexistent-org-id")

	assert.ErrorIs(t, err, ErrOrgNotFound)
}

// =============================================================================
// lookupStaffByBetterAuthUserID Integration Tests
// =============================================================================

func TestLookupStaffByBetterAuthUserID_FoundByUserID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ogsID := testpkg.SetupTestOGS(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff", ogsID)

	// Link staff to BetterAuth user
	ctx := context.Background()
	betterAuthUserID := "ba-user-" + testpkg.GenerateTestOGSID("test")
	_, err := db.NewRaw(`
		UPDATE users.staff SET betterauth_user_id = ? WHERE id = ?
	`, betterAuthUserID, staff.ID).Exec(ctx)
	require.NoError(t, err)

	// Lookup should find by user ID
	staffID, err := lookupStaffByBetterAuthUserID(ctx, db, betterAuthUserID, "test@example.com")

	require.NoError(t, err)
	require.NotNil(t, staffID)
	assert.Equal(t, staff.ID, *staffID)
}

func TestLookupStaffByBetterAuthUserID_FoundByEmailAndAutoLinks(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ogsID := testpkg.SetupTestOGS(t, db)

	// Create staff with account that has matching email
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Test", "StaffAutoLink", ogsID)

	// Lookup by email should find and auto-link
	ctx := context.Background()
	betterAuthUserID := "ba-user-" + testpkg.GenerateTestOGSID("test")
	staffID, err := lookupStaffByBetterAuthUserID(ctx, db, betterAuthUserID, account.Email)

	require.NoError(t, err)
	require.NotNil(t, staffID)
	assert.Equal(t, staff.ID, *staffID)

	// Verify auto-linking occurred
	var linkedUserID string
	err = db.NewRaw(`SELECT betterauth_user_id FROM users.staff WHERE id = ?`, staff.ID).Scan(ctx, &linkedUserID)
	require.NoError(t, err)
	assert.Equal(t, betterAuthUserID, linkedUserID)
}

func TestLookupStaffByBetterAuthUserID_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	staffID, err := lookupStaffByBetterAuthUserID(ctx, db, "nonexistent-user-id", "nonexistent@example.com")

	require.NoError(t, err)
	assert.Nil(t, staffID)
}

func TestLookupStaffByBetterAuthUserID_EmptyEmail(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	staffID, err := lookupStaffByBetterAuthUserID(ctx, db, "nonexistent-user-id", "")

	require.NoError(t, err)
	assert.Nil(t, staffID)
}

func TestLookupStaffByBetterAuthUserID_AlreadyLinked(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ogsID := testpkg.SetupTestOGS(t, db)

	// Create staff with account that has matching email
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Test", "AlreadyLinked", ogsID)

	// Pre-link staff to a different BetterAuth user
	ctx := context.Background()
	existingBAUserID := "existing-ba-user-" + testpkg.GenerateTestOGSID("test")
	_, err := db.NewRaw(`
		UPDATE users.staff SET betterauth_user_id = ? WHERE id = ?
	`, existingBAUserID, staff.ID).Exec(ctx)
	require.NoError(t, err)

	// Lookup by email should not find anything since staff is already linked
	newBAUserID := "new-ba-user-" + testpkg.GenerateTestOGSID("test")
	staffID, err := lookupStaffByBetterAuthUserID(ctx, db, newBAUserID, account.Email)

	// Should return nil because email lookup only finds unlinked staff
	require.NoError(t, err)
	assert.Nil(t, staffID)
}

// =============================================================================
// ensureAccountAndPerson Integration Tests
// =============================================================================

func TestEnsureAccountAndPerson_EmptyEmail(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	accountID, err := ensureAccountAndPerson(ctx, db, "", "Test User", "test-ogs")

	require.NoError(t, err)
	assert.Nil(t, accountID)
}

func TestEnsureAccountAndPerson_ExistingAccountWithPerson(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ogsID := testpkg.SetupTestOGS(t, db)

	// Create existing account with person
	person, account := testpkg.CreateTestPersonWithAccount(t, db, "Existing", "User", ogsID)
	_ = person // person exists

	ctx := context.Background()
	accountID, err := ensureAccountAndPerson(ctx, db, account.Email, "Different Name", ogsID)

	require.NoError(t, err)
	require.NotNil(t, accountID)
	assert.Equal(t, account.ID, *accountID)
}

func TestEnsureAccountAndPerson_ExistingAccountWithoutPerson(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ogsID := testpkg.SetupTestOGS(t, db)

	// Create account without person
	account := testpkg.CreateTestAccount(t, db, "orphan-account")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	ctx := context.Background()
	accountID, err := ensureAccountAndPerson(ctx, db, account.Email, "JIT Provisioned User", ogsID)

	require.NoError(t, err)
	require.NotNil(t, accountID)
	assert.Equal(t, account.ID, *accountID)

	// Verify person was JIT provisioned
	var personExists bool
	err = db.NewRaw(`SELECT EXISTS(SELECT 1 FROM users.persons WHERE account_id = ?)`, account.ID).Scan(ctx, &personExists)
	require.NoError(t, err)
	assert.True(t, personExists)
}

func TestEnsureAccountAndPerson_NewAccountAndPerson(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ogsID := testpkg.SetupTestOGS(t, db)

	ctx := context.Background()
	email := "new-jit-user-" + testpkg.GenerateTestOGSID("test") + "@example.com"
	accountID, err := ensureAccountAndPerson(ctx, db, email, "New JIT User", ogsID)

	require.NoError(t, err)
	require.NotNil(t, accountID)

	// Cleanup
	defer func() {
		_, _ = db.NewRaw(`DELETE FROM users.persons WHERE account_id = ?`, *accountID).Exec(ctx)
		_, _ = db.NewRaw(`DELETE FROM auth.accounts WHERE id = ?`, *accountID).Exec(ctx)
	}()

	// Verify account was created
	var accountEmail string
	err = db.NewRaw(`SELECT email FROM auth.accounts WHERE id = ?`, *accountID).Scan(ctx, &accountEmail)
	require.NoError(t, err)
	assert.Contains(t, accountEmail, "new-jit-user")

	// Verify person was created
	var firstName, lastName string
	err = db.NewRaw(`SELECT first_name, last_name FROM users.persons WHERE account_id = ?`, *accountID).Scan(ctx, &firstName, &lastName)
	require.NoError(t, err)
	assert.Equal(t, "New", firstName)
	assert.Equal(t, "JIT User", lastName)
}

func TestEnsureAccountAndPerson_NewAccountWithEmptyName(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ogsID := testpkg.SetupTestOGS(t, db)

	ctx := context.Background()
	email := "jit-empty-name-" + testpkg.GenerateTestOGSID("test") + "@example.com"
	accountID, err := ensureAccountAndPerson(ctx, db, email, "", ogsID)

	require.NoError(t, err)
	require.NotNil(t, accountID)

	// Cleanup
	defer func() {
		_, _ = db.NewRaw(`DELETE FROM users.persons WHERE account_id = ?`, *accountID).Exec(ctx)
		_, _ = db.NewRaw(`DELETE FROM auth.accounts WHERE id = ?`, *accountID).Exec(ctx)
	}()

	// Verify person was created with default name
	var firstName, lastName string
	err = db.NewRaw(`SELECT first_name, last_name FROM users.persons WHERE account_id = ?`, *accountID).Scan(ctx, &firstName, &lastName)
	require.NoError(t, err)
	assert.Equal(t, "New", firstName)
	assert.Equal(t, "User", lastName)
}

// =============================================================================
// Middleware Integration Tests
// =============================================================================

func TestMiddleware_NoSession(t *testing.T) {
	mock := newMockBetterAuthServer()
	defer mock.close()

	// Return 401 for session request
	mock.sessionErr = http.StatusUnauthorized

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMiddleware_NoActiveOrganization(t *testing.T) {
	mock := newMockBetterAuthServer()
	defer mock.close()

	// Return session without active org
	mock.sessionResp = &betterauth.SessionResponse{
		User: betterauth.UserInfo{
			ID:    "user-1",
			Email: "test@example.com",
			Name:  "Test User",
		},
		Session: betterauth.SessionInfo{
			ID:                   "session-1",
			ActiveOrganizationID: "", // No active org
		},
	}

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "no organization selected")
}

func TestMiddleware_MemberNotFound(t *testing.T) {
	mock := newMockBetterAuthServer()
	defer mock.close()

	mock.sessionResp = &betterauth.SessionResponse{
		User: betterauth.UserInfo{
			ID:    "user-1",
			Email: "test@example.com",
			Name:  "Test User",
		},
		Session: betterauth.SessionInfo{
			ID:                   "session-1",
			ActiveOrganizationID: "org-1",
		},
	}
	mock.memberErr = http.StatusNotFound

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestMiddleware_OrganizationNotFound(t *testing.T) {
	mock := newMockBetterAuthServer()
	defer mock.close()

	mock.sessionResp = &betterauth.SessionResponse{
		User: betterauth.UserInfo{
			ID:    "user-1",
			Email: "test@example.com",
			Name:  "Test User",
		},
		Session: betterauth.SessionInfo{
			ID:                   "session-1",
			ActiveOrganizationID: "nonexistent-org",
		},
	}
	mock.memberResp = &betterauth.MemberInfo{
		ID:             "member-1",
		OrganizationID: "nonexistent-org",
		UserID:         "user-1",
		Role:           "supervisor",
	}

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestMiddleware_Success_SupervisorRole(t *testing.T) {
	db, orgID, cleanup := setupTenantTestDB(t)
	defer cleanup()

	mock := newMockBetterAuthServer()
	defer mock.close()

	userID := "user-" + testpkg.GenerateTestOGSID("test")
	mock.sessionResp = &betterauth.SessionResponse{
		User: betterauth.UserInfo{
			ID:    userID,
			Email: "supervisor@example.com",
			Name:  "Test Supervisor",
		},
		Session: betterauth.SessionInfo{
			ID:                   "session-1",
			ActiveOrganizationID: orgID,
		},
	}
	mock.memberResp = &betterauth.MemberInfo{
		ID:             "member-1",
		OrganizationID: orgID,
		UserID:         userID,
		Role:           "supervisor",
	}

	var capturedTC *TenantContext
	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		capturedTC = TenantFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, capturedTC)
	assert.Equal(t, userID, capturedTC.UserID)
	assert.Equal(t, "supervisor@example.com", capturedTC.UserEmail)
	assert.Equal(t, "Test Supervisor", capturedTC.UserName)
	assert.Equal(t, orgID, capturedTC.OrgID)
	assert.Equal(t, "supervisor", capturedTC.Role)
	assert.Contains(t, capturedTC.Permissions, "location:read")
	assert.NotEmpty(t, capturedTC.TraegerID)
}

func TestMiddleware_Success_OgsAdminRole(t *testing.T) {
	db, orgID, cleanup := setupTenantTestDB(t)
	defer cleanup()

	mock := newMockBetterAuthServer()
	defer mock.close()

	userID := "user-" + testpkg.GenerateTestOGSID("test")
	mock.sessionResp = &betterauth.SessionResponse{
		User: betterauth.UserInfo{
			ID:    userID,
			Email: "admin@example.com",
			Name:  "Test Admin",
		},
		Session: betterauth.SessionInfo{
			ID:                   "session-1",
			ActiveOrganizationID: orgID,
		},
	}
	mock.memberResp = &betterauth.MemberInfo{
		ID:             "member-1",
		OrganizationID: orgID,
		UserID:         userID,
		Role:           "ogsAdmin",
	}

	var capturedTC *TenantContext
	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		capturedTC = TenantFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, capturedTC)
	assert.Equal(t, "ogsAdmin", capturedTC.Role)
	assert.Contains(t, capturedTC.Permissions, "student:create")
	assert.Contains(t, capturedTC.Permissions, "staff:invite")
}

func TestMiddleware_Success_WithBuero(t *testing.T) {
	db, orgID, cleanup := setupTenantTestDBWithBuero(t)
	defer cleanup()

	mock := newMockBetterAuthServer()
	defer mock.close()

	userID := "user-" + testpkg.GenerateTestOGSID("test")
	mock.sessionResp = &betterauth.SessionResponse{
		User: betterauth.UserInfo{
			ID:    userID,
			Email: "buero@example.com",
			Name:  "Büro Admin",
		},
		Session: betterauth.SessionInfo{
			ID:                   "session-1",
			ActiveOrganizationID: orgID,
		},
	}
	mock.memberResp = &betterauth.MemberInfo{
		ID:             "member-1",
		OrganizationID: orgID,
		UserID:         userID,
		Role:           "bueroAdmin",
	}

	var capturedTC *TenantContext
	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		capturedTC = TenantFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, capturedTC)
	assert.NotNil(t, capturedTC.BueroID)
	assert.NotNil(t, capturedTC.BueroName)
	assert.Equal(t, "Test Büro", *capturedTC.BueroName)
	assert.NotContains(t, capturedTC.Permissions, "location:read") // GDPR: bueroAdmin has no location access
}

func TestMiddleware_Success_WithStaffLinkage(t *testing.T) {
	db, orgID, cleanup := setupTenantTestDB(t)
	defer cleanup()

	// Create staff linked to BetterAuth user
	ctx := context.Background()
	staff := testpkg.CreateTestStaff(t, db, "Linked", "Staff", orgID)
	betterAuthUserID := "ba-user-" + testpkg.GenerateTestOGSID("test")
	_, err := db.NewRaw(`
		UPDATE users.staff SET betterauth_user_id = ? WHERE id = ?
	`, betterAuthUserID, staff.ID).Exec(ctx)
	require.NoError(t, err)

	mock := newMockBetterAuthServer()
	defer mock.close()

	mock.sessionResp = &betterauth.SessionResponse{
		User: betterauth.UserInfo{
			ID:    betterAuthUserID,
			Email: "linked@example.com",
			Name:  "Linked Staff",
		},
		Session: betterauth.SessionInfo{
			ID:                   "session-1",
			ActiveOrganizationID: orgID,
		},
	}
	mock.memberResp = &betterauth.MemberInfo{
		ID:             "member-1",
		OrganizationID: orgID,
		UserID:         betterAuthUserID,
		Role:           "supervisor",
	}

	var capturedTC *TenantContext
	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		capturedTC = TenantFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, capturedTC)
	require.NotNil(t, capturedTC.StaffID)
	assert.Equal(t, staff.ID, *capturedTC.StaffID)
}

func TestMiddleware_UnknownRole_EmptyPermissions(t *testing.T) {
	db, orgID, cleanup := setupTenantTestDB(t)
	defer cleanup()

	mock := newMockBetterAuthServer()
	defer mock.close()

	userID := "user-" + testpkg.GenerateTestOGSID("test")
	mock.sessionResp = &betterauth.SessionResponse{
		User: betterauth.UserInfo{
			ID:    userID,
			Email: "unknown@example.com",
			Name:  "Unknown Role User",
		},
		Session: betterauth.SessionInfo{
			ID:                   "session-1",
			ActiveOrganizationID: orgID,
		},
	}
	mock.memberResp = &betterauth.MemberInfo{
		ID:             "member-1",
		OrganizationID: orgID,
		UserID:         userID,
		Role:           "unknownRole",
	}

	var capturedTC *TenantContext
	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		capturedTC = TenantFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, capturedTC)
	assert.Equal(t, "unknownRole", capturedTC.Role)
	assert.Empty(t, capturedTC.Permissions) // Unknown roles get empty permissions
}

func TestMiddleware_CookiesForwarded(t *testing.T) {
	var receivedCookies []*http.Cookie

	mock := newMockBetterAuthServer()
	defer mock.close()

	// Capture cookies in handler
	mock.sessionHandler = func(w http.ResponseWriter, r *http.Request) {
		receivedCookies = r.Cookies()
		w.WriteHeader(http.StatusUnauthorized)
	}

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "better-auth.session_token", Value: "test-session-token"})
	req.AddCookie(&http.Cookie{Name: "other_cookie", Value: "other-value"})
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	// Verify cookies were forwarded to BetterAuth
	assert.Len(t, receivedCookies, 2)
}

func TestMiddleware_Success_AdminRole(t *testing.T) {
	db, orgID, cleanup := setupTenantTestDB(t)
	defer cleanup()

	mock := newMockBetterAuthServer()
	defer mock.close()

	userID := "user-" + testpkg.GenerateTestOGSID("test")
	mock.sessionResp = &betterauth.SessionResponse{
		User: betterauth.UserInfo{
			ID:    userID,
			Email: "admin@example.com",
			Name:  "Admin User",
		},
		Session: betterauth.SessionInfo{
			ID:                   "session-1",
			ActiveOrganizationID: orgID,
		},
	}
	mock.memberResp = &betterauth.MemberInfo{
		ID:             "member-1",
		OrganizationID: orgID,
		UserID:         userID,
		Role:           "admin", // BetterAuth default owner role
	}

	var capturedTC *TenantContext
	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		capturedTC = TenantFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, capturedTC)
	assert.Equal(t, "admin", capturedTC.Role)
	assert.Contains(t, capturedTC.Permissions, "student:create")
	assert.Contains(t, capturedTC.Permissions, "location:read")
}

func TestMiddleware_Success_TraegerAdminRole(t *testing.T) {
	db, orgID, cleanup := setupTenantTestDB(t)
	defer cleanup()

	mock := newMockBetterAuthServer()
	defer mock.close()

	userID := "user-" + testpkg.GenerateTestOGSID("test")
	mock.sessionResp = &betterauth.SessionResponse{
		User: betterauth.UserInfo{
			ID:    userID,
			Email: "traeger@example.com",
			Name:  "Träger Admin",
		},
		Session: betterauth.SessionInfo{
			ID:                   "session-1",
			ActiveOrganizationID: orgID,
		},
	}
	mock.memberResp = &betterauth.MemberInfo{
		ID:             "member-1",
		OrganizationID: orgID,
		UserID:         userID,
		Role:           "traegerAdmin",
	}

	var capturedTC *TenantContext
	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		capturedTC = TenantFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, capturedTC)
	assert.Equal(t, "traegerAdmin", capturedTC.Role)
	assert.NotContains(t, capturedTC.Permissions, "location:read") // GDPR: traegerAdmin has no location access
}

func TestMiddleware_Success_MemberRole(t *testing.T) {
	db, orgID, cleanup := setupTenantTestDB(t)
	defer cleanup()

	mock := newMockBetterAuthServer()
	defer mock.close()

	userID := "user-" + testpkg.GenerateTestOGSID("test")
	mock.sessionResp = &betterauth.SessionResponse{
		User: betterauth.UserInfo{
			ID:    userID,
			Email: "member@example.com",
			Name:  "Member User",
		},
		Session: betterauth.SessionInfo{
			ID:                   "session-1",
			ActiveOrganizationID: orgID,
		},
	}
	mock.memberResp = &betterauth.MemberInfo{
		ID:             "member-1",
		OrganizationID: orgID,
		UserID:         userID,
		Role:           "member",
	}

	var capturedTC *TenantContext
	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		capturedTC = TenantFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, capturedTC)
	assert.Equal(t, "member", capturedTC.Role)
	assert.Contains(t, capturedTC.Permissions, "student:read")
	assert.NotContains(t, capturedTC.Permissions, "student:create")
}

func TestMiddleware_Success_OwnerRole(t *testing.T) {
	db, orgID, cleanup := setupTenantTestDB(t)
	defer cleanup()

	mock := newMockBetterAuthServer()
	defer mock.close()

	userID := "user-" + testpkg.GenerateTestOGSID("test")
	mock.sessionResp = &betterauth.SessionResponse{
		User: betterauth.UserInfo{
			ID:    userID,
			Email: "owner@example.com",
			Name:  "Owner User",
		},
		Session: betterauth.SessionInfo{
			ID:                   "session-1",
			ActiveOrganizationID: orgID,
		},
	}
	mock.memberResp = &betterauth.MemberInfo{
		ID:             "member-1",
		OrganizationID: orgID,
		UserID:         userID,
		Role:           "owner",
	}

	var capturedTC *TenantContext
	r := chi.NewRouter()
	r.Use(Middleware(mock.client(), db))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		capturedTC = TenantFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, capturedTC)
	assert.Equal(t, "owner", capturedTC.Role)
	assert.Contains(t, capturedTC.Permissions, "student:create")
}

// =============================================================================
// Context Helper Tests
// =============================================================================

func TestPermissionsFromCtx_NoContext(t *testing.T) {
	ctx := context.Background()
	perms := PermissionsFromCtx(ctx)
	assert.Empty(t, perms)
}

func TestPermissionsFromCtx_WithContext(t *testing.T) {
	tc := &TenantContext{
		Permissions: []string{"student:read", "group:read"},
	}
	ctx := SetTenantContext(context.Background(), tc)

	perms := PermissionsFromCtx(ctx)
	assert.Equal(t, []string{"student:read", "group:read"}, perms)
}

func TestTenantFromCtx_NoContext(t *testing.T) {
	ctx := context.Background()
	tc := TenantFromCtx(ctx)
	assert.Nil(t, tc)
}

func TestTenantFromCtx_WithContext(t *testing.T) {
	expected := &TenantContext{
		UserID:    "user-1",
		UserEmail: "test@example.com",
		OrgID:     "org-1",
		Role:      "supervisor",
	}
	ctx := SetTenantContext(context.Background(), expected)

	tc := TenantFromCtx(ctx)
	assert.Equal(t, expected, tc)
}

func TestUserIDFromCtx_NoContext(t *testing.T) {
	ctx := context.Background()
	userID := UserIDFromCtx(ctx)
	assert.Empty(t, userID)
}

func TestUserIDFromCtx_WithContext(t *testing.T) {
	tc := &TenantContext{
		UserID: "test-user-123",
	}
	ctx := SetTenantContext(context.Background(), tc)

	userID := UserIDFromCtx(ctx)
	assert.Equal(t, "test-user-123", userID)
}

func TestOrgIDFromCtx_NoContext(t *testing.T) {
	ctx := context.Background()
	orgID := OrgIDFromCtx(ctx)
	assert.Empty(t, orgID)
}

func TestOrgIDFromCtx_WithContext(t *testing.T) {
	tc := &TenantContext{
		OrgID: "test-org-456",
	}
	ctx := SetTenantContext(context.Background(), tc)

	orgID := OrgIDFromCtx(ctx)
	assert.Equal(t, "test-org-456", orgID)
}

func TestRoleFromCtx_NoContext(t *testing.T) {
	ctx := context.Background()
	role := RoleFromCtx(ctx)
	assert.Empty(t, role)
}

func TestRoleFromCtx_WithContext(t *testing.T) {
	tc := &TenantContext{
		Role: "supervisor",
	}
	ctx := SetTenantContext(context.Background(), tc)

	role := RoleFromCtx(ctx)
	assert.Equal(t, "supervisor", role)
}

func TestTraegerIDFromCtx_NoContext(t *testing.T) {
	ctx := context.Background()
	traegerID := TraegerIDFromCtx(ctx)
	assert.Empty(t, traegerID)
}

func TestTraegerIDFromCtx_WithContext(t *testing.T) {
	tc := &TenantContext{
		TraegerID: "traeger-789",
	}
	ctx := SetTenantContext(context.Background(), tc)

	traegerID := TraegerIDFromCtx(ctx)
	assert.Equal(t, "traeger-789", traegerID)
}

func TestBueroIDFromCtx_NoContext(t *testing.T) {
	ctx := context.Background()
	bueroID := BueroIDFromCtx(ctx)
	assert.Nil(t, bueroID)
}

func TestBueroIDFromCtx_NoBuero(t *testing.T) {
	tc := &TenantContext{
		BueroID: nil,
	}
	ctx := SetTenantContext(context.Background(), tc)

	bueroID := BueroIDFromCtx(ctx)
	assert.Nil(t, bueroID)
}

func TestBueroIDFromCtx_WithBuero(t *testing.T) {
	expectedBueroID := "buero-123"
	tc := &TenantContext{
		BueroID: &expectedBueroID,
	}
	ctx := SetTenantContext(context.Background(), tc)

	bueroID := BueroIDFromCtx(ctx)
	require.NotNil(t, bueroID)
	assert.Equal(t, "buero-123", *bueroID)
}

func TestAccountIDFromCtx_NoContext(t *testing.T) {
	ctx := context.Background()
	accountID := AccountIDFromCtx(ctx)
	assert.Nil(t, accountID)
}

func TestAccountIDFromCtx_NoAccount(t *testing.T) {
	tc := &TenantContext{
		AccountID: nil,
	}
	ctx := SetTenantContext(context.Background(), tc)

	accountID := AccountIDFromCtx(ctx)
	assert.Nil(t, accountID)
}

func TestAccountIDFromCtx_WithAccount(t *testing.T) {
	expectedAccountID := int64(42)
	tc := &TenantContext{
		AccountID: &expectedAccountID,
	}
	ctx := SetTenantContext(context.Background(), tc)

	accountID := AccountIDFromCtx(ctx)
	require.NotNil(t, accountID)
	assert.Equal(t, int64(42), *accountID)
}

// =============================================================================
// GetPermissionsForRole Tests
// =============================================================================

func TestGetPermissionsForRole_Supervisor(t *testing.T) {
	perms := GetPermissionsForRole("supervisor")

	assert.Contains(t, perms, "student:read")
	assert.Contains(t, perms, "location:read")
	assert.NotContains(t, perms, "student:delete")
}

func TestGetPermissionsForRole_OgsAdmin(t *testing.T) {
	perms := GetPermissionsForRole("ogsAdmin")

	assert.Contains(t, perms, "student:create")
	assert.Contains(t, perms, "student:delete")
	assert.Contains(t, perms, "location:read")
	assert.Contains(t, perms, "staff:invite")
}

func TestGetPermissionsForRole_BueroAdmin(t *testing.T) {
	perms := GetPermissionsForRole("bueroAdmin")

	assert.Contains(t, perms, "student:read")
	assert.NotContains(t, perms, "location:read") // GDPR: no location access
}

func TestGetPermissionsForRole_TraegerAdmin(t *testing.T) {
	perms := GetPermissionsForRole("traegerAdmin")

	assert.Contains(t, perms, "student:read")
	assert.NotContains(t, perms, "location:read") // GDPR: no location access
}

func TestGetPermissionsForRole_Unknown(t *testing.T) {
	perms := GetPermissionsForRole("unknownRole")

	assert.Empty(t, perms)
}

func TestGetPermissionsForRole_ReturnsCopy(t *testing.T) {
	perms1 := GetPermissionsForRole("supervisor")
	perms2 := GetPermissionsForRole("supervisor")

	// Modify first copy
	perms1[0] = "modified"

	// Second copy should be unchanged
	assert.NotEqual(t, "modified", perms2[0])
}

// =============================================================================
// Permission Check Helper Tests
// =============================================================================

func TestHasLocationPermission_NoContext(t *testing.T) {
	ctx := context.Background()
	assert.False(t, HasLocationPermission(ctx))
}

func TestHasLocationPermission_WithoutPermission(t *testing.T) {
	tc := &TenantContext{
		Permissions: []string{"student:read"},
	}
	ctx := SetTenantContext(context.Background(), tc)
	assert.False(t, HasLocationPermission(ctx))
}

func TestHasLocationPermission_WithPermission(t *testing.T) {
	tc := &TenantContext{
		Permissions: []string{"student:read", "location:read"},
	}
	ctx := SetTenantContext(context.Background(), tc)
	assert.True(t, HasLocationPermission(ctx))
}

func TestIsAdmin_NoContext(t *testing.T) {
	ctx := context.Background()
	assert.False(t, IsAdmin(ctx))
}

func TestIsAdmin_Supervisor(t *testing.T) {
	tc := &TenantContext{Role: "supervisor"}
	ctx := SetTenantContext(context.Background(), tc)
	assert.False(t, IsAdmin(ctx))
}

func TestIsAdmin_OgsAdmin(t *testing.T) {
	tc := &TenantContext{Role: "ogsAdmin"}
	ctx := SetTenantContext(context.Background(), tc)
	assert.True(t, IsAdmin(ctx))
}

func TestIsAdmin_BueroAdmin(t *testing.T) {
	tc := &TenantContext{Role: "bueroAdmin"}
	ctx := SetTenantContext(context.Background(), tc)
	assert.True(t, IsAdmin(ctx))
}

func TestIsAdmin_TraegerAdmin(t *testing.T) {
	tc := &TenantContext{Role: "traegerAdmin"}
	ctx := SetTenantContext(context.Background(), tc)
	assert.True(t, IsAdmin(ctx))
}

func TestIsSupervisor_NoContext(t *testing.T) {
	ctx := context.Background()
	assert.False(t, IsSupervisor(ctx))
}

func TestIsSupervisor_Supervisor(t *testing.T) {
	tc := &TenantContext{Role: "supervisor"}
	ctx := SetTenantContext(context.Background(), tc)
	assert.True(t, IsSupervisor(ctx))
}

func TestIsSupervisor_Admin(t *testing.T) {
	tc := &TenantContext{Role: "ogsAdmin"}
	ctx := SetTenantContext(context.Background(), tc)
	assert.False(t, IsSupervisor(ctx))
}

func TestCanManageStaff_NoContext(t *testing.T) {
	ctx := context.Background()
	assert.False(t, CanManageStaff(ctx))
}

func TestCanManageStaff_Supervisor(t *testing.T) {
	tc := &TenantContext{
		Permissions: GetPermissionsForRole("supervisor"),
	}
	ctx := SetTenantContext(context.Background(), tc)
	assert.False(t, CanManageStaff(ctx))
}

func TestCanManageStaff_OgsAdmin(t *testing.T) {
	tc := &TenantContext{
		Permissions: GetPermissionsForRole("ogsAdmin"),
	}
	ctx := SetTenantContext(context.Background(), tc)
	assert.True(t, CanManageStaff(ctx))
}

func TestCanManageOGS_NoContext(t *testing.T) {
	ctx := context.Background()
	assert.False(t, CanManageOGS(ctx))
}

func TestCanManageOGS_Supervisor(t *testing.T) {
	tc := &TenantContext{
		Permissions: GetPermissionsForRole("supervisor"),
	}
	ctx := SetTenantContext(context.Background(), tc)
	assert.False(t, CanManageOGS(ctx))
}

func TestCanManageOGS_OgsAdmin(t *testing.T) {
	tc := &TenantContext{
		Permissions: GetPermissionsForRole("ogsAdmin"),
	}
	ctx := SetTenantContext(context.Background(), tc)
	assert.True(t, CanManageOGS(ctx))
}
