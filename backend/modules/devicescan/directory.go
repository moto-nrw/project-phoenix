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

// DirectoryStudent is the minimal student projection used for kiosk selection
// and bracelet assignment. IDs remain JSON numbers for the established wire.
type DirectoryStudent struct {
	StudentID   int64  `json:"student_id"`
	PersonID    int64  `json:"person_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
	GroupName   string `json:"group_name"`
	RFIDTag     string `json:"rfid_tag,omitempty"`
}
