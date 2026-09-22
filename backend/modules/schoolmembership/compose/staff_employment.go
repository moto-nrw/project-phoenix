package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// StaffEmployment is the consumer-owned port over Workforce's employment
// profile of a staff membership (#2753). The composition root binds it to the
// Workforce StaffEmployments capability; School Membership never reads or
// writes users.staff_employment_profiles itself. Errors that callers classify
// are returned as schoolmembership errors (ErrPersonnelNumberConflict,
// ErrStaffNotFound).
type StaffEmployment interface {
	StaffEmployments(ctx context.Context, membershipIDs []int64) (map[int64]StaffEmploymentProfile, error)
	SaveStaffEmployment(context.Context, StaffEmploymentProfile) error
	ClearStaffWorkTimeModel(ctx context.Context, membershipID int64) error
}

// StaffEmploymentProfile is the employment half of a staff member.
// RotationAnchorDate is a calendar date in DateLayout, empty when unset.
type StaffEmploymentProfile struct {
	MembershipID          int64
	StaffNotes            string
	EmploymentType        *string
	WorkTimeModelID       *int64
	PersonnelNumber       *string
	RotationAnchorDate    string
	BirthdayDisplayOptOut bool
}

type staffEmployment struct{ port StaffEmployment }

func (e staffEmployment) StaffEmployments(ctx context.Context, ids []int64) (map[int64]domain.StaffEmployment, error) {
	values, err := e.port.StaffEmployments(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]domain.StaffEmployment, len(values))
	for id, value := range values {
		result[id] = domain.StaffEmployment(value)
	}
	return result, nil
}

func (e staffEmployment) SaveStaffEmployment(ctx context.Context, value domain.StaffEmployment) error {
	return e.port.SaveStaffEmployment(ctx, StaffEmploymentProfile(value))
}

func (e staffEmployment) ClearStaffWorkTimeModel(ctx context.Context, membershipID int64) error {
	return e.port.ClearStaffWorkTimeModel(ctx, membershipID)
}

var errStaffEmploymentUnbound = errors.New("school membership compose: staff employment is required")
