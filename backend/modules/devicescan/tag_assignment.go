package devicescan

import "context"

// TagAssignmentQuery reports whether a bracelet belongs to a current student
// or a staff member. A free bracelet is not a failed attendance scan.
type TagAssignmentQuery interface {
	LookupTagAssignment(context.Context, string) (TagAssignment, error)
}

type StaffTagAssignments interface {
	AssignStaffTag(context.Context, int64, string) (TagAssignmentChange, error)
	UnassignStaffTag(context.Context, int64) (TagAssignmentChange, error)
}

type TagAssignments interface {
	TagAssignmentQuery
	StaffTagAssignments
}

// TagAssignmentChange retains the student-named fields also used for staff
// by the established kiosk wire contract.
type TagAssignmentChange struct {
	Success     bool    `json:"success"`
	StudentID   int64   `json:"student_id"`
	StudentName string  `json:"student_name"`
	RFIDTag     string  `json:"rfid_tag"`
	PreviousTag *string `json:"previous_tag,omitempty"`
	Message     string  `json:"message"`
}

type TagAssignment struct {
	Assigned   bool                `json:"assigned"`
	PersonType string              `json:"person_type,omitempty"`
	Person     *TagAssignedPerson  `json:"person,omitempty"`
	Student    *TagAssignedStudent `json:"student,omitempty"`
}

type TagAssignedPerson struct {
	ID       int64  `json:"id"`
	PersonID int64  `json:"person_id"`
	Name     string `json:"name"`
	Group    string `json:"group"`
}

// TagAssignedStudent retains the deprecated student wire for older kiosks.
type TagAssignedStudent struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"`
}
