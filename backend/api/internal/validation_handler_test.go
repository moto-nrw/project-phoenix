package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAccountRepo implements auth.AccountRepository for testing validateEmails.
// Only FindByEmail is implemented; other methods panic if called.
type mockAccountRepo struct {
	existingEmails map[string]bool
}

func newMockAccountRepo(existingEmails ...string) *mockAccountRepo {
	m := &mockAccountRepo{existingEmails: make(map[string]bool)}
	for _, email := range existingEmails {
		m.existingEmails[strings.ToLower(email)] = true
	}
	return m
}

func (m *mockAccountRepo) FindByEmail(_ context.Context, email string) (*auth.Account, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if m.existingEmails[normalizedEmail] {
		return &auth.Account{Email: normalizedEmail}, nil
	}
	return nil, errors.New("account not found")
}

// Stub implementations for interface compliance (not used in validateEmails tests)
func (m *mockAccountRepo) Create(_ context.Context, _ *auth.Account) error {
	panic("not implemented")
}
func (m *mockAccountRepo) FindByID(_ context.Context, _ interface{}) (*auth.Account, error) {
	panic("not implemented")
}
func (m *mockAccountRepo) FindByUsername(_ context.Context, _ string) (*auth.Account, error) {
	panic("not implemented")
}
func (m *mockAccountRepo) Update(_ context.Context, _ *auth.Account) error {
	panic("not implemented")
}
func (m *mockAccountRepo) Delete(_ context.Context, _ interface{}) error {
	panic("not implemented")
}
func (m *mockAccountRepo) List(_ context.Context, _ map[string]interface{}) ([]*auth.Account, error) {
	panic("not implemented")
}
func (m *mockAccountRepo) UpdateLastLogin(_ context.Context, _ int64) error {
	panic("not implemented")
}
func (m *mockAccountRepo) UpdatePassword(_ context.Context, _ int64, _ string) error {
	panic("not implemented")
}
func (m *mockAccountRepo) FindByRole(_ context.Context, _ string) ([]*auth.Account, error) {
	panic("not implemented")
}
func (m *mockAccountRepo) FindAccountsWithRolesAndPermissions(_ context.Context, _ map[string]interface{}) ([]*auth.Account, error) {
	panic("not implemented")
}

// createTestResource creates a Resource with the given accountRepo for testing.
func createTestResource(accountRepo auth.AccountRepository) *Resource {
	return &Resource{
		accountRepo: accountRepo,
		// Other fields (mailer, dispatcher, fromEmail, userSyncService) are not needed for validateEmails
	}
}

