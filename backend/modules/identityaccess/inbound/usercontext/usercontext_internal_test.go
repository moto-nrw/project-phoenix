// Package usercontext internal tests for pure helper functions.
// These tests verify logic that doesn't require database access.
//
// Upload helper tests (detectContentType, CloseFile, etc.) moved to api/common/.
package usercontext

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// ProfileUpdateRequest Tests
// =============================================================================

func TestProfileUpdateRequest_Fields(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	var req ProfileUpdateRequest
	assert.Nil(t, req.FirstName)
	assert.Nil(t, req.LastName)
	assert.Nil(t, req.Username)
	assert.Nil(t, req.Bio)
}

func TestProfileUpdateRequest_PartialFields(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	resource := NewResource(nil, nil)
	assert.NotNil(t, resource)
}
