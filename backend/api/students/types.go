package students

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Constants for date formats
const (
	dateFormatYYYYMMDD = "2006-01-02"
)

// StudentResponse represents a student response
type StudentResponse struct {
	ID                 int64            `json:"id"`
	PersonID           int64            `json:"person_id"`
	FirstName          string           `json:"first_name"`
	LastName           string           `json:"last_name"`
	TagID              string           `json:"tag_id,omitempty"`
	Birthday           string           `json:"birthday,omitempty"` // Date in YYYY-MM-DD format
	SchoolClass        string           `json:"school_class"`
	Location           string           `json:"current_location"`
	LocationSince      *time.Time       `json:"location_since,omitempty"`     // When student entered current location
	RoomColor          *string          `json:"current_room_color,omitempty"` // Hex of the current room when set; nil for status-only locations or rooms without override
	GuardianName       string           `json:"guardian_name,omitempty"`
	GuardianContact    string           `json:"guardian_contact,omitempty"`
	GuardianEmail      string           `json:"guardian_email,omitempty"`
	GuardianPhone      string           `json:"guardian_phone,omitempty"`
	GroupID            int64            `json:"group_id,omitempty"`
	GroupName          string           `json:"group_name,omitempty"`
	ExtraInfo          string           `json:"extra_info,omitempty"`
	HealthInfo         string           `json:"health_info,omitempty"`
	SupervisorNotes    string           `json:"supervisor_notes,omitempty"`
	PickupStatus       string           `json:"pickup_status,omitempty"`
	PickupDays         users.PickupDays `json:"pickup_days,omitempty"`          // Weekdays on which the child is picked up
	PickupTime         *string          `json:"pickup_time,omitempty"`          // Today's effective pickup time (HH:MM)
	PickupIsException  bool             `json:"pickup_is_exception,omitempty"`  // True if today's pickup time is an exception
	PickupNotes        string           `json:"pickup_notes,omitempty"`         // Exception reason or schedule notes
	ArrivalTime        *string          `json:"arrival_time,omitempty"`         // Today's effective arrival time (HH:MM)
	ArrivalIsException bool             `json:"arrival_is_exception,omitempty"` // True if today's arrival time is an exception
	ArrivalNotes       string           `json:"arrival_notes,omitempty"`        // Exception reason or schedule notes
	ActualArrivalTime  *string          `json:"actual_arrival_time,omitempty"`  // Today's actual arrival time from attendance (HH:MM)
	ActualPickupTime   *string          `json:"actual_pickup_time,omitempty"`   // Today's actual pickup time from attendance (HH:MM)
	Bus                bool             `json:"bus"`
	BusDays            users.BusDays    `json:"bus_days,omitempty"`
	Sick               bool             `json:"sick"`
	SickSince          *time.Time       `json:"sick_since,omitempty"`
	Excused            bool             `json:"excused"`
	ExcusedSince       *time.Time       `json:"excused_since,omitempty"`
	ClassTrip          bool             `json:"class_trip"`
	ClassTripSince     *time.Time       `json:"class_trip_since,omitempty"`
	DayPlanningStatus  string           `json:"day_planning_status,omitempty"`
	DayPlanningReason  string           `json:"day_planning_reason,omitempty"`
	DayPlanningLabel   string           `json:"day_planning_label,omitempty"`

	// Photo (gated by operations.student_photos_enabled). PhotoURL is empty
	// when no photo is set OR when the feature is off — the frontend's Avatar
	// fallback to initials handles both cases without conditional rendering.
	//
	// PhotoConsentGiven is a *bool with omitempty so the JSON layer can
	// distinguish three states the frontend cares about:
	//   - field omitted     → tenant has photos disabled (don't render
	//                         the consent UI at all)
	//   - "..._given": true → consent recorded
	//   - "..._given": false→ consent not given / explicitly withdrawn
	// A plain bool would serialise the zero value `false` even when the
	// feature is off, making "photos disabled" indistinguishable from
	// "consent withdrawn" — the frontend ended up special-casing this in
	// buildDraft(...) just to recover the missing signal.
	PhotoURL            string     `json:"photo_url,omitempty"`
	PhotoConsentGiven   *bool      `json:"photo_consent_given,omitempty"`
	PhotoConsentGivenAt *time.Time `json:"photo_consent_given_at,omitempty"`
	PhotoConsentGivenBy *int64     `json:"photo_consent_given_by,omitempty"`

	// Other consents stamped on the student row by the enrollment
	// decision service from request.consent_flags. Read-only here —
	// updates flow via a new enrollment submission, not the student
	// edit form. NULL = no consent recorded.
	AGBAcceptedAt            *time.Time `json:"agb_accepted_at,omitempty"`
	DataProcessingAcceptedAt *time.Time `json:"data_processing_accepted_at,omitempty"`
	EmailContactAcceptedAt   *time.Time `json:"email_contact_accepted_at,omitempty"`

	HasFullAccess bool      `json:"has_full_access"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
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
	HasFullAccess        bool                `json:"has_full_access"`
	HasWriteAccess       bool                `json:"has_write_access"`
	GroupSupervisors     []SupervisorContact `json:"group_supervisors,omitempty"`
	AttendanceLogEnabled bool                `json:"attendance_log_enabled"`
	FeedbackEnabled      bool                `json:"feedback_enabled"`
}

type StudentStatusDayResponse struct {
	ID         int64      `json:"id"`
	StudentID  int64      `json:"student_id"`
	Date       string     `json:"date"`
	Status     string     `json:"status"`
	Label      string     `json:"label"`
	ReportedAt time.Time  `json:"reported_at"`
	ClearedAt  *time.Time `json:"cleared_at,omitempty"`
	Source     string     `json:"source"`
	// Note carries an optional free-text reason (e.g. a parent sick note).
	Note      *string   `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateStudentStatusDaysRequest struct {
	Status string   `json:"status"`
	Dates  []string `json:"dates"`
	Reason string   `json:"reason,omitempty"` // optional free-text reason stamped on each day
}

type BulkCreateStudentStatusDaysRequest struct {
	StudentIDs []int64 `json:"student_ids"`
	Status     string  `json:"status"`
	From       string  `json:"from"`
	To         string  `json:"to"`
	Reason     string  `json:"reason,omitempty"`
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
	GroupID         *int64            `json:"group_id,omitempty"`
	ExtraInfo       *string           `json:"extra_info,omitempty"`       // Extra information visible to supervisors
	HealthInfo      *string           `json:"health_info,omitempty"`      // Static health and medical information
	SupervisorNotes *string           `json:"supervisor_notes,omitempty"` // Notes from supervisors
	PickupStatus    *string           `json:"pickup_status,omitempty"`    // How the child gets home
	PickupDays      *users.PickupDays `json:"pickup_days,omitempty"`      // Weekdays on which the child is picked up
	Bus             *bool             `json:"bus,omitempty"`              // Administrative permission flag (Buskind)
	BusDays         *users.BusDays    `json:"bus_days,omitempty"`         // Weekdays on which the child is a Buskind

	// Guardians created together with the student in one atomic transaction
	// (guardian_profiles system). Optional and independent of the legacy
	// scalar guardian_* fields above.
	Guardians []GuardianInput `json:"guardians,omitempty"`

	// Weekly recurring arrival/pickup schedules persisted together with the
	// student in the same atomic transaction. Reuse the bulk-update item DTOs
	// so the time format and validation match the post-creation editing
	// endpoints (PUT /students/{id}/{arrival,pickup}-schedules).
	ArrivalSchedules []ArrivalScheduleRequestItem `json:"arrival_schedules,omitempty"`
	PickupSchedules  []PickupScheduleRequest      `json:"pickup_schedules,omitempty"`
}

// GuardianPhoneInput is one phone number for a guardian created alongside a student.
type GuardianPhoneInput struct {
	PhoneNumber string `json:"phone_number"`
	PhoneType   string `json:"phone_type,omitempty"` // mobile, home, work, other
	Label       string `json:"label,omitempty"`
	IsPrimary   bool   `json:"is_primary,omitempty"`
}

// GuardianInput is one guardian (profile + relationship + phone numbers) to be
// created together with a new student. Mirrors the fields managed on the
// student detail page's guardian form.
type GuardianInput struct {
	// GuardianProfileID, when set, links an EXISTING guardian profile to the
	// new student instead of creating a new one (sibling case, issue #1513).
	// When present the profile fields below (name, email, address, phone
	// numbers) are ignored and the existing profile is never mutated — only the
	// relationship flags apply to the new link.
	GuardianProfileID *int64 `json:"guardian_profile_id,omitempty"`

	// Profile
	FirstName              string `json:"first_name"`
	LastName               string `json:"last_name"`
	Email                  string `json:"email,omitempty"`
	AddressStreet          string `json:"address_street,omitempty"`
	AddressCity            string `json:"address_city,omitempty"`
	AddressPostalCode      string `json:"address_postal_code,omitempty"`
	PreferredContactMethod string `json:"preferred_contact_method,omitempty"`
	LanguagePreference     string `json:"language_preference,omitempty"`
	Notes                  string `json:"notes,omitempty"`

	// Relationship to the student
	RelationshipType   string `json:"relationship_type"` // parent, guardian, relative, other
	IsPrimary          bool   `json:"is_primary,omitempty"`
	IsEmergencyContact bool   `json:"is_emergency_contact,omitempty"`
	CanPickup          bool   `json:"can_pickup,omitempty"`
	PickupNotes        string `json:"pickup_notes,omitempty"`
	EmergencyPriority  int    `json:"emergency_priority,omitempty"`

	PhoneNumbers []GuardianPhoneInput `json:"phone_numbers,omitempty"`
}

// UpdateStudentRequest represents a student update request
type UpdateStudentRequest struct {
	// Person details (optional for update)
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Birthday  *string `json:"birthday,omitempty"` // Date in YYYY-MM-DD format
	TagID     *string `json:"tag_id,omitempty"`

	// Student-specific details (optional for update)
	SchoolClass     *string           `json:"school_class,omitempty"`
	GuardianName    *string           `json:"guardian_name,omitempty"`
	GuardianContact *string           `json:"guardian_contact,omitempty"`
	GuardianEmail   *string           `json:"guardian_email,omitempty"`
	GuardianPhone   *string           `json:"guardian_phone,omitempty"`
	GroupID         *int64            `json:"group_id,omitempty"`
	HealthInfo      *string           `json:"health_info,omitempty"`      // Static health and medical information
	SupervisorNotes *string           `json:"supervisor_notes,omitempty"` // Notes from supervisors
	ExtraInfo       *string           `json:"extra_info,omitempty"`       // Extra information visible to supervisors
	PickupStatus    *string           `json:"pickup_status,omitempty"`    // How the child gets home
	PickupDays      *users.PickupDays `json:"pickup_days,omitempty"`      // Weekdays on which the child is picked up
	Bus             *bool             `json:"bus,omitempty"`              // Administrative permission flag (Buskind)
	BusDays         *users.BusDays    `json:"bus_days,omitempty"`         // Weekdays on which the child is a Buskind
	Sick            *bool             `json:"sick,omitempty"`             // true = currently sick
	SickReason      *string           `json:"sick_reason,omitempty"`      // optional free-text reason stamped on today's sick day
	Excused         *bool             `json:"excused,omitempty"`          // true = currently excused (not attending today)

	// PhotoConsentGiven: documented parental photo-consent flag. The handler
	// records who set it and when (photo_consent_given_at/_by columns) on a
	// false→true transition; on true→false the photo file is deleted in the
	// same tenant transaction so consent withdrawal can never leave a stored
	// image behind.
	PhotoConsentGiven *bool `json:"photo_consent_given,omitempty"`
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

	// Validate guardians created alongside the student. Email/phone format and
	// other field-level rules are enforced at the model layer on insert (which
	// rolls back the whole transaction on failure); here we only guard the
	// relationship type, which has no default and is required to link.
	for i := range req.Guardians {
		if strings.TrimSpace(req.Guardians[i].RelationshipType) == "" {
			return errors.New("guardian relationship_type is required")
		}
	}

	// Validate weekly schedules with the same rules as the bulk-update
	// endpoints so the atomic create path cannot persist invalid times.
	if err := validateArrivalScheduleItems(req.ArrivalSchedules); err != nil {
		return err
	}
	if err := validatePickupScheduleItems(req.PickupSchedules); err != nil {
		return err
	}
	if req.BusDays != nil {
		if err := req.BusDays.Validate(); err != nil {
			return err
		}
		normalized := req.BusDays.Normalize()
		req.BusDays = &normalized
	}
	if req.PickupDays != nil {
		if err := req.PickupDays.Validate(); err != nil {
			return err
		}
		normalized := req.PickupDays.Normalize()
		req.PickupDays = &normalized
	}

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
	if req.BusDays != nil {
		if err := req.BusDays.Validate(); err != nil {
			return err
		}
		normalized := req.BusDays.Normalize()
		req.BusDays = &normalized
	}
	if req.PickupDays != nil {
		if err := req.PickupDays.Validate(); err != nil {
			return err
		}
		normalized := req.PickupDays.Normalize()
		req.PickupDays = &normalized
	}
	// Guardian fields are deprecated - allow empty strings for clearing
	// Empty strings will be converted to nil in the update handler
	return nil
}

func (req *CreateStudentStatusDaysRequest) Bind(_ *http.Request) error {
	req.Status = strings.TrimSpace(req.Status)
	if !isValidStudentStatusDayStatus(req.Status) {
		return errors.New("status must be sick, excused, or class_trip")
	}
	if len(req.Dates) == 0 {
		return errors.New("dates are required")
	}

	seen := make(map[string]struct{}, len(req.Dates))
	for i, rawDate := range req.Dates {
		date := strings.TrimSpace(rawDate)
		if date == "" {
			return errors.New("date cannot be empty")
		}
		if _, err := time.Parse(dateFormatYYYYMMDD, date); err != nil {
			return errors.New("invalid date format, expected YYYY-MM-DD")
		}
		if _, ok := seen[date]; ok {
			return errors.New("duplicate dates are not allowed")
		}
		seen[date] = struct{}{}
		req.Dates[i] = date
	}
	return nil
}

func (req *BulkCreateStudentStatusDaysRequest) Bind(_ *http.Request) error {
	req.Status = strings.TrimSpace(req.Status)
	if !isValidStudentStatusDayStatus(req.Status) {
		return errors.New("status must be sick, excused, or class_trip")
	}
	if len(req.StudentIDs) == 0 {
		return errors.New("student_ids are required")
	}
	seen := make(map[int64]struct{}, len(req.StudentIDs))
	for _, id := range req.StudentIDs {
		if id <= 0 {
			return errors.New("student_ids must be positive")
		}
		if _, ok := seen[id]; ok {
			return errors.New("duplicate student_ids are not allowed")
		}
		seen[id] = struct{}{}
	}
	req.From = strings.TrimSpace(req.From)
	req.To = strings.TrimSpace(req.To)
	if req.From == "" || req.To == "" {
		return errors.New("from and to are required")
	}
	if _, err := time.Parse(dateFormatYYYYMMDD, req.From); err != nil {
		return errors.New("invalid from date format, expected YYYY-MM-DD")
	}
	if _, err := time.Parse(dateFormatYYYYMMDD, req.To); err != nil {
		return errors.New("invalid to date format, expected YYYY-MM-DD")
	}
	return nil
}

func isValidStudentStatusDayStatus(status string) bool {
	switch status {
	case active.StudentStatusDaySick, active.StudentStatusDayExcused, active.StudentStatusDayClassTrip:
		return true
	default:
		return false
	}
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
