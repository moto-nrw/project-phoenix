package care

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// AbsenceReport announces the parent's accepted absence inside its transaction.
// Delivery chooses recipients and lock-screen-safe wording.
type AbsenceReport struct {
	TenantID       int64
	StudentIDs     []int64
	Status         string
	Dates          []timezone.Date
	FromParent     bool
	ActorAccountID int64
}

type AbsenceNotifier interface {
	NotifyAbsenceReported(context.Context, AbsenceReport) error
}

// StaffParentMessageReport identifies the guardian message to deliver to staff.
type StaffParentMessageReport struct {
	TenantID       int64
	ThreadID       int64
	MessageID      int64
	StudentID      int64
	ActorAccountID int64
}

type StaffParentMessageNotifier interface {
	NotifyStaffParentMessage(context.Context, StaffParentMessageReport) error
}
