package guardians

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/models/users"
)

// GuardianResponse represents a guardian profile response
type GuardianResponse struct {
	ID                     int64   `json:"id"`
	FirstName              string  `json:"first_name"`
	LastName               string  `json:"last_name"`
	Email                  *string `json:"email,omitempty"`
	Phone                  *string `json:"phone,omitempty"`
	MobilePhone            *string `json:"mobile_phone,omitempty"`
	AddressStreet          *string `json:"address_street,omitempty"`
	AddressCity            *string `json:"address_city,omitempty"`
	AddressPostalCode      *string `json:"address_postal_code,omitempty"`
	PreferredContactMethod string  `json:"preferred_contact_method"`
	LanguagePreference     string  `json:"language_preference"`
	Occupation             *string `json:"occupation,omitempty"`
	Employer               *string `json:"employer,omitempty"`
	Notes                  *string `json:"notes,omitempty"`
	HasAccount             bool    `json:"has_account"`
	AccountID              *int64  `json:"account_id,omitempty"`
}

// GuardianCreateRequest represents a request to create a new guardian
type GuardianCreateRequest struct {
	FirstName              string  `json:"first_name"`
	LastName               string  `json:"last_name"`
	Email                  *string `json:"email,omitempty"`
	Phone                  *string `json:"phone,omitempty"`
	MobilePhone            *string `json:"mobile_phone,omitempty"`
	AddressStreet          *string `json:"address_street,omitempty"`
	AddressCity            *string `json:"address_city,omitempty"`
	AddressPostalCode      *string `json:"address_postal_code,omitempty"`
	PreferredContactMethod string  `json:"preferred_contact_method"`
	LanguagePreference     string  `json:"language_preference"`
	Occupation             *string `json:"occupation,omitempty"`
	Employer               *string `json:"employer,omitempty"`
	Notes                  *string `json:"notes,omitempty"`
}

// GuardianUpdateRequest represents a request to update a guardian
type GuardianUpdateRequest struct {
	FirstName              *string `json:"first_name,omitempty"`
	LastName               *string `json:"last_name,omitempty"`
	Email                  *string `json:"email,omitempty"`
	Phone                  *string `json:"phone,omitempty"`
	MobilePhone            *string `json:"mobile_phone,omitempty"`
	AddressStreet          *string `json:"address_street,omitempty"`
	AddressCity            *string `json:"address_city,omitempty"`
	AddressPostalCode      *string `json:"address_postal_code,omitempty"`
	PreferredContactMethod *string `json:"preferred_contact_method,omitempty"`
	LanguagePreference     *string `json:"language_preference,omitempty"`
	Occupation             *string `json:"occupation,omitempty"`
	Employer               *string `json:"employer,omitempty"`
	Notes                  *string `json:"notes,omitempty"`
}

// StudentGuardianLinkRequest represents a request to link a guardian to a student
type StudentGuardianLinkRequest struct {
	GuardianProfileID  int64   `json:"guardian_profile_id"`
	RelationshipType   string  `json:"relationship_type"`
	IsPrimary          bool    `json:"is_primary"`
	IsEmergencyContact bool    `json:"is_emergency_contact"`
	CanPickup          bool    `json:"can_pickup"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  int     `json:"emergency_priority"`
}

// StudentGuardianUpdateRequest represents a request to update a student-guardian relationship
type StudentGuardianUpdateRequest struct {
	RelationshipType   *string `json:"relationship_type,omitempty"`
	IsPrimary          *bool   `json:"is_primary,omitempty"`
	IsEmergencyContact *bool   `json:"is_emergency_contact,omitempty"`
	CanPickup          *bool   `json:"can_pickup,omitempty"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  *int    `json:"emergency_priority,omitempty"`
}

// GuardianWithStudentsResponse represents a guardian with their students
type GuardianWithStudentsResponse struct {
	Guardian *GuardianResponse          `json:"guardian"`
	Students []*StudentWithRelationship `json:"students"`
}

// StudentWithRelationship represents a student with guardian relationship details
type StudentWithRelationship struct {
	StudentID          int64   `json:"student_id"`
	FirstName          string  `json:"first_name"`
	LastName           string  `json:"last_name"`
	SchoolClass        string  `json:"school_class"`
	RelationshipID     int64   `json:"relationship_id"`
	RelationshipType   string  `json:"relationship_type"`
	IsPrimary          bool    `json:"is_primary"`
	IsEmergencyContact bool    `json:"is_emergency_contact"`
	CanPickup          bool    `json:"can_pickup"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  int     `json:"emergency_priority"`
}

// GuardianWithRelationship represents a guardian with student relationship details
type GuardianWithRelationship struct {
	Guardian           *GuardianResponse `json:"guardian"`
	RelationshipID     int64             `json:"relationship_id"`
	RelationshipType   string            `json:"relationship_type"`
	IsPrimary          bool              `json:"is_primary"`
	IsEmergencyContact bool              `json:"is_emergency_contact"`
	CanPickup          bool              `json:"can_pickup"`
	PickupNotes        *string           `json:"pickup_notes,omitempty"`
	EmergencyPriority  int               `json:"emergency_priority"`
}

// Bind validates the guardian create request
func (req *GuardianCreateRequest) Bind(_ *http.Request) error {
	if req.FirstName == "" {
		return errors.New("first_name is required")
	}
	if req.LastName == "" {
		return errors.New("last_name is required")
	}
	// At least one contact method is required
	if (req.Email == nil || *req.Email == "") &&
		(req.Phone == nil || *req.Phone == "") &&
		(req.MobilePhone == nil || *req.MobilePhone == "") {
		return errors.New("at least one contact method (email, phone, or mobile_phone) is required")
	}
	return nil
}

// Bind validates the guardian update request
func (req *GuardianUpdateRequest) Bind(_ *http.Request) error {
	if req.FirstName != nil && *req.FirstName == "" {
		return errors.New("first_name cannot be empty")
	}
	if req.LastName != nil && *req.LastName == "" {
		return errors.New("last_name cannot be empty")
	}
	return nil
}

// Bind validates the student-guardian link request
func (req *StudentGuardianLinkRequest) Bind(_ *http.Request) error {
	if req.GuardianProfileID == 0 {
		return errors.New("guardian_profile_id is required")
	}
	if req.RelationshipType == "" {
		return errors.New("relationship_type is required")
	}
	if req.EmergencyPriority < 1 {
		return errors.New("emergency_priority must be at least 1")
	}
	return nil
}

// Bind validates the student-guardian update request
func (req *StudentGuardianUpdateRequest) Bind(_ *http.Request) error {
	// All fields are optional for update
	return nil
}

// newGuardianResponse converts a guardian profile model to a response
func newGuardianResponse(profile *users.GuardianProfile) *GuardianResponse {
	return &GuardianResponse{
		ID:                     profile.ID,
		FirstName:              profile.FirstName,
		LastName:               profile.LastName,
		Email:                  profile.Email,
		Phone:                  profile.Phone,
		MobilePhone:            profile.MobilePhone,
		AddressStreet:          profile.AddressStreet,
		AddressCity:            profile.AddressCity,
		AddressPostalCode:      profile.AddressPostalCode,
		PreferredContactMethod: profile.PreferredContactMethod,
		LanguagePreference:     profile.LanguagePreference,
		Occupation:             profile.Occupation,
		Employer:               profile.Employer,
		Notes:                  profile.Notes,
		HasAccount:             profile.HasAccount,
		AccountID:              profile.AccountID,
	}
}
