package education

import (
	"errors"
	"fmt"
)

// Common service errors
var (
	ErrGroupNotFound           = errors.New("education group not found")
	ErrTeacherNotFound         = errors.New("teacher not found")
	ErrGroupTeacherNotFound    = errors.New("group-teacher relationship not found")
	ErrSubstitutionNotFound    = errors.New("substitution not found")
	ErrRoomNotFound            = errors.New("room not found")
	ErrDuplicateGroup          = errors.New("a group with this name already exists")
	ErrDuplicateTeacherInGroup = errors.New("this teacher is already assigned to the group")
	ErrSubstitutionConflict    = errors.New("substitution conflicts with an existing one")
	ErrSameTeacherSubstitution = errors.New("regular staff and substitute staff cannot be the same")
	ErrInvalidDateRange        = errors.New("invalid date range")
	ErrDatabaseOperation       = errors.New("database operation failed")
	ErrInvalidData             = errors.New("invalid data provided")
)

// EducationError represents an error that occurred in the education service
type EducationError struct {
	Op  string // Operation that failed
	Err error  // Underlying error
}

// Error returns the error message
func (e *EducationError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("education: %s: unknown error", e.Op)
	}
	return fmt.Sprintf("education: %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *EducationError) Unwrap() error {
	return e.Err
}
