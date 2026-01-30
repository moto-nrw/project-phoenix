package suggestions

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostNotFoundError_Error(t *testing.T) {
	err := &PostNotFoundError{PostID: 42}
	assert.Equal(t, ErrPostNotFound.Error(), err.Error())
}

func TestPostNotFoundError_Unwrap(t *testing.T) {
	err := &PostNotFoundError{PostID: 42}
	assert.True(t, errors.Is(err, ErrPostNotFound))
}

func TestForbiddenError_Error_Default(t *testing.T) {
	err := &ForbiddenError{}
	assert.Equal(t, ErrForbidden.Error(), err.Error())
}

func TestForbiddenError_Error_CustomReason(t *testing.T) {
	err := &ForbiddenError{Reason: "custom reason"}
	assert.Equal(t, "custom reason", err.Error())
}

func TestForbiddenError_Unwrap(t *testing.T) {
	err := &ForbiddenError{}
	assert.True(t, errors.Is(err, ErrForbidden))
}

func TestInvalidDataError_Error_WithInner(t *testing.T) {
	inner := fmt.Errorf("title is required")
	err := &InvalidDataError{Err: inner}
	assert.Equal(t, "title is required", err.Error())
}

func TestInvalidDataError_Error_NilInner(t *testing.T) {
	err := &InvalidDataError{}
	assert.Equal(t, ErrInvalidData.Error(), err.Error())
}

func TestInvalidDataError_Unwrap(t *testing.T) {
	err := &InvalidDataError{Err: fmt.Errorf("some detail")}
	assert.True(t, errors.Is(err, ErrInvalidData))
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	require.False(t, errors.Is(ErrPostNotFound, ErrForbidden))
	require.False(t, errors.Is(ErrPostNotFound, ErrInvalidData))
	require.False(t, errors.Is(ErrForbidden, ErrInvalidData))
}
