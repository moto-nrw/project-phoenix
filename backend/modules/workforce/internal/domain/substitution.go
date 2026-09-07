package domain

import "time"

const (
	GroupSubstitutionTypeGroupHandover = "group_handover"
	GroupSubstitutionTypeLegacy        = "legacy_personnel_substitution"
)

// InvalidGroupSubstitutionError carries the caller-facing validation reason.
type InvalidGroupSubstitutionError struct{ Reason string }

func (e *InvalidGroupSubstitutionError) Error() string { return e.Reason }
func (e *InvalidGroupSubstitutionError) Unwrap() error { return ErrInvalidGroupSubstitution }

func invalidSubstitution(reason string) error { return &InvalidGroupSubstitutionError{Reason: reason} }

// GroupSubstitution is a temporary assignment of a substitute staff member to
// an education group. Dates are inclusive calendar days in DateLayout.
type GroupSubstitution struct {
	ID                int64
	TenantID          int64
	TargetType        string
	GroupID           int64
	RegularStaffID    *int64
	SubstituteStaffID int64
	StartDate         string
	EndDate           string
	Reason            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Validate enforces the invariants of a stored substitution. The wording is
// the established contract of the legacy repository.
func (s GroupSubstitution) Validate() error {
	if s.GroupID <= 0 {
		return invalidSubstitution("group ID is required")
	}
	if s.RegularStaffID != nil && *s.RegularStaffID <= 0 {
		return invalidSubstitution("regular staff ID must be positive if provided")
	}
	if s.SubstituteStaffID <= 0 {
		return invalidSubstitution("substitute staff ID is required")
	}
	if s.StartDate == "" {
		return invalidSubstitution("start date is required")
	}
	if s.EndDate == "" {
		return invalidSubstitution("end date is required")
	}
	if err := validateSubstitutionDate(s.StartDate, "start date"); err != nil {
		return err
	}
	if err := validateSubstitutionDate(s.EndDate, "end date"); err != nil {
		return err
	}
	if s.EndDate < s.StartDate {
		return invalidSubstitution("end date cannot be before start date")
	}
	if s.RegularStaffID != nil && *s.RegularStaffID == s.SubstituteStaffID {
		return invalidSubstitution("regular staff and substitute staff cannot be the same")
	}
	return nil
}

func validateSubstitutionDate(value, field string) error {
	parsed, err := time.Parse(DateLayout, value)
	if err != nil || parsed.Format(DateLayout) != value {
		return invalidSubstitution(field + " must be a " + DateLayout + " date")
	}
	return nil
}

type GroupSubstitutionFilter struct {
	TenantID          int64
	GroupID           int64
	GroupIDs          []int64
	SubstituteStaffID int64
	RegularStaffID    int64
	StaffID           int64
	TargetType        string
	On                string
	OverlapFrom       string
	OverlapTo         string
	EndsOnOrAfter     string
	ReasonContains    string
	Limit             int
	Offset            int
}

func (f GroupSubstitutionFilter) Validate() error {
	for _, pair := range []struct{ value, field string }{
		{f.On, "on"}, {f.OverlapFrom, "overlap_from"}, {f.OverlapTo, "overlap_to"}, {f.EndsOnOrAfter, "ends_on_or_after"},
	} {
		if pair.value == "" {
			continue
		}
		if err := validateSubstitutionDate(pair.value, pair.field); err != nil {
			return err
		}
	}
	if f.Limit < 0 || f.Offset < 0 {
		return invalidSubstitution("limit and offset must not be negative")
	}
	return nil
}
