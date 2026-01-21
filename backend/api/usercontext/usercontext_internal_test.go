// Package usercontext internal tests for pure helper functions.
// These tests verify logic that doesn't require database access.
package usercontext

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// detectAndValidateContentType Tests
// =============================================================================

// mockReadSeeker is a test helper that implements io.ReadSeeker
type mockReadSeeker struct {
	*bytes.Reader
}

func newMockReadSeeker(data []byte) *mockReadSeeker {
	return &mockReadSeeker{bytes.NewReader(data)}
}

func TestDetectAndValidateContentType_JPEG(t *testing.T) {
	// JPEG magic bytes
	jpegBytes := make([]byte, 512)
	jpegBytes[0] = 0xFF
	jpegBytes[1] = 0xD8
	jpegBytes[2] = 0xFF

	reader := newMockReadSeeker(jpegBytes)
	contentType, err := detectAndValidateContentType(reader)

	assert.NoError(t, err)
	assert.Equal(t, "image/jpeg", contentType)
}

func TestDetectAndValidateContentType_PNG(t *testing.T) {
	// PNG magic bytes
	pngBytes := make([]byte, 512)
	copy(pngBytes, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})

	reader := newMockReadSeeker(pngBytes)
	contentType, err := detectAndValidateContentType(reader)

	assert.NoError(t, err)
	assert.Equal(t, "image/png", contentType)
}

// Note: WebP test skipped because http.DetectContentType requires full RIFF header
// which is complex to mock correctly

func TestDetectAndValidateContentType_InvalidType(t *testing.T) {
	// HTML content - not allowed
	htmlBytes := []byte("<!DOCTYPE html><html><body>Hello</body></html>")
	// Pad to 512 bytes
	padded := make([]byte, 512)
	copy(padded, htmlBytes)

	reader := newMockReadSeeker(padded)
	_, err := detectAndValidateContentType(reader)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file type")
}

func TestDetectAndValidateContentType_EmptyFile(t *testing.T) {
	reader := newMockReadSeeker([]byte{})
	_, err := detectAndValidateContentType(reader)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read file")
}

// =============================================================================
// closeFile Tests
// =============================================================================

type mockCloser struct {
	closed bool
	err    error
}

func (m *mockCloser) Close() error {
	m.closed = true
	return m.err
}

func TestCloseFile_Success(t *testing.T) {
	closer := &mockCloser{}
	closeFile(closer)
	assert.True(t, closer.closed)
}

func TestCloseFile_WithError(t *testing.T) {
	closer := &mockCloser{err: assert.AnError}
	// Should not panic even if close returns error
	closeFile(closer)
	assert.True(t, closer.closed)
}

// NOTE: closeFileHandle tests skipped as they require real *os.File

// =============================================================================
// ProfileUpdateRequest Tests
// =============================================================================

func TestProfileUpdateRequest_Fields(t *testing.T) {
	firstName := "John"
	lastName := "Doe"
	username := "johndoe"
	bio := "Test bio"

	req := ProfileUpdateRequest{
		FirstName: &firstName,
		LastName:  &lastName,
		Username:  &username,
		Bio:       &bio,
	}

	assert.Equal(t, "John", *req.FirstName)
	assert.Equal(t, "Doe", *req.LastName)
	assert.Equal(t, "johndoe", *req.Username)
	assert.Equal(t, "Test bio", *req.Bio)
}

func TestProfileUpdateRequest_NilFields(t *testing.T) {
	var req ProfileUpdateRequest
	assert.Nil(t, req.FirstName)
	assert.Nil(t, req.LastName)
	assert.Nil(t, req.Username)
	assert.Nil(t, req.Bio)
}

func TestProfileUpdateRequest_PartialFields(t *testing.T) {
	firstName := "Jane"
	req := ProfileUpdateRequest{
		FirstName: &firstName,
	}

	assert.Equal(t, "Jane", *req.FirstName)
	assert.Nil(t, req.LastName)
	assert.Nil(t, req.Username)
	assert.Nil(t, req.Bio)
}

// =============================================================================
// NewResource Tests
// =============================================================================

func TestNewResource_ReturnsResource(t *testing.T) {
	resource := NewResource(nil, nil)
	assert.NotNil(t, resource)
}
