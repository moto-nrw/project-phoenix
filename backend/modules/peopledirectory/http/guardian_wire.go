package users

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// PhoneNumberResponse is one guardian phone number on the wire.
type PhoneNumberResponse struct {
	ID          int64   `json:"id"`
	PhoneNumber string  `json:"phone_number"`
	PhoneType   string  `json:"phone_type"`
	Label       *string `json:"label,omitempty"`
	IsPrimary   bool    `json:"is_primary"`
	Priority    int     `json:"priority"`
}

// GuardianResponse is the full guardian profile.
type GuardianResponse struct {
	ID                     int64                  `json:"id"`
	FirstName              string                 `json:"first_name"`
	LastName               string                 `json:"last_name"`
	Email                  *string                `json:"email,omitempty"`
	PhoneNumbers           []*PhoneNumberResponse `json:"phone_numbers,omitempty"`
	AddressStreet          *string                `json:"address_street,omitempty"`
	AddressCity            *string                `json:"address_city,omitempty"`
	AddressPostalCode      *string                `json:"address_postal_code,omitempty"`
	PreferredContactMethod string                 `json:"preferred_contact_method"`
	LanguagePreference     string                 `json:"language_preference"`
	Notes                  *string                `json:"notes,omitempty"`
	HasAccount             bool                   `json:"has_account"`
	AccountID              *int64                 `json:"account_id,omitempty"`
}

// GuardianPickerResponse is the minimal, GDPR-safe projection of the picker
// search (#1513): only a COUNT of linked children, never their names, and
// none of the address, notes or preference fields.
type GuardianPickerResponse struct {
	ID                  int64   `json:"id"`
	FirstName           string  `json:"first_name"`
	LastName            string  `json:"last_name"`
	Email               *string `json:"email,omitempty"`
	LinkedChildrenCount int     `json:"linked_children_count"`
}

// GuardianCreateRequest creates a profile; phone numbers are added through
// the phone-number routes. Names are optional (imports may carry only a
// relationship type).
type GuardianCreateRequest struct {
	FirstName              string  `json:"first_name"`
	LastName               string  `json:"last_name"`
	Email                  *string `json:"email,omitempty"`
	AddressStreet          *string `json:"address_street,omitempty"`
	AddressCity            *string `json:"address_city,omitempty"`
	AddressPostalCode      *string `json:"address_postal_code,omitempty"`
	PreferredContactMethod string  `json:"preferred_contact_method"`
	LanguagePreference     string  `json:"language_preference"`
	Notes                  *string `json:"notes,omitempty"`
}

func (req *GuardianCreateRequest) Bind(_ *http.Request) error { return nil }

// GuardianUpdateRequest is a partial update; nil keeps the stored value.
type GuardianUpdateRequest struct {
	FirstName              *string `json:"first_name,omitempty"`
	LastName               *string `json:"last_name,omitempty"`
	Email                  *string `json:"email,omitempty"`
	AddressStreet          *string `json:"address_street,omitempty"`
	AddressCity            *string `json:"address_city,omitempty"`
	AddressPostalCode      *string `json:"address_postal_code,omitempty"`
	PreferredContactMethod *string `json:"preferred_contact_method,omitempty"`
	LanguagePreference     *string `json:"language_preference,omitempty"`
	Notes                  *string `json:"notes,omitempty"`
}

func (req *GuardianUpdateRequest) Bind(_ *http.Request) error { return nil }

// StudentGuardianLinkRequest links an existing guardian to a child.
type StudentGuardianLinkRequest struct {
	GuardianProfileID  int64   `json:"guardian_profile_id"`
	RelationshipType   string  `json:"relationship_type"`
	GuardianRole       string  `json:"guardian_role,omitempty"`
	IsPrimary          bool    `json:"is_primary"`
	IsEmergencyContact bool    `json:"is_emergency_contact"`
	CanPickup          bool    `json:"can_pickup"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  int     `json:"emergency_priority"`
}

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

// StudentGuardianUpdateRequest changes the relationship flags; every field
// is optional.
type StudentGuardianUpdateRequest struct {
	RelationshipType   *string `json:"relationship_type,omitempty"`
	GuardianRole       *string `json:"guardian_role,omitempty"`
	IsPrimary          *bool   `json:"is_primary,omitempty"`
	IsEmergencyContact *bool   `json:"is_emergency_contact,omitempty"`
	CanPickup          *bool   `json:"can_pickup,omitempty"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  *int    `json:"emergency_priority,omitempty"`
}

