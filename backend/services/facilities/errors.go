// backend/services/facilities/errors.go
package facilities

import (
	"errors"
	"fmt"
)

// Common facilities errors
var (
	ErrRoomNotFound         = errors.New("room not found")
	ErrDuplicateRoom        = errors.New("Ein Raum mit diesem Namen existiert bereits")               //nolint:staticcheck // ST1005: user-facing German message
	ErrDuplicateToiletRoom  = errors.New("Es existiert bereits ein Toilettenraum (WC oder Toilette)") //nolint:staticcheck // ST1005: user-facing German message
	ErrInvalidRoomData      = errors.New("invalid room data")
	ErrRoomCapacityExceeded = errors.New("room capacity exceeded")
	ErrBuildingNotFound     = errors.New("building not found")
	ErrCategoryNotFound     = errors.New("category not found")
	ErrRoomInUse            = errors.New("Raum kann nicht gelöscht werden: Raum wird aktuell von einer aktiven Gruppe verwendet") //nolint:staticcheck // ST1005: user-facing German message
	ErrSystemRoomProtected  = errors.New("Systemraum kann nicht gelöscht oder umbenannt werden")                                  //nolint:staticcheck // ST1005: user-facing German message
	ErrColorAlreadyInUse    = errors.New("Diese Farbe ist bereits einem anderen Raum zugeordnet")                                 //nolint:staticcheck // ST1005: user-facing German message
	ErrColorReserved        = errors.New("Diese Farbe ist für Statusbadges reserviert und kann nicht für Räume verwendet werden") //nolint:staticcheck // ST1005: user-facing German message
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
