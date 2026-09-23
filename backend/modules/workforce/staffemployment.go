package workforce

import (
	"context"
	"errors"
)

// ErrStaffEmploymentNotFound means the membership carries no employment
// profile in the caller's school.
var ErrStaffEmploymentNotFound = errors.New("staff employment profile not found")

// StaffEmployment is the Workforce half of a staff member (#2753): the
// employment facts kept on users.staff_employment_profiles, keyed by the
// School Membership row. RotationAnchorDate is a calendar date in DateLayout,
// empty when unset.
type StaffEmployment struct {
	MembershipID          int64
	StaffNotes            string
	EmploymentType        *string
	WorkTimeModelID       *int64
	PersonnelNumber       *string
	RotationAnchorDate    string
	BirthdayDisplayOptOut bool
}

// StaffEmploymentQuery reads employment profiles in the caller's transaction.
type StaffEmploymentQuery interface {
	// StaffEmployments returns the profiles of the given memberships; a
	// membership without one is absent from the map.
	StaffEmployments(ctx context.Context, membershipIDs []int64) (map[int64]StaffEmployment, error)
	// StaffOnWorkTimeModel returns the memberships bound to a work-time
	// template, sorted by ID, whether or not the membership is still live.
	StaffOnWorkTimeModel(ctx context.Context, workTimeModelID int64) ([]int64, error)
}

// StaffEmploymentCommand is the only writer of users.staff_employment_profiles.
// A taken personnel number fails with ErrPersonnelNumberTaken.
type StaffEmploymentCommand interface {
	// SaveStaffEmployment creates or replaces the membership's profile.
	SaveStaffEmployment(context.Context, StaffEmployment) error
	// ClearStaffWorkTimeModel detaches the membership from its template.
	ClearStaffWorkTimeModel(ctx context.Context, membershipID int64) error
	// AppendStaffNotes adds a paragraph to the private staff notes under a
	// row lock and returns the updated profile.
	AppendStaffNotes(ctx context.Context, membershipID int64, notes string) (StaffEmployment, error)
	SetStaffBirthdayDisplayOptOut(ctx context.Context, membershipID int64, optOut bool) error
	// RebaseStaffRotationAnchor stamps the anchor date onto the given
	// memberships' profiles.
	RebaseStaffRotationAnchor(ctx context.Context, membershipIDs []int64, anchorDate string) error
}

type StaffEmployments interface {
	StaffEmploymentQuery
	StaffEmploymentCommand
}
