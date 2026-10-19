package timetable

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StaffNoticeRetention deletes a school's ended staff notices once its team
// chat window has passed (#4045): a notice counts from its last valid day or,
// when the school switched it off earlier, from that last change. An active
// notice without a last valid day stays until the school deletes it. The
// acknowledgements go with their notice.
type StaffNoticeRetention interface {
	DeleteExpiredStaffNotices(ctx context.Context, retentionDays int) (int, error)
}

// ErrStaffNoticeRetentionWindow refuses a retention window that is not
// positive: nothing is deleted on a wrong period, and the run is caught up
// once the setting is fixed.
var ErrStaffNoticeRetentionWindow = errors.New("staff notice retention window must be positive")

// StaffNoticeRetentionStore deletes the staff notices of the school in
// context that ended before cutoff; the owner's staff notice repository is
// one.
type StaffNoticeRetentionStore interface {
	DeleteEndedBefore(ctx context.Context, cutoff calendar.Date) (int, error)
}

type staffNoticeRetention struct {
	store StaffNoticeRetentionStore
	today func() calendar.Date
}

// NewStaffNoticeRetention composes the deletion of ended staff notices over
// the store and the Berlin calendar clock.
func NewStaffNoticeRetention(store StaffNoticeRetentionStore, today func() calendar.Date) StaffNoticeRetention {
	if store == nil || today == nil {
		panic("timetable: staff notice retention requires its store and clock")
	}
	return staffNoticeRetention{store: store, today: today}
}

// DeleteExpiredStaffNotices deletes the notices that ended before the first
// Berlin calendar day a window of retentionDays keeps.
func (r staffNoticeRetention) DeleteExpiredStaffNotices(ctx context.Context, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("%w, got %d", ErrStaffNoticeRetentionWindow, retentionDays)
	}
	return r.store.DeleteEndedBefore(ctx, r.today().AddDays(-retentionDays))
}
