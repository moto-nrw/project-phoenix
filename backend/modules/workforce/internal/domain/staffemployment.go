package domain

import "errors"

var (
	ErrStaffEmploymentNotFound = errors.New("staff employment profile not found")
	ErrPersonnelNumberTaken    = errors.New("personnel number is already taken")
)

// StaffEmployment is one users.staff_employment_profiles row.
type StaffEmployment struct {
	MembershipID          int64
	StaffNotes            string
	EmploymentType        *string
	WorkTimeModelID       *int64
	PersonnelNumber       *string
	RotationAnchorDate    string
	BirthdayDisplayOptOut bool
}

// AppendStaffNotes joins a new paragraph onto existing staff notes.
func AppendStaffNotes(existing, notes string) string {
	if existing == "" {
		return notes
	}
	return existing + "\n" + notes
}
