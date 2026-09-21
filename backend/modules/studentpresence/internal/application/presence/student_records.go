package presence

import (
	"context"
)

// PresenceStudents is the student access the attendance service needs:
// row-locked reads for check-in guards and the live-flag write-back.
type PresenceStudents interface {
	FindByID(context.Context, int64) (*StudentRecord, error)
	FindByIDForUpdate(context.Context, int64) (*StudentRecord, error)
	FindByIDsForUpdate(context.Context, []int64) (map[int64]*StudentRecord, error)
	// UpdateLiveStatus persists the record's sick/excused flags and their
	// timestamps; no other column is touched.
	UpdateLiveStatus(context.Context, *StudentRecord) error
}
