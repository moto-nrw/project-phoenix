package records

import "fmt"

// AttendanceWindowError rejects a write that would let the visible window
// exceed the retention window. Key is the setting written; the HTTP layer
// names the conflicting window to the person through its own error code.
type AttendanceWindowError struct {
	Key           string
	VisibleDays   int
	RetentionDays int
}

func (e *AttendanceWindowError) Error() string {
	return fmt.Sprintf("invalid value for setting %s: attendance visible for %d days exceeds its retention of %d days",
		e.Key, e.VisibleDays, e.RetentionDays)
}

// VisibleWritten reports whether the write lengthened the visible window;
// otherwise it shortened the retention window.
func (e *AttendanceWindowError) VisibleWritten() bool {
	return e.Key == KeyAttendanceVisibleDays
}

// Unwrap keeps the rejection an invalid value (400) for every caller.
func (e *AttendanceWindowError) Unwrap() error {
	return ErrInvalidValue
}
