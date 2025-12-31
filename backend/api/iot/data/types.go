package data

// DeviceTeacherResponse represents a teacher available for device login selection
type DeviceTeacherResponse struct {
	StaffID     int64  `json:"staff_id"`
	PersonID    int64  `json:"person_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	DisplayName string `json:"display_name"`
}

// TeacherStudentResponse represents a student supervised by a teacher for RFID devices
type TeacherStudentResponse struct {
	StudentID   int64  `json:"student_id"`
	PersonID    int64  `json:"person_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
	GroupName   string `json:"group_name"`
	RFIDTag     string `json:"rfid_tag,omitempty"`
}

// DeviceActivityResponse represents an activity available for teacher selection on RFID devices
type DeviceActivityResponse struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	CategoryName    string `json:"category_name"`
	CategoryColor   string `json:"category_color,omitempty"`
	RoomName        string `json:"room_name,omitempty"`
	EnrollmentCount int    `json:"enrollment_count"`
	MaxParticipants int    `json:"max_participants"`
	HasSpots        bool   `json:"has_spots"`
	SupervisorName  string `json:"supervisor_name"`
	IsActive        bool   `json:"is_active"`
}

// TeacherActivityResponse represents an activity in the teacher's activity list
type TeacherActivityResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// DeviceRoomResponse represents a room available for RFID device selection
type DeviceRoomResponse struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Building   string  `json:"building,omitempty"`
	Floor      *int    `json:"floor,omitempty"`
	Capacity   *int    `json:"capacity,omitempty"`
	Category   *string `json:"category,omitempty"`
	Color      *string `json:"color,omitempty"`
	IsOccupied bool    `json:"is_occupied"`
}

// RFIDTagAssignmentResponse represents RFID tag assignment status
type RFIDTagAssignmentResponse struct {
	Assigned   bool                    `json:"assigned"`
	PersonType string                  `json:"person_type,omitempty"` // "student" or "staff"
	Person     *RFIDTagAssignedPerson  `json:"person,omitempty"`
	Student    *RFIDTagAssignedStudent `json:"student,omitempty"` // Deprecated: kept for backward compatibility
}

// RFIDTagAssignedPerson represents person info for assigned RFID tag (generic)
type RFIDTagAssignedPerson struct {
	ID       int64  `json:"id"`        // Student ID or Staff ID (not person_id)
	PersonID int64  `json:"person_id"` // Underlying person ID
	Name     string `json:"name"`
	Group    string `json:"group"` // School class for students, role/specialization for staff
}

// RFIDTagAssignedStudent represents student info for assigned RFID tag
// Deprecated: Use RFIDTagAssignedPerson instead
type RFIDTagAssignedStudent struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"`
}
