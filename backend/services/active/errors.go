package active

import (
	"errors"
	"fmt"
)

// Common service errors
var (
	ErrActiveGroupNotFound       = errors.New("active group not found")
	ErrVisitNotFound             = errors.New("visit not found")
	ErrGroupSupervisorNotFound   = errors.New("group supervisor not found")
	ErrCombinedGroupNotFound     = errors.New("combined group not found")
	ErrGroupMappingNotFound      = errors.New("group mapping not found")
	ErrActiveGroupAlreadyEnded   = errors.New("active group session already ended")
	ErrVisitAlreadyEnded         = errors.New("visit already ended")
	ErrSupervisionAlreadyEnded   = errors.New("supervision already ended")
	ErrCombinedGroupAlreadyEnded = errors.New("combined group already ended")
	ErrStudentAlreadyInGroup     = errors.New("student already present in this group")
	ErrGroupAlreadyInCombination = errors.New("group already part of this combination")
	ErrInvalidTimeRange          = errors.New("invalid time range")
	ErrCannotDeleteActiveGroup   = errors.New("cannot delete active group with active visits")
	ErrStudentAlreadyActive      = errors.New("student already has an active visit")
	ErrStaffAlreadySupervising   = errors.New("staff member already supervising this group")
	ErrInvalidData               = errors.New("invalid data provided")
	ErrDatabaseOperation         = errors.New("database operation failed")
	// Activity session management errors
	ErrActivityAlreadyActive     = errors.New("activity is already active on another device")
	ErrDeviceAlreadyActive       = errors.New("device is already running an activity session")
	ErrNoActiveSession           = errors.New("no active session found")
	ErrSessionConflict           = errors.New("session conflict detected")
	ErrInvalidActivitySession    = errors.New("invalid activity session parameters")
)

// ActiveError represents an error that occurred in the active service
type ActiveError struct {
	Op  string // Operation that failed
	Err error  // Underlying error
}

// Error returns the error message
func (e *ActiveError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("active: %s: unknown error", e.Op)
	}
	return fmt.Sprintf("active: %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *ActiveError) Unwrap() error {
	return e.Err
}
