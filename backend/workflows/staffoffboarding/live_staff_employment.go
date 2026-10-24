package staffoffboarding

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// LiveStaffMembership locks a staff member's membership row while the member
// still belongs to the school; an offboarded or unknown member fails with
// School Membership's error, and the workflow writes nothing.
type LiveStaffMembership interface {
	LockLiveStaff(ctx context.Context, staffID int64) error
}

// StaffEmploymentWrites are Workforce's employment-only staff writes.
type StaffEmploymentWrites interface {
	AppendStaffNotes(ctx context.Context, membershipID int64, notes string) (workforce.StaffEmployment, error)
	SetStaffBirthdayDisplayOptOut(ctx context.Context, membershipID int64, optOut bool) error
}

// LiveStaffEmployment writes the employment-only fields of a staff member
// who is still on the staff, so an offboarded member's profile stays as the
// offboarding left it (#2753). The membership lock comes first and the
// Workforce write runs inside it, in the caller's transaction.
type LiveStaffEmployment struct {
	membership LiveStaffMembership
	employment StaffEmploymentWrites
}

// NewLiveStaffEmployment binds the membership check to the Workforce writes.
func NewLiveStaffEmployment(membership LiveStaffMembership, employment StaffEmploymentWrites) (*LiveStaffEmployment, error) {
	if membership == nil || employment == nil {
		return nil, errors.New("live staff employment: membership and employment are required")
	}
	return &LiveStaffEmployment{membership: membership, employment: employment}, nil
}

// SetBirthdayDisplayOptOut records whether a live staff member's birthday
// stays off the shared birthday list.
func (w *LiveStaffEmployment) SetBirthdayDisplayOptOut(ctx context.Context, staffID int64, optOut bool) error {
	if err := w.membership.LockLiveStaff(ctx, staffID); err != nil {
		return err
	}
	return w.employment.SetStaffBirthdayDisplayOptOut(ctx, staffID, optOut)
}

// AppendNotes adds a paragraph to a live staff member's private notes.
func (w *LiveStaffEmployment) AppendNotes(ctx context.Context, staffID int64, notes string) error {
	if err := w.membership.LockLiveStaff(ctx, staffID); err != nil {
		return err
	}
	_, err := w.employment.AppendStaffNotes(ctx, staffID, notes)
	return err
}
