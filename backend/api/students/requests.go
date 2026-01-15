package students

import (
	"errors"
	"net/http"
)

// StudentRequest represents a student creation request with person details
type StudentRequest struct {
	// Person details (required)
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	TagID     string `json:"tag_id,omitempty"`   // RFID tag ID (optional)
	Birthday  string `json:"birthday,omitempty"` // Date in YYYY-MM-DD format

	// Student-specific details (required)
	SchoolClass string `json:"school_class"`

	// Legacy guardian fields (optional - use guardian_profiles system instead)
	GuardianName    string `json:"guardian_name,omitempty"`
	GuardianContact string `json:"guardian_contact,omitempty"`
	GuardianEmail   string `json:"guardian_email,omitempty"`
	GuardianPhone   string `json:"guardian_phone,omitempty"`

	// Optional fields
	GroupID         *int64  `json:"group_id,omitempty"`
	ExtraInfo       *string `json:"extra_info,omitempty"`       // Extra information visible to supervisors
	HealthInfo      *string `json:"health_info,omitempty"`      // Static health and medical information
	SupervisorNotes *string `json:"supervisor_notes,omitempty"` // Notes from supervisors
	PickupStatus    *string `json:"pickup_status,omitempty"`    // How the child gets home
	Bus             *bool   `json:"bus,omitempty"`              // Administrative permission flag (Buskind)
}

// Bind validates the student request
func (req *StudentRequest) Bind(_ *http.Request) error {
	// Basic validation for person fields
	if req.FirstName == "" {
		return errors.New("first name is required")
	}
	if req.LastName == "" {
		return errors.New("last name is required")
	}

	// Basic validation for student fields
	if req.SchoolClass == "" {
		return errors.New("school class is required")
	}

	// Guardian fields are now optional (legacy fields - use guardian_profiles system instead)
	// No validation required for guardian fields

	return nil
}

// UpdateStudentRequest represents a student update request
type UpdateStudentRequest struct {
	// Person details (optional for update)
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Birthday  *string `json:"birthday,omitempty"` // Date in YYYY-MM-DD format
	TagID     *string `json:"tag_id,omitempty"`

	// Student-specific details (optional for update)
	SchoolClass     *string `json:"school_class,omitempty"`
	GuardianName    *string `json:"guardian_name,omitempty"`
	GuardianContact *string `json:"guardian_contact,omitempty"`
	GuardianEmail   *string `json:"guardian_email,omitempty"`
	GuardianPhone   *string `json:"guardian_phone,omitempty"`
	GroupID         *int64  `json:"group_id,omitempty"`
	HealthInfo      *string `json:"health_info,omitempty"`      // Static health and medical information
	SupervisorNotes *string `json:"supervisor_notes,omitempty"` // Notes from supervisors
	ExtraInfo       *string `json:"extra_info,omitempty"`       // Extra information visible to supervisors
	PickupStatus    *string `json:"pickup_status,omitempty"`    // How the child gets home
	Bus             *bool   `json:"bus,omitempty"`              // Administrative permission flag (Buskind)
	Sick            *bool   `json:"sick,omitempty"`             // true = currently sick
}

// Bind validates the update student request
func (req *UpdateStudentRequest) Bind(_ *http.Request) error {
	// All fields are optional for updates, but validate if provided
	if req.FirstName != nil && *req.FirstName == "" {
		return errors.New("first name cannot be empty")
	}
	if req.LastName != nil && *req.LastName == "" {
		return errors.New("last name cannot be empty")
	}
	if req.SchoolClass != nil && *req.SchoolClass == "" {
		return errors.New("school class cannot be empty")
	}
	// Guardian fields are deprecated - allow empty strings for clearing
	// Empty strings will be converted to nil in the update handler
	return nil
}
