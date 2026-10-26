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
}
