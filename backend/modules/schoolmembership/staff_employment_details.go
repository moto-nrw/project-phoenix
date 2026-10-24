package schoolmembership

import "context"

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
	// LockLiveStaff locks a live membership and refuses a retired or unknown
	// one with ErrStaffNotFound. Inside the caller's transaction the
	// membership stays locked until it commits, so a write the caller makes
	// after it cannot land on a member an offboarding retires meanwhile.
	LockLiveStaff(ctx context.Context, staffID int64) error
}
