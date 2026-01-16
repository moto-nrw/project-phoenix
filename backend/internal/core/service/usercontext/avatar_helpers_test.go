package usercontext

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
