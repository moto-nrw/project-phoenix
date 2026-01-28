package usercontext

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// ErrorRenderer Tests
// =============================================================================

// Helper to create a UserContextError wrapping a sentinel error
func wrapError(err error) error {
	return &usercontext.UserContextError{Op: "test", Err: err}
}

func TestErrorRenderer_UserNotAuthenticated(t *testing.T) {
	err := wrapError(usercontext.ErrUserNotAuthenticated)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
}

func TestErrorRenderer_UserNotAuthorized(t *testing.T) {
	err := wrapError(usercontext.ErrUserNotAuthorized)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusForbidden, errResp.HTTPStatusCode)
}

func TestErrorRenderer_UserNotFound(t *testing.T) {
	err := wrapError(usercontext.ErrUserNotFound)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestErrorRenderer_UserNotLinkedToPerson(t *testing.T) {
	err := wrapError(usercontext.ErrUserNotLinkedToPerson)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestErrorRenderer_UserNotLinkedToStaff(t *testing.T) {
	err := wrapError(usercontext.ErrUserNotLinkedToStaff)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestErrorRenderer_UserNotLinkedToTeacher(t *testing.T) {
	err := wrapError(usercontext.ErrUserNotLinkedToTeacher)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestErrorRenderer_GroupNotFound(t *testing.T) {
	err := wrapError(usercontext.ErrGroupNotFound)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestErrorRenderer_NoActiveGroups(t *testing.T) {
	err := wrapError(usercontext.ErrNoActiveGroups)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestErrorRenderer_InvalidOperation(t *testing.T) {
	err := wrapError(usercontext.ErrInvalidOperation)
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
}

func TestErrorRenderer_GenericUserContextError(t *testing.T) {
	// Create a generic UserContextError that doesn't match specific types
	err := &usercontext.UserContextError{
		Op:  "test",
		Err: errors.New("generic error"),
	}
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
}

func TestErrorRenderer_NonUserContextError(t *testing.T) {
	// Test with a regular error (not a UserContextError)
	err := errors.New("some random error")
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
}

// =============================================================================
// getFileExtension Tests
// =============================================================================

func TestGetFileExtension_WithExtension(t *testing.T) {
	tests := []struct {
		filename    string
		contentType string
		expected    string
	}{
		{"image.jpg", "image/jpeg", ".jpg"},
		{"photo.png", "image/png", ".png"},
		{"pic.webp", "image/webp", ".webp"},
		{"file.JPEG", "image/jpeg", ".JPEG"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := getFileExtension(tt.filename, tt.contentType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetFileExtension_WithoutExtension(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		contentType string
		expected    string
	}{
		{"jpeg", "image", "image/jpeg", ".jpg"},
		{"jpg", "image", "image/jpg", ".jpg"},
		{"png", "image", "image/png", ".png"},
		{"webp", "image", "image/webp", ".webp"},
		{"unknown", "image", "application/octet-stream", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFileExtension(tt.filename, tt.contentType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// generateRandomString Tests
// =============================================================================

func TestGenerateRandomString_ReturnsCorrectLength(t *testing.T) {
	lengths := []int{4, 8, 16, 32}

	for _, length := range lengths {
		result, err := generateRandomString(length)
		assert.NoError(t, err)
		assert.Equal(t, length, len(result))
	}
}

func TestGenerateRandomString_ReturnsUnique(t *testing.T) {
	results := make(map[string]bool)
	for i := 0; i < 100; i++ {
		result, err := generateRandomString(16)
		assert.NoError(t, err)
		assert.False(t, results[result], "Generated duplicate string")
		results[result] = true
	}
}

func TestGenerateRandomString_ZeroLength(t *testing.T) {
	result, err := generateRandomString(0)
	assert.NoError(t, err)
	assert.Equal(t, "", result)
}

// =============================================================================
// validateAvatarPath Tests
// =============================================================================

func TestValidateAvatarPath_ValidPath(t *testing.T) {
	validPaths := []string{
		"12345_abc123.jpg",
		"1_x.png",
		"99999999_abcdefgh.webp",
	}

	for _, path := range validPaths {
		t.Run(path, func(t *testing.T) {
			filePath, errRenderer := validateAvatarPath(path)
			assert.Nil(t, errRenderer, "Expected no error for valid path")
			assert.NotEmpty(t, filePath, "Expected non-empty file path")
		})
	}
}

func TestValidateAvatarPath_InvalidPath(t *testing.T) {
	// Test path traversal attacks - these should be rejected
	invalidPaths := []struct {
		name string
		path string
	}{
		{"path traversal up", "../etc/passwd"},
		{"double dot", ".."},
		{"path traversal up2", "../../secret.txt"},
	}

	for _, tt := range invalidPaths {
		t.Run(tt.name, func(t *testing.T) {
			_, errRenderer := validateAvatarPath(tt.path)
			assert.NotNil(t, errRenderer, "Expected error for invalid path: %s", tt.path)
		})
	}
}

// =============================================================================
// AllowedImageTypes Tests
// =============================================================================

func TestAllowedImageTypes(t *testing.T) {
	allowed := []string{"image/jpeg", "image/jpg", "image/png", "image/webp"}
	notAllowed := []string{"image/gif", "image/bmp", "application/pdf", "text/html"}

	for _, ct := range allowed {
		assert.True(t, allowedImageTypes[ct], "%s should be allowed", ct)
	}

	for _, ct := range notAllowed {
		assert.False(t, allowedImageTypes[ct], "%s should not be allowed", ct)
	}
}
