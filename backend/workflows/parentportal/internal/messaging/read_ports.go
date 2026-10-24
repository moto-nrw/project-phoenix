package messaging

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

// The read ports below are the rows of other owners the messaging flows read.
// They name only reads and row locks; this package writes only
// Communication's own conversation, announcement and sharing rows.

// GuardianProfileReads reads the guardian profiles messaging addresses.
type GuardianProfileReads interface {
	FindByAccountID(ctx context.Context, accountID int64) (*care.GuardianProfile, error)
	FindActivePortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*care.GuardianProfile, error)
}

// GuardianProfiles is the guardian-profile surface the child flows and the
// messaging flows read together; the portal composition takes one binding.
type GuardianProfiles interface {
	care.GuardianProfileReads
	GuardianProfileReads
}

// CareRequestReads reads the submitting account, child and kind of a request.
type CareRequestReads interface {
	FindIdentity(ctx context.Context, id int64) (submittedBy, studentID int64, requestKind string, err error)
}

// ExcusedRequestReads reads one absence request.
type ExcusedRequestReads interface {
	FindByID(ctx context.Context, id any) (*absencerecords.ExcusedAbsenceRequest, error)
}

// OfferingChangeRequestReads reads one offering change request.
type OfferingChangeRequestReads interface {
	FindByID(ctx context.Context, id int64) (*OfferingChangeIdentity, error)
}

// FamilyProtectionReads reads the current family protection of children.
type FamilyProtectionReads interface {
	CurrentForStudents(ctx context.Context, studentIDs []int64) (map[int64]*FamilyProtectionEvent, error)
}

// OfferingChangeIdentity is the child and submitting account of the request.
// It is not an authorization result; the sharing flow checks access itself.
type OfferingChangeIdentity struct {
	StudentID   int64
	SubmittedBy int64
}
