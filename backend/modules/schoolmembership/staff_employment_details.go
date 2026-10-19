package schoolmembership

import (
	"context"
	"strings"
)

// UpdateEmploymentDetails replaces the employment type, the personnel
// number or both of a live staff member, the facts the personnel record
// (#3752) changes on its own. A field whose Set flag is false keeps its
// stored value; a nil value clears it.
type UpdateEmploymentDetails struct {
	ID                 int64
	SetEmploymentType  bool
	EmploymentType     *string
	SetPersonnelNumber bool
	PersonnelNumber    *string
}

// BindSchedule binds a live staff member to a work-time model from a
// rotation anchor day (time tracking). Both replace the stored values;
// RotationAnchorDate is a calendar date in DateLayout or empty.
type BindSchedule struct {
	ID                 int64
	WorkTimeModelID    *int64
	RotationAnchorDate string
}

// StaffEmploymentCommands write the employment half of a live staff member
// without the rest of the row. Each command locks the membership first, so
// an offboarded staff member is ErrStaffNotFound and nothing is written.
type StaffEmploymentCommands interface {
	UpdateEmploymentDetails(context.Context, UpdateEmploymentDetails) (Staff, error)
	BindSchedule(context.Context, BindSchedule) (Staff, error)
	// WithLiveStaff runs write only for a live membership. Inside the
	// caller's transaction the membership stays locked until it commits;
	// without one the check and the write are two short transactions, and an
	// offboarding that commits in between leaves the write on the retired
	// member, which nothing reads for live staff.
	WithLiveStaff(ctx context.Context, staffID int64, write func(context.Context) error) error
}

func (m *Module) UpdateEmploymentDetails(ctx context.Context, input UpdateEmploymentDetails) (Staff, error) {
	if input.ID <= 0 {
		return Staff{}, invalid("staff ID is required")
	}
	if input.SetEmploymentType {
		if err := validateEmploymentType(input.EmploymentType); err != nil {
			return Staff{}, err
		}
	}
	if input.SetPersonnelNumber {
		input.PersonnelNumber = normalizePersonnelNumber(input.PersonnelNumber)
	}
	return m.engine.UpdateEmploymentDetails(ctx, input)
}

func (m *Module) BindSchedule(ctx context.Context, input BindSchedule) (Staff, error) {
	if input.ID <= 0 {
		return Staff{}, invalid("staff ID is required")
	}
	if input.WorkTimeModelID != nil && *input.WorkTimeModelID <= 0 {
		return Staff{}, invalid("work time model ID must be positive")
	}
	if err := validateDate(input.RotationAnchorDate, "rotation anchor date"); err != nil {
		return Staff{}, err
	}
	return m.engine.BindSchedule(ctx, input)
}

func (m *Module) WithLiveStaff(ctx context.Context, staffID int64, write func(context.Context) error) error {
	if _, err := m.FindStaffForMutation(ctx, staffID); err != nil {
		return err
	}
	return write(ctx)
}

func validateEmploymentType(value *string) error {
	if value == nil {
		return nil
	}
	switch *value {
	case EmploymentTypeFullTime, EmploymentTypePartTime, EmploymentTypeMinijob:
		return nil
	default:
		return invalid("employment_type must be 'full_time', 'part_time', or 'minijob'")
	}
}

// normalizePersonnelNumber trims the number; a blank one is no number.
func normalizePersonnelNumber(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
