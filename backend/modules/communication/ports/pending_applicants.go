package ports

import (
	"context"
)

// PendingAnnouncementApplicant is one undecided enrollment application the
// pending_enrollment target reaches. Enrollment owns those rows; the
// composition root converts them so Communication never reads its tables.
type PendingAnnouncementApplicant struct {
	TenantID          int64
	GuardianFirstName string
	GuardianLastName  string
	GuardianAccountID *int64
	GuardianEmail     string
}

// PendingApplicantSource supplies Enrollment's undecided applications for
// the caller's school and for an explicit school set.
type PendingApplicantSource interface {
	PendingAnnouncementApplicants(context.Context) ([]PendingAnnouncementApplicant, error)
	PendingAnnouncementApplicantsForSchools(context.Context, []int64) ([]PendingAnnouncementApplicant, error)
}