// executeValidateEmails sends a POST request to /validate-emails and returns the response.
func executeValidateEmails(resource *Resource, body interface{}) *httptest.ResponseRecorder {
	router := chi.NewRouter()
	router.Post("/validate-emails", resource.validateEmails)

	var bodyBytes []byte
	switch v := body.(type) {
	case []byte:
		bodyBytes = v
	case string:
		bodyBytes = []byte(v)
	default:
		bodyBytes, _ = json.Marshal(body)
	}

	req := httptest.NewRequest(http.MethodPost, "/validate-emails", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// parseErrorResponse extracts the error message from a JSON error response.
func parseErrorResponse(t *testing.T, body []byte) string {
	t.Helper()
	var resp map[string]string
	err := json.Unmarshal(body, &resp)
	require.NoError(t, err, "Failed to parse error response: %s", string(body))
	return resp["error"]
}

// parseValidateEmailsResponse parses a successful ValidateEmailsResponse.
func parseValidateEmailsResponse(t *testing.T, body []byte) ValidateEmailsResponse {
	t.Helper()
	var resp ValidateEmailsResponse
	err := json.Unmarshal(body, &resp)
	require.NoError(t, err, "Failed to parse response: %s", string(body))
	return resp
}

// =============================================================================
// Error Cases
// =============================================================================

func TestValidateEmails_AccountRepoNil(t *testing.T) {
	// Given: Resource with nil accountRepo
	resource := createTestResource(nil)

	// When: POST to /validate-emails
	rr := executeValidateEmails(resource, ValidateEmailsRequest{Emails: []string{"test@example.com"}})

	// Then: Should return 503 Service Unavailable
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	errMsg := parseErrorResponse(t, rr.Body.Bytes())
	assert.Equal(t, "account repository unavailable", errMsg)
}

func TestValidateEmails_InvalidJSON(t *testing.T) {
	// Given: Resource with valid accountRepo
	resource := createTestResource(newMockAccountRepo())

	// When: POST invalid JSON
	rr := executeValidateEmails(resource, "this is not valid json{{{")

	// Then: Should return 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	errMsg := parseErrorResponse(t, rr.Body.Bytes())
	assert.Equal(t, "invalid JSON body", errMsg)
}

func TestValidateEmails_EmptyEmailsArray(t *testing.T) {
	// Given: Resource with valid accountRepo
	resource := createTestResource(newMockAccountRepo())

	// When: POST with empty emails array
	rr := executeValidateEmails(resource, ValidateEmailsRequest{Emails: []string{}})

	// Then: Should return 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	errMsg := parseErrorResponse(t, rr.Body.Bytes())
	assert.Equal(t, "emails array is required and must not be empty", errMsg)
}

func TestValidateEmails_NilEmailsArray(t *testing.T) {
	// Given: Resource with valid accountRepo
	resource := createTestResource(newMockAccountRepo())

	// When: POST with nil emails array (JSON: {})
	rr := executeValidateEmails(resource, map[string]interface{}{})

	// Then: Should return 400 Bad Request (nil array has length 0)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	errMsg := parseErrorResponse(t, rr.Body.Bytes())
	assert.Equal(t, "emails array is required and must not be empty", errMsg)
}

func TestValidateEmails_TooManyEmails(t *testing.T) {
	// Given: Resource with valid accountRepo
	resource := createTestResource(newMockAccountRepo())

	// When: POST with 51 emails (exceeds maximum of 50)
	emails := make([]string, 51)
	for i := range emails {
		emails[i] = "test@example.com"
	}
	rr := executeValidateEmails(resource, ValidateEmailsRequest{Emails: emails})

	// Then: Should return 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	errMsg := parseErrorResponse(t, rr.Body.Bytes())
	assert.Equal(t, "too many emails, maximum is 50", errMsg)
}

func TestValidateEmails_ExactlyMaxEmails(t *testing.T) {
	// Given: Resource with valid accountRepo
	resource := createTestResource(newMockAccountRepo())

	// When: POST with exactly 50 emails (at the limit)
	emails := make([]string, 50)
	for i := range emails {
		emails[i] = "test@example.com"
	}
	rr := executeValidateEmails(resource, ValidateEmailsRequest{Emails: emails})

	// Then: Should return 200 OK (50 is allowed)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// =============================================================================
// Success Cases
// =============================================================================

func TestValidateEmails_AllAvailable(t *testing.T) {
	// Given: Resource with no existing emails
	resource := createTestResource(newMockAccountRepo())

	// When: POST with emails that don't exist
	req := ValidateEmailsRequest{
		Emails: []string{"new1@example.com", "new2@example.com", "new3@example.com"},
	}
	rr := executeValidateEmails(resource, req)

	// Then: All emails should be in "available" list
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseValidateEmailsResponse(t, rr.Body.Bytes())
	assert.Len(t, resp.Available, 3)
	assert.Empty(t, resp.Unavailable)
	assert.Contains(t, resp.Available, "new1@example.com")
	assert.Contains(t, resp.Available, "new2@example.com")
	assert.Contains(t, resp.Available, "new3@example.com")
}

func TestValidateEmails_AllUnavailable(t *testing.T) {
	// Given: Resource with all requested emails already existing
	resource := createTestResource(newMockAccountRepo(
		"existing1@example.com",
		"existing2@example.com",
	))

	// When: POST with emails that all exist
	req := ValidateEmailsRequest{
		Emails: []string{"existing1@example.com", "existing2@example.com"},
	}
	rr := executeValidateEmails(resource, req)

	// Then: All emails should be in "unavailable" list
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseValidateEmailsResponse(t, rr.Body.Bytes())
	assert.Empty(t, resp.Available)
	assert.Len(t, resp.Unavailable, 2)
	assert.Contains(t, resp.Unavailable, "existing1@example.com")
	assert.Contains(t, resp.Unavailable, "existing2@example.com")
}

func TestValidateEmails_MixedAvailability(t *testing.T) {
	// Given: Resource with some existing emails
	resource := createTestResource(newMockAccountRepo(
		"existing@example.com",
		"taken@example.com",
	))

	// When: POST with mix of existing and new emails
	req := ValidateEmailsRequest{
		Emails: []string{
			"existing@example.com", // unavailable
			"new@example.com",      // available
			"taken@example.com",    // unavailable
			"fresh@example.com",    // available
		},
	}
	rr := executeValidateEmails(resource, req)

	// Then: Emails should be correctly categorized
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseValidateEmailsResponse(t, rr.Body.Bytes())
	assert.Len(t, resp.Available, 2)
	assert.Len(t, resp.Unavailable, 2)
	assert.Contains(t, resp.Available, "new@example.com")
	assert.Contains(t, resp.Available, "fresh@example.com")
	assert.Contains(t, resp.Unavailable, "existing@example.com")
	assert.Contains(t, resp.Unavailable, "taken@example.com")
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestValidateEmails_EmailNormalization(t *testing.T) {
	// Given: Resource with existing email in lowercase
	resource := createTestResource(newMockAccountRepo("existing@example.com"))

	// When: POST with various case/whitespace variations
	req := ValidateEmailsRequest{
		Emails: []string{
			"  EXISTING@EXAMPLE.COM  ", // should match existing (unavailable)
			"  NEW@EXAMPLE.COM  ",      // should not match (available)
			"ExIsTiNg@ExAmPlE.cOm",     // should match existing (unavailable)
		},
	}
	rr := executeValidateEmails(resource, req)

	// Then: Emails should be normalized and correctly categorized
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseValidateEmailsResponse(t, rr.Body.Bytes())

	// Check that normalization happened (all should be lowercase and trimmed)
	assert.Len(t, resp.Unavailable, 2, "Both variations of existing@example.com should be unavailable")
	assert.Len(t, resp.Available, 1)
	assert.Contains(t, resp.Available, "new@example.com")
}

func TestValidateEmails_EmptyStringsSkipped(t *testing.T) {
	// Given: Resource with no existing emails
	resource := createTestResource(newMockAccountRepo())

	// When: POST with empty strings and whitespace-only strings
	req := ValidateEmailsRequest{
		Emails: []string{
			"valid@example.com",
			"",
			"   ",
			"another@example.com",
			"",
		},
	}
	rr := executeValidateEmails(resource, req)

	// Then: Empty strings should be skipped, only valid emails processed
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseValidateEmailsResponse(t, rr.Body.Bytes())
	assert.Len(t, resp.Available, 2, "Only non-empty emails should be processed")
	assert.Contains(t, resp.Available, "valid@example.com")
	assert.Contains(t, resp.Available, "another@example.com")
}

func TestValidateEmails_SingleEmail(t *testing.T) {
	// Given: Resource with no existing emails
	resource := createTestResource(newMockAccountRepo())

	// When: POST with single email
	req := ValidateEmailsRequest{
		Emails: []string{"single@example.com"},
	}
	rr := executeValidateEmails(resource, req)

	// Then: Single email should work correctly
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseValidateEmailsResponse(t, rr.Body.Bytes())
	assert.Len(t, resp.Available, 1)
	assert.Empty(t, resp.Unavailable)
	assert.Equal(t, "single@example.com", resp.Available[0])
}

func TestValidateEmails_DuplicateEmails(t *testing.T) {
	// Given: Resource with no existing emails
	resource := createTestResource(newMockAccountRepo())

	// When: POST with duplicate emails in request
	req := ValidateEmailsRequest{
		Emails: []string{
			"duplicate@example.com",
			"duplicate@example.com",
			"DUPLICATE@EXAMPLE.COM", // Same email, different case
		},
	}
	rr := executeValidateEmails(resource, req)

	// Then: All duplicates should appear in result (handler doesn't deduplicate)
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseValidateEmailsResponse(t, rr.Body.Bytes())
	// All three should be available and all normalized
	assert.Len(t, resp.Available, 3)
	for _, email := range resp.Available {
		assert.Equal(t, "duplicate@example.com", email)
	}
}

func TestValidateEmails_ContentTypeHeader(t *testing.T) {
	// Given: Resource with valid accountRepo
	resource := createTestResource(newMockAccountRepo())

	// When: POST request is made
	rr := executeValidateEmails(resource, ValidateEmailsRequest{Emails: []string{"test@example.com"}})

	// Then: Response should have JSON content type
	assert.Equal(t, http.StatusOK, rr.Code)
	contentType := rr.Header().Get("Content-Type")
	assert.Equal(t, "application/json", contentType)
}

// =============================================================================
// Response Structure Tests
// =============================================================================

func TestValidateEmails_ResponseStructure(t *testing.T) {
	// Given: Resource with mix of available/unavailable emails
	resource := createTestResource(newMockAccountRepo("taken@example.com"))

	// When: POST with emails
	req := ValidateEmailsRequest{
		Emails: []string{"taken@example.com", "free@example.com"},
	}
	rr := executeValidateEmails(resource, req)

	// Then: Response should have correct JSON structure
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse as raw JSON to verify structure
	var rawResp map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &rawResp)
	require.NoError(t, err)

	// Verify keys exist
	_, hasAvailable := rawResp["available"]
	_, hasUnavailable := rawResp["unavailable"]
	assert.True(t, hasAvailable, "Response should have 'available' key")
	assert.True(t, hasUnavailable, "Response should have 'unavailable' key")

	// Verify arrays (not nil even if empty)
	available, ok := rawResp["available"].([]interface{})
	require.True(t, ok, "'available' should be an array")
	unavailable, ok := rawResp["unavailable"].([]interface{})
	require.True(t, ok, "'unavailable' should be an array")

	assert.Len(t, available, 1)
	assert.Len(t, unavailable, 1)
}

func TestValidateEmails_EmptyArraysInResponse(t *testing.T) {
	// Given: Resource with no existing emails
	resource := createTestResource(newMockAccountRepo())

	// When: POST with only empty strings (all get skipped)
	req := ValidateEmailsRequest{
		Emails: []string{"", "   "},
	}
	// Note: This will fail validation because after trimming, all emails are empty,
	// but the request has len > 0, so it passes the empty check.
	// The result will have empty arrays for both available and unavailable.
	rr := executeValidateEmails(resource, req)

	// Then: Both arrays should exist (possibly empty) in response
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseValidateEmailsResponse(t, rr.Body.Bytes())
	assert.NotNil(t, resp.Available)
	assert.NotNil(t, resp.Unavailable)
	assert.Empty(t, resp.Available)
	assert.Empty(t, resp.Unavailable)
}
