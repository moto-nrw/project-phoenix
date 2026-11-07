package sessiontoken

// These tests reach unexported helpers, so they stay in the package; the
// token-minting tests live in sessiontoken_test because tokentest imports
// this package (#5990).

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBearerToken_ValidToken(t *testing.T) {
	t.Parallel()

	// Using a clearly fake test token (not a real JWT)
	result := extractBearerToken("Bearer test-token-value-for-unit-test")
	assert.Equal(t, "test-token-value-for-unit-test", result)
}

func TestExtractBearerToken_EmptyHeader(t *testing.T) {
	t.Parallel()

	result := extractBearerToken("")
	assert.Empty(t, result)
}

func TestExtractBearerToken_NoBearer(t *testing.T) {
	t.Parallel()

	// Token without "Bearer " prefix should return empty
	result := extractBearerToken("some-token-without-bearer-prefix")
	assert.Empty(t, result)
}

func TestExtractBearerToken_LowercaseBearer(t *testing.T) {
	t.Parallel()

	result := extractBearerToken("bearer token")
	assert.Empty(t, result, "Should be case-sensitive")
}

func TestExtractBearerToken_BearerNoSpace(t *testing.T) {
	t.Parallel()

	result := extractBearerToken("Bearertoken")
	assert.Empty(t, result)
}

func TestExtractBearerToken_BearerOnly(t *testing.T) {
	t.Parallel()

	result := extractBearerToken("Bearer ")
	assert.Empty(t, result)
}

func TestExtractBearerToken_TooShort(t *testing.T) {
	t.Parallel()

	result := extractBearerToken("Bearer")
	assert.Empty(t, result)
}

func TestExtractBearerToken_ExtraSpaces(t *testing.T) {
	t.Parallel()

	result := extractBearerToken("Bearer  token")
	assert.Equal(t, " token", result, "Should preserve extra spaces in token")
}

func TestRenderUnauthorized_SuccessfulRender(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	renderUnauthorized(rr, req, ErrTokenUnauthorized)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRejectGeneratedSecret(t *testing.T) {
	t.Parallel()

	err := rejectGeneratedSecret("random")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AUTH_JWT_SECRET=random")
	assert.NoError(t, rejectGeneratedSecret("explicit"))
}
