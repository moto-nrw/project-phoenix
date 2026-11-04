package devicescan

import "context"

// Directory is the authenticated kiosk roster. A nil teacher filter selects
// all students; a non-nil empty filter selects none. The caller must retain
// that distinction when parsing teacher_ids.
type Directory interface {
	Teachers(context.Context) ([]Teacher, error)
	Students(context.Context, []int64) ([]DirectoryStudent, error)
	Activities(context.Context) ([]DirectoryActivity, error)
}

// DirectoryActivity reports the occupancy of an activity the kiosk can select.
type DirectoryActivity struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	IsOccupied bool   `json:"is_occupied"`
}

// Teacher is a teacher-roster entry, independent of caregiver account state.
type Teacher struct {
	StaffID     int64  `json:"staff_id"`
	PersonID    int64  `json:"person_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	DisplayName string `json:"display_name"`
}

// HolidayGuestSchools names, by student, the school the hint „Gast aus …“
// on a holiday guest names (#5630), for every guest the school cares for
// today. A guest without a named school is absent. It runs in the tenant
// transaction of the school.
type HolidayGuestSchools func(ctx context.Context, schoolID int64) (map[int64]string, error)

// DirectoryStudent is the minimal student projection used for kiosk selection
// and bracelet assignment. IDs remain JSON numbers for the established wire.
// HolidayGuestSchool is the school a holiday guest comes from, only during
// its week at the hosting school (PyrePortal#499); empty otherwise and for
// a guest without a named school, whose class stays „Feriengast“.
type DirectoryStudent struct {
	StudentID   int64  `json:"student_id"`
	PersonID    int64  `json:"person_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
	GroupName   string `json:"group_name"`
	RFIDTag     string `json:"rfid_tag,omitempty"`
	// HolidayGuestSchool is omitted when empty, so an older tablet sees the
	// established wire.
	HolidayGuestSchool string `json:"holiday_guest_school,omitempty"`
}