func (req *StudentGuardianUpdateRequest) Bind(_ *http.Request) error { return nil }

// GuardianLinkResponse is one student-guardian relationship on the wire. It keeps
// the field names of the persisted row the link endpoint used to echo.
type GuardianLinkResponse struct {
	ID                 int64           `json:"id"`
	TenantID           int64           `json:"tenant_id"`
	StudentID          int64           `json:"student_id"`
	GuardianProfileID  int64           `json:"guardian_profile_id"`
	RelationshipType   string          `json:"relationship_type"`
	GuardianRole       string          `json:"guardian_role"`
	IsPrimary          bool            `json:"is_primary"`
	IsEmergencyContact bool            `json:"is_emergency_contact"`
	CanPickup          bool            `json:"can_pickup"`
	PickupNotes        *string         `json:"pickup_notes,omitempty"`
	EmergencyPriority  int             `json:"emergency_priority"`
	IsPayer            bool            `json:"is_payer"`
	Permissions        map[string]bool `json:"permissions,omitempty"`
}

// PhoneNumberCreateRequest adds a phone number; the type defaults to mobile.
type PhoneNumberCreateRequest struct {
	PhoneNumber string  `json:"phone_number"`
	PhoneType   string  `json:"phone_type"`
	Label       *string `json:"label,omitempty"`
	IsPrimary   bool    `json:"is_primary"`
}

func (req *PhoneNumberCreateRequest) Bind(_ *http.Request) error {
	if req.PhoneNumber == "" {
		return errors.New("phone_number is required")
	}
	if req.PhoneType != "" && !validPhoneTypes[req.PhoneType] {
		return errors.New("phone_type must be one of: mobile, home, work, other")
	}
	if req.PhoneType == "" {
		req.PhoneType = "mobile"
	}
	return nil
}

// PhoneNumberUpdateRequest changes a phone number; every field is optional.
type PhoneNumberUpdateRequest struct {
	PhoneNumber *string `json:"phone_number,omitempty"`
	PhoneType   *string `json:"phone_type,omitempty"`
	Label       *string `json:"label,omitempty"`
	IsPrimary   *bool   `json:"is_primary,omitempty"`
	Priority    *int    `json:"priority,omitempty"`
}

func (req *PhoneNumberUpdateRequest) Bind(_ *http.Request) error {
	if req.PhoneNumber != nil && *req.PhoneNumber == "" {
		return errors.New("phone_number cannot be empty")
	}
	if req.PhoneType != nil && !validPhoneTypes[*req.PhoneType] {
		return errors.New("phone_type must be one of: mobile, home, work, other")
	}
	return nil
}

// StudentWithRelationship is one child of a guardian.
type StudentWithRelationship struct {
	StudentID          int64   `json:"student_id"`
	FirstName          string  `json:"first_name"`
	LastName           string  `json:"last_name"`
	SchoolClass        string  `json:"school_class"`
	RelationshipID     int64   `json:"relationship_id"`
	RelationshipType   string  `json:"relationship_type"`
	GuardianRole       string  `json:"guardian_role"`
	IsPrimary          bool    `json:"is_primary"`
	IsEmergencyContact bool    `json:"is_emergency_contact"`
	CanPickup          bool    `json:"can_pickup"`
	PickupNotes        *string `json:"pickup_notes,omitempty"`
	EmergencyPriority  int     `json:"emergency_priority"`
}

// GuardianWithRelationship is one guardian of a child. IsPayer is only
// shown to callers with the financial permission; AccountStatus is the
// portal-access state for the staff tab: active, active_no_access, pending
// or none.
type GuardianWithRelationship struct {
	Guardian           *GuardianResponse `json:"guardian"`
	RelationshipID     int64             `json:"relationship_id"`
	RelationshipType   string            `json:"relationship_type"`
	GuardianRole       string            `json:"guardian_role"`
	IsPrimary          bool              `json:"is_primary"`
	IsEmergencyContact bool              `json:"is_emergency_contact"`
	CanPickup          bool              `json:"can_pickup"`
	PickupNotes        *string           `json:"pickup_notes,omitempty"`
	// PickupWeekdays limits can_pickup to these weekdays (1 = Monday …
	// 5 = Friday); absent means every day (#4780).
	PickupWeekdays    []int  `json:"pickup_weekdays,omitempty"`
	EmergencyPriority int    `json:"emergency_priority"`
	IsPayer           bool   `json:"is_payer"`
	AccountStatus     string `json:"account_status"`
}

