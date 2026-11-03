package studentpresence

import (
	"time"
)

type SchoolStatus struct {
	StudentID                            int64
	CheckedInBy                          int64
	CheckedOutBy                         *int64
	CheckedOutDeviceID                   *int64
	Date, Status                         string
	CheckInTime, CheckOutTime, YardSince *time.Time
	CheckOutNote                         *string
	CheckOutReason                       *string
}

// IsInClass reports a child whose last stay of the day ended with "Geht zum
// Unterricht" (#5656).
func (s SchoolStatus) IsInClass() bool {
	return s.Status == "checked_out" && isClassReason(s.CheckOutReason)
}
