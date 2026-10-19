package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// StaffEmploymentStore is the persistence port over the staff employment
// profiles: the work-time-model binding, rotation anchor, notes and display
// preferences of a staff membership.
type StaffEmploymentStore interface {
	StaffEmployments(ctx context.Context, membershipIDs []int64) (map[int64]domain.StaffEmployment, error)
	StaffOnWorkTimeModel(ctx context.Context, workTimeModelID int64) ([]int64, error)
	LockStaffEmployment(ctx context.Context, membershipID int64) (domain.StaffEmployment, error)
	SaveStaffEmployment(ctx context.Context, value domain.StaffEmployment) error
	ReleaseWorkTimeModel(ctx context.Context, workTimeModelID int64, membershipIDs []int64) error
	SetStaffNotes(ctx context.Context, membershipID int64, notes string) error
	SetStaffBirthdayDisplayOptOut(ctx context.Context, membershipID int64, optOut bool) error
	RebaseStaffRotationAnchor(ctx context.Context, membershipIDs []int64, anchorDate string) error
}
