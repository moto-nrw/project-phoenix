// backend/services/facilities/errors.go
package facilities

import (
	"errors"
	"fmt"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

// Common facilities errors
var (
	ErrRoomNotFound               = facilitiesModule.ErrRoomNotFound
	ErrDuplicateRoom              = facilitiesModule.ErrDuplicateRoom
	ErrDuplicateToiletRoom        = facilitiesModule.ErrDuplicateToiletRoom
	ErrRoomInUse                  = facilitiesModule.ErrRoomInUse
	ErrRoomRequiredByCareOffering = facilitiesModule.ErrRoomRequiredByOffering
	ErrSystemRoomProtected        = facilitiesModule.ErrSystemRoomProtected
	ErrSystemRoomNameReserved     = facilitiesModule.ErrSystemRoomNameReserved
	ErrColorAlreadyInUse          = facilitiesModule.ErrRoomColorAlreadyInUse
	ErrColorReserved              = facilitiesModule.ErrRoomColorReserved
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

// Code classifies stable domain failures without requiring composition roots
// to import this legacy application package.
func (e *FacilitiesError) Code() string {
	switch {
	case errors.Is(e.Err, ErrRoomInUse):
		return "room_in_use"
	case errors.Is(e.Err, ErrRoomRequiredByCareOffering):
		return "room_required_by_offering"
	default:
		return "internal_error"
	}
}
