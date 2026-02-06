package suggestions

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// PostNotFoundError Tests
// =============================================================================

func TestPostNotFoundError_Error(t *testing.T) {
	err := &PostNotFoundError{PostID: 123}
	assert.Equal(t, "suggestion post not found", err.Error())
}

func TestPostNotFoundError_Unwrap(t *testing.T) {
	err := &PostNotFoundError{PostID: 123}
	unwrapped := err.Unwrap()
	assert.Equal(t, ErrPostNotFound, unwrapped)
	assert.True(t, errors.Is(err, ErrPostNotFound))
}

// =============================================================================
// CommentNotFoundError Tests
// =============================================================================

func TestCommentNotFoundError_Error(t *testing.T) {
	err := &CommentNotFoundError{CommentID: 456}
	assert.Equal(t, "comment not found", err.Error())
}

func TestCommentNotFoundError_Unwrap(t *testing.T) {
	err := &CommentNotFoundError{CommentID: 456}
	unwrapped := err.Unwrap()
	assert.Equal(t, ErrCommentNotFound, unwrapped)
	assert.True(t, errors.Is(err, ErrCommentNotFound))
}

// =============================================================================
// ForbiddenError Tests
// =============================================================================

func TestForbiddenError_Error_WithReason(t *testing.T) {
	err := &ForbiddenError{Reason: "you cannot edit this post"}
	assert.Equal(t, "you cannot edit this post", err.Error())
}

func TestForbiddenError_Error_WithoutReason(t *testing.T) {
	err := &ForbiddenError{}
	assert.Equal(t, "forbidden: you can only modify your own suggestions", err.Error())
}

func TestForbiddenError_Error_EmptyReason(t *testing.T) {
	err := &ForbiddenError{Reason: ""}
	assert.Equal(t, "forbidden: you can only modify your own suggestions", err.Error())
}

func TestForbiddenError_Unwrap(t *testing.T) {
	err := &ForbiddenError{Reason: "custom reason"}
	unwrapped := err.Unwrap()
	assert.Equal(t, ErrForbidden, unwrapped)
	assert.True(t, errors.Is(err, ErrForbidden))
}

// =============================================================================
// InvalidDataError Tests
// =============================================================================

func TestInvalidDataError_Error_WithErr(t *testing.T) {
	innerErr := errors.New("field X is required")
	err := &InvalidDataError{Err: innerErr}
	assert.Equal(t, "field X is required", err.Error())
}

func TestInvalidDataError_Error_WithoutErr(t *testing.T) {
	err := &InvalidDataError{}
	assert.Equal(t, "invalid suggestion data", err.Error())
}

func TestInvalidDataError_Error_NilErr(t *testing.T) {
	err := &InvalidDataError{Err: nil}
	assert.Equal(t, "invalid suggestion data", err.Error())
}

func TestInvalidDataError_Unwrap(t *testing.T) {
	err := &InvalidDataError{Err: errors.New("validation failed")}
	unwrapped := err.Unwrap()
	assert.Equal(t, ErrInvalidData, unwrapped)
	assert.True(t, errors.Is(err, ErrInvalidData))
}

// =============================================================================
// Sentinel Errors Tests
// =============================================================================

func TestSentinelErrors(t *testing.T) {
	assert.Equal(t, "suggestion post not found", ErrPostNotFound.Error())
	assert.Equal(t, "comment not found", ErrCommentNotFound.Error())
	assert.Equal(t, "forbidden: you can only modify your own suggestions", ErrForbidden.Error())
	assert.Equal(t, "invalid suggestion data", ErrInvalidData.Error())
}

// =============================================================================
// errors.Is() Compatibility Tests
// =============================================================================

func TestErrorsIs_PostNotFound(t *testing.T) {
	err := &PostNotFoundError{PostID: 1}
	assert.True(t, errors.Is(err, ErrPostNotFound))
	assert.False(t, errors.Is(err, ErrCommentNotFound))
	assert.False(t, errors.Is(err, ErrForbidden))
}

func TestErrorsIs_CommentNotFound(t *testing.T) {
	err := &CommentNotFoundError{CommentID: 1}
	assert.True(t, errors.Is(err, ErrCommentNotFound))
	assert.False(t, errors.Is(err, ErrPostNotFound))
	assert.False(t, errors.Is(err, ErrForbidden))
}

func TestErrorsIs_Forbidden(t *testing.T) {
	err := &ForbiddenError{Reason: "test"}
	assert.True(t, errors.Is(err, ErrForbidden))
	assert.False(t, errors.Is(err, ErrPostNotFound))
	assert.False(t, errors.Is(err, ErrInvalidData))
}

func TestErrorsIs_InvalidData(t *testing.T) {
	err := &InvalidDataError{Err: errors.New("test")}
	assert.True(t, errors.Is(err, ErrInvalidData))
	assert.False(t, errors.Is(err, ErrPostNotFound))
	assert.False(t, errors.Is(err, ErrForbidden))
}
