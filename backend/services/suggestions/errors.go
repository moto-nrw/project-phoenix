package suggestions

import (
	"errors"
)

// Sentinel errors for the suggestions service
var (
	ErrPostNotFound    = errors.New("suggestion post not found")
	ErrCommentNotFound = errors.New("comment not found")
	ErrForbidden       = errors.New("forbidden: you can only modify your own suggestions")
	ErrInvalidData     = errors.New("invalid suggestion data")
)

// PostNotFoundError wraps post-not-found with additional context
type PostNotFoundError struct {
	PostID int64
}

func (e *PostNotFoundError) Error() string {
	return ErrPostNotFound.Error()
}

func (e *PostNotFoundError) Unwrap() error {
	return ErrPostNotFound
}

// CommentNotFoundError wraps comment-not-found with additional context
type CommentNotFoundError struct {
	CommentID int64
}

func (e *CommentNotFoundError) Error() string {
	return ErrCommentNotFound.Error()
}

func (e *CommentNotFoundError) Unwrap() error {
	return ErrCommentNotFound
}

// ForbiddenError wraps permission errors
type ForbiddenError struct {
	Reason string
}

func (e *ForbiddenError) Error() string {
	if e.Reason != "" {
		return e.Reason
	}
	return ErrForbidden.Error()
}

func (e *ForbiddenError) Unwrap() error {
	return ErrForbidden
}

// InvalidDataError wraps validation errors
type InvalidDataError struct {
	Err error
}

func (e *InvalidDataError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return ErrInvalidData.Error()
}

func (e *InvalidDataError) Unwrap() error {
	return ErrInvalidData
}
