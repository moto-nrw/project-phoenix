package facilities

import (
	"errors"
	"fmt"
)

// Common facilities errors
var (
	ErrRoomNotFound         = errors.New("room not found")
	ErrDuplicateRoom        = errors.New("room with this name already exists")
	ErrInvalidRoomData      = errors.New("invalid room data")
	ErrRoomCapacityExceeded = errors.New("room capacity exceeded")
	ErrBuildingNotFound     = errors.New("building not found")
	ErrCategoryNotFound     = errors.New("category not found")
)

// FacilitiesError represents a facilities-related error
type FacilitiesError struct {
	Op  string // Operation that failed
	Err error  // Original error
}

// Error returns the error message
func (e *FacilitiesError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("facilities error during %s", e.Op)
	}
	return fmt.Sprintf("facilities error during %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *FacilitiesError) Unwrap() error {
	return e.Err
}
