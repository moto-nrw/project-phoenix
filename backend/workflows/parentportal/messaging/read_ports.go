package messaging

import (
	"context"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
)

// The read ports below are the rows of other owners the messaging flows read.
// They name only reads and row locks; this package writes only
// Communication's own conversation, announcement and sharing rows.

// GuardianProfileReads reads the guardian profiles messaging addresses.
type GuardianProfileReads interface {
	FindByAccountID(ctx context.Context, accountID int64) (*usersModels.GuardianProfile, error)
	FindActivePortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.GuardianProfile, error)
}

// CareRequestReads reads one care-schedule change request.
type CareRequestReads interface {
	FindByID(ctx context.Context, id any) (*scheduleModels.CareScheduleChangeRequest, error)
}

// ExcusedRequestReads reads one absence request.
type ExcusedRequestReads interface {
	FindByID(ctx context.Context, id any) (*activeModels.ExcusedAbsenceRequest, error)
}

// OfferingChangeRequestReads reads one offering change request.
type OfferingChangeRequestReads interface {
	FindByID(ctx context.Context, id any) (*enrollmentModels.OfferingChangeRequest, error)
}

// FamilyProtectionReads reads the current family protection of children.
type FamilyProtectionReads interface {
	CurrentForStudents(ctx context.Context, studentIDs []int64) (map[int64]*usersModels.FamilyProtectionEvent, error)
}
