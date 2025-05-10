// backend/services/facilities/errors.go
package facilities

import (
	"errors"
	"fmt"
)

// Common facilities errors
var (
	ErrRoomNotFound          = errors.New("room not found")
	ErrInvalidRoomData       = errors.New("invalid room data")
	ErrRoomAlreadyExists     = errors.New("room already exists")
	ErrInvalidCapacity       = errors.New("invalid capacity")
	ErrInvalidSearchCriteria = errors.New("invalid search criteria")
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
