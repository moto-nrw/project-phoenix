// backend/services/schedule/errors.go
package schedule

import (
	"errors"
	"fmt"
)

// Common schedule errors
var (
	ErrDateframeNotFound = errors.New("dateframe not found")
	ErrTimeframeNotFound = errors.New("timeframe not found")
	// ErrCategoryNotAssignable rejects new references to missing or archived
	// activity categories. Existing rows may keep an archived category, but no
	// create, category change, or series split may assign one.
	ErrCategoryNotAssignable = errors.New("activity category is not available for new assignments")
	// ErrTimeframeRequiredByCareOffering covers both updates and deletions that
	// would make a linked care offering impossible to materialize.
	ErrTimeframeRequiredByCareOffering = errors.New("Zeitrahmen kann nicht geändert oder gelöscht werden: Ein verknüpftes Betreuungsangebot benötigt ihn") //nolint:staticcheck // ST1005: user-facing German message
	ErrRecurrenceRuleNotFound          = errors.New("recurrence rule not found")
	ErrInvalidDateRange                = errors.New("invalid date range")
	ErrInvalidTimeRange                = errors.New("invalid time range")
	ErrInvalidDuration                 = errors.New("invalid duration")
)

// Sentinel errors returned from the pickup/arrival care-exception write flows so
// the API layer can map each to a precise HTTP status instead of every in-tx
// failure collapsing into a 500.
var (
	// ErrCareExceptionStaffProfileRequired means a guardian-authored exception can
	// only be reclaimed by a user who has a staff profile.
	ErrCareExceptionStaffProfileRequired = errors.New("a staff profile is required to change a parent-set time")
	// ErrCareExceptionNotFound means the targeted exception no longer exists (it
	// was removed between the ownership pre-check and the locked re-read).
	ErrCareExceptionNotFound = errors.New("schedule exception not found")
	// ErrCareExceptionWrongStudent means the exception does not belong to the
	// student named in the path.
	ErrCareExceptionWrongStudent = errors.New("schedule exception does not belong to this student")
	// ErrCareExceptionDayConflict means a staff-authored exception already existed
	// for the day when a create came in.
	ErrCareExceptionDayConflict = errors.New("schedule exception for this day was just changed")
)

// ScheduleError represents a schedule-related error
type ScheduleError struct {
	Op  string // Operation that failed
	Err error  // Original error
}

// Error returns the error message
func (e *ScheduleError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("schedule error during %s", e.Op)
	}
	return fmt.Sprintf("schedule error during %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *ScheduleError) Unwrap() error {
	return e.Err
}
