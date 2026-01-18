package students

import (
	"errors"
	"net/http"
	"time"
)

// Constants for date formats
const (
	dateFormatYYYYMMDD = "2006-01-02"
)

// StudentResponse represents a student response
type StudentResponse struct {
	ID              int64      `json:"id"`
	PersonID        int64      `json:"person_id"`
	FirstName       string     `json:"first_name"`
	LastName        string     `json:"last_name"`
	TagID           string     `json:"tag_id,omitempty"`
	Birthday        string     `json:"birthday,omitempty"` // Date in YYYY-MM-DD format
	SchoolClass     string     `json:"school_class"`
	Location        string     `json:"current_location"`
	LocationSince   *time.Time `json:"location_since,omitempty"` // When student entered current location
	GuardianName    string     `json:"guardian_name,omitempty"`
	GuardianContact string     `json:"guardian_contact,omitempty"`
	GuardianEmail   string     `json:"guardian_email,omitempty"`
	GuardianPhone   string     `json:"guardian_phone,omitempty"`
	GroupID         int64      `json:"group_id,omitempty"`
	GroupName       string     `json:"group_name,omitempty"`
	ExtraInfo       string     `json:"extra_info,omitempty"`
	HealthInfo      string     `json:"health_info,omitempty"`
	SupervisorNotes string     `json:"supervisor_notes,omitempty"`
	PickupStatus    string     `json:"pickup_status,omitempty"`
	Bus             bool       `json:"bus"`
	Sick            bool       `json:"sick"`
	SickSince       *time.Time `json:"sick_since,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// SupervisorContact represents contact information for a group supervisor
type SupervisorContact struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Role      string `json:"role"` // "teacher" or "staff"
}

// StudentDetailResponse represents a detailed student response with access control
type StudentDetailResponse struct {
	StudentResponse
	HasFullAccess    bool                `json:"has_full_access"`
	GroupSupervisors []SupervisorContact `json:"group_supervisors,omitempty"`
}

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

// RFIDAssignmentRequest represents an RFID tag assignment request
type RFIDAssignmentRequest struct {
	RFIDTag string `json:"rfid_tag"`
}

// RFIDAssignmentResponse represents an RFID tag assignment response
type RFIDAssignmentResponse struct {
	Success     bool    `json:"success"`
	StudentID   int64   `json:"student_id"`
	StudentName string  `json:"student_name"`
	RFIDTag     string  `json:"rfid_tag"`
	PreviousTag *string `json:"previous_tag,omitempty"`
	Message     string  `json:"message"`
}

// PrivacyConsentResponse represents a privacy consent response
type PrivacyConsentResponse struct {
	ID                int64                  `json:"id"`
	StudentID         int64                  `json:"student_id"`
	PolicyVersion     string                 `json:"policy_version"`
	Accepted          bool                   `json:"accepted"`
	AcceptedAt        *time.Time             `json:"accepted_at,omitempty"`
	ExpiresAt         *time.Time             `json:"expires_at,omitempty"`
	DurationDays      *int                   `json:"duration_days,omitempty"`
	RenewalRequired   bool                   `json:"renewal_required"`
	DataRetentionDays int                    `json:"data_retention_days"`
	Details           map[string]interface{} `json:"details,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// PrivacyConsentRequest represents a privacy consent update request
type PrivacyConsentRequest struct {
	PolicyVersion     string                 `json:"policy_version"`
	Accepted          bool                   `json:"accepted"`
	DurationDays      *int                   `json:"duration_days,omitempty"`
	DataRetentionDays int                    `json:"data_retention_days"`
	Details           map[string]interface{} `json:"details,omitempty"`
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

// Bind validates the RFID assignment request
func (req *RFIDAssignmentRequest) Bind(_ *http.Request) error {
	if req.RFIDTag == "" {
		return errors.New("rfid_tag is required")
	}
	if len(req.RFIDTag) < 8 {
		return errors.New("rfid_tag must be at least 8 characters")
	}
	if len(req.RFIDTag) > 64 {
		return errors.New("rfid_tag must be at most 64 characters")
	}
	return nil
}

// Bind validates the privacy consent request
func (req *PrivacyConsentRequest) Bind(_ *http.Request) error {
	if req.PolicyVersion == "" {
		return errors.New("policy version is required")
	}
	if req.DataRetentionDays < 1 || req.DataRetentionDays > 31 {
		return errors.New("data retention days must be between 1 and 31")
	}
	return nil
}