// guardianAccountStatus derives the staff-facing account status. An open
// invitation outranks "active_no_access": a pending role-upgrade request
// must read as pending, not as an actionable no-access state (#2172).
func guardianAccountStatus(hasAccount, hasPortalAccess, invitationPending bool) string {
	switch {
	case hasAccount && hasPortalAccess:
		return "active"
	case invitationPending:
		return "pending"
	case hasAccount:
		return "active_no_access"
	default:
		return "none"
	}
}

// CreateStudentGuardiansRequest is the body of POST /students/{studentId}/guardians/batch.
type CreateStudentGuardiansRequest struct {
	Guardians []peopledirectory.NewStudentGuardian `json:"guardians"`
}

func (req *CreateStudentGuardiansRequest) Bind(_ *http.Request) error {
	if len(req.Guardians) == 0 {
		return errors.New("at least one guardian is required")
	}
	return nil
}

// GuardianDeletePreview backs GET /guardians/{id}/delete-preview. Link IDs
// travel as strings so JavaScript never rounds them.
type GuardianDeletePreview struct {
	LinkedCount     int      `json:"linked_count"`
	AffectedNames   []string `json:"affected_names"`
	AffectedLinkIDs []string `json:"affected_link_ids"`
	Warning         string   `json:"warning"`
}

func newGuardianResponse(guardian peopledirectory.Guardian) *GuardianResponse {
	response := &GuardianResponse{
		ID: guardian.ID, FirstName: guardian.FirstName, LastName: guardian.LastName, Email: guardian.Email,
		AddressStreet: guardian.AddressStreet, AddressCity: guardian.AddressCity, AddressPostalCode: guardian.AddressPostalCode,
		PreferredContactMethod: guardian.PreferredContactMethod, LanguagePreference: guardian.LanguagePreference,
		Notes: guardian.Notes, HasAccount: guardian.HasAccount, AccountID: guardian.AccountID,
	}
	if len(guardian.PhoneNumbers) > 0 {
		response.PhoneNumbers = make([]*PhoneNumberResponse, 0, len(guardian.PhoneNumbers))
		for _, phone := range guardian.PhoneNumbers {
			response.PhoneNumbers = append(response.PhoneNumbers, newPhoneNumberResponse(phone))
		}
	}
	return response
}

func newGuardianResponses(guardians []peopledirectory.Guardian) []*GuardianResponse {
	responses := make([]*GuardianResponse, 0, len(guardians))
	for _, guardian := range guardians {
		responses = append(responses, newGuardianResponse(guardian))
	}
	return responses
}

func newPhoneNumberResponse(phone peopledirectory.GuardianPhone) *PhoneNumberResponse {
	return &PhoneNumberResponse{
		ID: phone.ID, PhoneNumber: phone.PhoneNumber, PhoneType: phone.PhoneType,
		Label: phone.Label, IsPrimary: phone.IsPrimary, Priority: phone.Priority,
	}
}

func newGuardianLinkResponse(link peopledirectory.GuardianLink) *GuardianLinkResponse {
	response := &GuardianLinkResponse{
		ID: link.ID, TenantID: link.TenantID, StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID,
		RelationshipType: link.RelationshipType, GuardianRole: link.GuardianRole,
		IsPrimary: link.IsPrimary, IsEmergencyContact: link.IsEmergencyContact, CanPickup: link.CanPickup,
		PickupNotes: link.PickupNotes, EmergencyPriority: link.EmergencyPriority, IsPayer: link.IsPayer,
	}
	if len(link.Permissions) > 0 {
		response.Permissions = make(map[string]bool, len(link.Permissions))
		for _, name := range link.Permissions {
			response.Permissions[name] = true
		}
	}
	return response
}

// --- profile handlers ---

// listGuardians is the admin full-profile list; the picker search lives in
// its own handler with a minimal projection.
