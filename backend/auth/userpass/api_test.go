package userpass

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// We'll use the TokenAuthInterface from api.go

// TestPasswordValidation tests our password validation functions
func TestPasswordValidation(t *testing.T) {
	// Test that validation detects too short passwords
	err := validatePassword("short")
	assert.Error(t, err)
	assert.Equal(t, ErrPasswordTooShort, err)

	// Test that validation rejects missing uppercase
	err = validatePassword("nogoodupper12345!")
	assert.Error(t, err)
	assert.Equal(t, ErrPasswordNoUpper, err)

	// Test that a good password passes validation
	err = validatePassword("GoodP@ssw0rd")
	assert.NoError(t, err)
}

// TestPasswordHashing tests our password hashing and verification
func TestPasswordHashing(t *testing.T) {
	// Test that we can hash and verify passwords
	password := "TestP@ssw0rd"
	hash, err := HashPassword(password, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Test that verification works with valid password
	valid, err := VerifyPassword(password, hash)
	assert.NoError(t, err)
	assert.True(t, valid)

	// Test that verification fails with wrong password
	valid, err = VerifyPassword("WrongPassword", hash)
	assert.NoError(t, err)
	assert.False(t, valid)
}

// MockTokenAuth implements TokenAuthInterface for testing
type MockTokenAuth struct {
	mock.Mock
	RefreshExpiry time.Duration
}

func (m *MockTokenAuth) GetRefreshExpiry() time.Duration {
	return m.RefreshExpiry
}

func (m *MockTokenAuth) Verifier() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

func (m *MockTokenAuth) GenTokenPair(appClaims jwt.AppClaims, refreshClaims jwt.RefreshClaims) (string, string, error) {
	args := m.Called(appClaims, refreshClaims)
	return args.String(0), args.String(1), args.Error(2)
}

func TestLoginAPI(t *testing.T) {
	// Create test password hash
	password := "TestP@ssw0rd"
	hash, err := HashPassword(password, nil)
	assert.NoError(t, err)

	// Create mock auth store
	mockStore := new(MockAuthStore)

	// Create mock token auth
	mockTokenAuth := new(MockTokenAuth)
	mockTokenAuth.RefreshExpiry = time.Hour * 24 * 7 // 7 days

	// Setup test account
	testAccount := &Account{
		ID:           1,
		Email:        "test@example.com",
		Name:         "Test User",
		Active:       true,
		Roles:        []string{"user"},
		PasswordHash: hash,
	}

	// Define mock token pair
	accessToken := "mock-access-token"
	refreshToken := "mock-refresh-token"

	// Setup mock expectations
	mockStore.On("GetAccountByEmail", "test@example.com").Return(testAccount, nil)
	mockStore.On("CreateOrUpdateToken", mock.AnythingOfType("*jwt.Token")).Return(nil)
	mockStore.On("UpdateAccount", mock.AnythingOfType("*userpass.Account")).Return(nil)
	mockTokenAuth.On("GenTokenPair", mock.AnythingOfType("jwt.AppClaims"), mock.AnythingOfType("jwt.RefreshClaims")).
		Return(accessToken, refreshToken, nil)

	// Create resource
	resource := &Resource{
		TokenAuth: mockTokenAuth,
		Store:     mockStore,
	}

	// Setup router
	r := chi.NewRouter()
	r.Post("/login", resource.login)

	// Create test request
	loginReq := loginRequest{
		Email:    "test@example.com",
		Password: password,
	}
	body, err := json.Marshal(loginReq)
	assert.NoError(t, err)

	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute request
	r.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify response contains token fields
	var response tokenResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, accessToken, response.Access)
	assert.Equal(t, refreshToken, response.Refresh)

	// Verify mocks were called
	mockStore.AssertExpectations(t)
	mockTokenAuth.AssertExpectations(t)
}