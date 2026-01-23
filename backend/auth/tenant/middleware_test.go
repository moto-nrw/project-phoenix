package tenant

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/betterauth"
	"github.com/stretchr/testify/assert"
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
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHandleAuthError_NoActiveOrg(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	handleAuthError(w, r, betterauth.ErrNoActiveOrg, "test context")

	resp := w.Result()
	defer resp.Body.Close()
	// ErrNoOrganization returns 401 Unauthorized (user needs to select an org)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHandleAuthError_MemberNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	handleAuthError(w, r, betterauth.ErrMemberNotFound, "test context")

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestHandleAuthError_UnknownError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	handleAuthError(w, r, assert.AnError, "test context")

	resp := w.Result()
	defer resp.Body.Close()
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
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestErrNoOrganization(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	err := render.Render(w, r, ErrNoOrganization)
	assert.NoError(t, err)

	resp := w.Result()
	defer resp.Body.Close()
	// ErrNoOrganization returns 401 Unauthorized (user needs to select an org)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestErrInternalServer(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	err := render.Render(w, r, ErrInternalServer)
	assert.NoError(t, err)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestNewErrForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	errResp := NewErrForbidden("custom error message")
	err := render.Render(w, r, errResp)
	assert.NoError(t, err)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
