package students

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	guardiansAPI "github.com/moto-nrw/project-phoenix/api/guardians"
	iotDataAPI "github.com/moto-nrw/project-phoenix/api/iot/data"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// validateDepartureCompanionNote bounds the free-text "mit wem" companion note
// length server-side. The column is TEXT and both the staff and enrollment
// inputs are user free-text, so without this an unbounded payload could be
// stored and echoed back in the student API/exports (#1694).
func validateDepartureCompanionNote(note *string) error {
	if note != nil && utf8.RuneCountInString(*note) > users.MaxDepartureCompanionNoteLen {
		return fmt.Errorf("departure_companion_note must be at most %d characters", users.MaxDepartureCompanionNoteLen)
	}
	return nil
}

// departureModesAllowAccompanied reports whether the EFFECTIVE departure plan
// allows the accompanied ("Mit anderem Kind") mode on any weekday. Accompanied
// can only enter via allowed_departure_modes or departure_days — the legacy
// bus/pickup maps cannot express it (#1694).
//
// It mirrors applyDeparturePlan's persistence precedence: allowed_departure_modes
// is authoritative when present and departure_days is ignored; departure_days
// only applies when allowed is absent. Validating the OR of both would reject a
// request whose persisted plan would NOT contain accompanied — e.g. a new-format
// allowed payload (no accompanied) that also echoes a stale legacy departure_days
// carrying accompanied without a companion note.
func departureModesAllowAccompanied(allowed *users.AllowedDepartureModes, days *users.DepartureDays) bool {
	if allowed != nil {
		return allowed.HasMode(users.DepartureAccompanied)
	}
	if days != nil {
		return days.HasMode(users.DepartureAccompanied)
	}
	return false
}

// isBlankCompanionNote reports whether the companion note is missing or
// whitespace-only.
func isBlankCompanionNote(note *string) bool {
	return note == nil || strings.TrimSpace(*note) == ""
}

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
	AddressStreet      string           `json:"address_street,omitempty"`
	AddressCity        string           `json:"address_city,omitempty"`
	AddressPostalCode  string           `json:"address_postal_code,omitempty"`
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
	// DepartureDays is the authoritative per-weekday departure mode
	// (alone/bus/pickup). Bus, BusDays and PickupDays are derived from it and
	// kept for backward compatibility with clients not yet on departure_days.
	DepartureDays           users.DepartureDays         `json:"departure_days,omitempty"`
	AllowedDepartureModes   users.AllowedDepartureModes `json:"allowed_departure_modes,omitempty"`
	DepartureRuleConfigured bool                        `json:"-"`
	// DepartureCompanionNote is the free-text "mit wem" for the accompanied
	// departure mode (#1694).
	DepartureCompanionNote string `json:"departure_companion_note,omitempty"`
	// CompanionStudentIDs lists the other children in this child's
	// Laufgemeinschaft on the requested day (users.student_companions): the whole
	// connected component, not only the direct links, so a page that shows just
	// the ends of a chain still groups them together. Only populated when the
	// caller passes include_companions=true and has full access to the child. The
	// Kindersuche buckets by these ids client-side.
	CompanionStudentIDs []int64 `json:"companion_student_ids,omitempty"`
	// DepartureCompanions are this child's own "läuft mit" links, with the
	// companion's name and weekdays. Server-side only (json:"-"): it exists for
	// the offline lists, which must print WHO the child walks home with — a
	// child whose "mit wem" is answered by links has no free-text note to fall
	// back on, so an export built from the note alone would tell staff nothing
	// but "Mit anderem Kind". Clients read the links from
	// GET /api/students/{id}/companions instead, so this never widens the list
	// payload (or leaks names into a response the caller may not see them in).
	DepartureCompanions []users.CompanionLink `json:"-"`
	// CompanionsChanged is the WRITE's verdict, not a property of the child: it
	// says whether the update that produced this response actually changed the
	// Laufgemeinschaft (a replaced list, a trimmed weekday, or a widened
	// companion plan) — the same condition that decides the
	// student_companions_changed broadcast.
	//
	// It exists because a client cannot tell from its own payload: the forms
	// resubmit the whole departure plan on every save, and reacting to that as a
	// link change costs an open Laufgemeinschaft editor its draft. Set only on
	// the update path (a pointer, so reads keep the response shape they have) and
	// always set there, so an absent key means "this response carries no verdict"
	// rather than "nothing changed".
	CompanionsChanged *bool         `json:"companions_changed,omitempty"`
	Bus               bool          `json:"bus"`
	BusDays           users.BusDays `json:"bus_days,omitempty"`
	Sick              bool          `json:"sick"`
	SickSince         *time.Time    `json:"sick_since,omitempty"`
	Excused           bool          `json:"excused"`
	ExcusedSince      *time.Time    `json:"excused_since,omitempty"`
	ClassTrip         bool          `json:"class_trip"`
	ClassTripSince    *time.Time    `json:"class_trip_since,omitempty"`
	DayPlanningStatus string        `json:"day_planning_status,omitempty"`
	DayPlanningReason string        `json:"day_planning_reason,omitempty"`
	DayPlanningLabel  string        `json:"day_planning_label,omitempty"`
	// PendingExcusedNote is set (#1845) when the child has a parent excused-absence
	// request awaiting office approval that covers the planning day. The child
	// stays "expected" (this does NOT change DayPlanningStatus); the planning
	// views render it as an informational "entschuldigt – Freigabe ausstehend"
	// badge carrying the parent's note so staff can decide it in context.
	PendingExcusedNote *string `json:"pending_excused_note,omitempty"`

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
	Role      string `json:"role"` // "teacher" or "staff"
}

// StudentDetailResponse represents a detailed student response with access control
type StudentDetailResponse struct {
	StudentResponse
	HasFullAccess  bool `json:"has_full_access"`
	HasWriteAccess bool `json:"has_write_access"`
	// HasAbsenceWriteAccess gates the Krankmeldung / Entschuldigung /
	// Klassenfahrt actions specifically. It is a superset of HasWriteAccess: in
	// a school running operations.group_mode = open_care, staff holding
	// users:absence may report absences for children they cannot otherwise edit
	// (#2232). Never use it to show Stammdaten edit affordances.
	HasAbsenceWriteAccess bool                `json:"has_absence_write_access"`
	GroupSupervisors      []SupervisorContact `json:"group_supervisors,omitempty"`
	AttendanceLogEnabled  bool                `json:"attendance_log_enabled"`
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

const maxStudentStatusDayCreateDays = 366

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
	GroupID           *int64  `json:"group_id,omitempty"`
	AddressStreet     string  `json:"address_street,omitempty"`
	AddressCity       string  `json:"address_city,omitempty"`
	AddressPostalCode string  `json:"address_postal_code,omitempty"`
	ExtraInfo         *string `json:"extra_info,omitempty"`       // Extra information visible to supervisors
	HealthInfo        *string `json:"health_info,omitempty"`      // Static health and medical information
	SupervisorNotes   *string `json:"supervisor_notes,omitempty"` // Notes from supervisors
	// DepartureDays is the authoritative per-weekday departure mode
	// (alone/bus/pickup). When provided it supersedes the legacy PickupStatus /
	// PickupDays / Bus / BusDays inputs below, which remain accepted for clients
	// not yet migrated.
	DepartureDays         *users.DepartureDays         `json:"departure_days,omitempty"`
	AllowedDepartureModes *users.AllowedDepartureModes `json:"allowed_departure_modes,omitempty"`
	// DepartureCompanionNote is the free-text "mit wem" for the accompanied
	// departure mode (#1694).
	DepartureCompanionNote *string           `json:"departure_companion_note,omitempty"`
	PickupStatus           *string           `json:"pickup_status,omitempty"` // How the child gets home (legacy)
	PickupDays             *users.PickupDays `json:"pickup_days,omitempty"`   // Weekdays on which the child is picked up (legacy)
	Bus                    *bool             `json:"bus,omitempty"`           // Administrative permission flag (Buskind, legacy)
	BusDays                *users.BusDays    `json:"bus_days,omitempty"`      // Weekdays on which the child is a Buskind (legacy)

	// Guardians created together with the student in one atomic transaction
	// (guardian_profiles system). Optional and independent of the legacy
	// scalar guardian_* fields above.
	Guardians []guardiansAPI.GuardianWithRelationshipInput `json:"guardians,omitempty"`

	// Weekly recurring arrival/pickup schedules persisted together with the
	// student in the same atomic transaction. Reuse the bulk-update item DTOs
	// so the time format and validation match the post-creation editing
	// endpoints (PUT /students/{id}/{arrival,pickup}-schedules).
	ArrivalSchedules []ArrivalScheduleRequestItem `json:"arrival_schedules,omitempty"`
	PickupSchedules  []PickupScheduleRequest      `json:"pickup_schedules,omitempty"`
}

// UpdateStudentRequest represents a student update request
type UpdateStudentRequest struct {
	// Person details (optional for update)
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Birthday  *string `json:"birthday,omitempty"` // Date in YYYY-MM-DD format
	TagID     *string `json:"tag_id,omitempty"`

	// Student-specific details (optional for update)
	SchoolClass       *string `json:"school_class,omitempty"`
	GuardianName      *string `json:"guardian_name,omitempty"`
	GuardianContact   *string `json:"guardian_contact,omitempty"`
	GuardianEmail     *string `json:"guardian_email,omitempty"`
	GuardianPhone     *string `json:"guardian_phone,omitempty"`
	GroupID           *int64  `json:"group_id,omitempty"`
	AddressStreet     *string `json:"address_street,omitempty"`
	AddressCity       *string `json:"address_city,omitempty"`
	AddressPostalCode *string `json:"address_postal_code,omitempty"`
	HealthInfo        *string `json:"health_info,omitempty"`      // Static health and medical information
	SupervisorNotes   *string `json:"supervisor_notes,omitempty"` // Notes from supervisors
	ExtraInfo         *string `json:"extra_info,omitempty"`       // Extra information visible to supervisors
	// AllowedDepartureModes supersedes the legacy exclusive departure/pickup/bus
	// inputs below when provided.
	AllowedDepartureModes *users.AllowedDepartureModes `json:"allowed_departure_modes,omitempty"`
	DepartureDays         *users.DepartureDays         `json:"departure_days,omitempty"`
	// DepartureCompanionNote is the free-text "mit wem" for the accompanied
	// departure mode (#1694). A pointer so omitting it leaves the stored note
	// untouched on update.
	DepartureCompanionNote *string           `json:"departure_companion_note,omitempty"`
	PickupStatus           *string           `json:"pickup_status,omitempty"` // How the child gets home (legacy)
	PickupDays             *users.PickupDays `json:"pickup_days,omitempty"`   // Weekdays on which the child is picked up (legacy)
	Bus                    *bool             `json:"bus,omitempty"`           // Administrative permission flag (Buskind, legacy)
	BusDays                *users.BusDays    `json:"bus_days,omitempty"`      // Weekdays on which the child is a Buskind (legacy)
	Sick                   *bool             `json:"sick,omitempty"`          // true = currently sick
	SickReason             *string           `json:"sick_reason,omitempty"`   // optional free-text reason stamped on today's sick day
	Excused                *bool             `json:"excused,omitempty"`       // true = currently excused (not attending today)

	// PhotoConsentGiven: documented parental photo-consent flag. The handler
	// records who set it and when (photo_consent_given_at/_by columns) on a
	// false→true transition; on true→false the photo file is deleted in the
	// same tenant transaction so consent withdrawal can never leave a stored
	// image behind.
	PhotoConsentGiven *bool `json:"photo_consent_given,omitempty"`
	// Companions ("läuft mit") replaces the child's complete Laufgemeinschaft.
	// It travels with the departure plan on purpose: the link is only legal on a
	// day the plan allows "Anderes Kind", so both have to be validated against
	// each other in ONE request instead of racing as two.
	// nil = leave the links untouched; [] = clear them.
	Companions *[]CompanionEntry `json:"companions,omitempty"`
	// CompanionsFingerprint is the fingerprint of the list the client LOADED
	// before it built Companions (models/users.CompanionLinksFingerprint,
	// mirrored by companionsFingerprint() in the frontend).
	//
	// Because Companions is a complete replacement, two staff members editing
	// the same child from the same snapshot would both submit everything they
	// saw and the second write would delete the first one's committed links.
	// The handler compares this against the stored links while it holds the
	// child's row lock and answers a mismatch with a retriable 409 instead.
	//
	// It is equally required WITHOUT Companions: a departure plan alone removes
	// links too (the backend trims the weekdays the new plan no longer allows),
	// and the forms resubmit their whole plan on every save, so a stale plan
	// deletes an edge somebody else committed just as effectively as a stale list
	// does. A client that writes a plan therefore sends the fingerprint of the
	// links it loaded; omitted means the caller makes no claim about the state it
	// started from, which is refused as soon as the trim would actually drop
	// something.
	CompanionsFingerprint *string `json:"companions_fingerprint,omitempty"`
	// ExtendCompanionPlans confirms widening a companion's own departure plan.
	ExtendCompanionPlans bool `json:"extend_companion_plans,omitempty"`
	// ConfirmedCompanionExtensions is WHAT the user confirmed when they set
	// ExtendCompanionPlans: the companions and weekdays the client showed them,
	// i.e. the conflicts of the 409 (or of the picker's up-front question).
	//
	// The rows are unlocked while the human answers, so the conflict set can grow
	// in the meantime — another admin narrowing a companion's plan adds one. The
	// flag alone would authorize that new conflict too and silently widen a child
	// nobody was asked about, so the write pass only extends what appears here
	// and answers a wider set with a fresh 409.
	ConfirmedCompanionExtensions []CompanionEntry `json:"confirmed_companion_extensions,omitempty"`
}

// RFIDAssignmentRequest is the RFID tag assignment payload, shared with
// the device-auth endpoint in api/iot/data (single declaration site).
type RFIDAssignmentRequest = iotDataAPI.RFIDAssignmentRequest

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

// validateGuardianRelationships guards that every guardian created alongside the
// student carries a relationship type. Email/phone format and other field-level
// rules are enforced at the model layer on insert (which rolls back the whole
// transaction on failure); the relationship type has no default and is required
// to link.
func validateGuardianRelationships(guardians []guardiansAPI.GuardianWithRelationshipInput) error {
	for i := range guardians {
		if strings.TrimSpace(guardians[i].RelationshipType) == "" {
			return errors.New("guardian relationship_type is required")
		}
	}
	return nil
}

// normalizeDayFields validates and normalizes the per-weekday departure/pickup/bus
// maps in place, so downstream persistence always sees canonical values.
func (req *StudentRequest) normalizeDayFields() error {
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
	if req.DepartureDays != nil {
		if err := req.DepartureDays.Validate(); err != nil {
			return err
		}
		normalized := req.DepartureDays.Normalize()
		req.DepartureDays = &normalized
	}
	if req.AllowedDepartureModes != nil {
		if err := req.AllowedDepartureModes.Validate(); err != nil {
			return err
		}
		normalized := req.AllowedDepartureModes.Normalize()
		req.AllowedDepartureModes = &normalized
	}
	return nil
}

// normalizeDayFields validates and normalizes the per-weekday departure/pickup/bus
// maps in place, so downstream persistence always sees canonical values.
func (req *UpdateStudentRequest) normalizeDayFields() error {
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
	if req.DepartureDays != nil {
		if err := req.DepartureDays.Validate(); err != nil {
			return err
		}
		normalized := req.DepartureDays.Normalize()
		req.DepartureDays = &normalized
	}
	if req.AllowedDepartureModes != nil {
		if err := req.AllowedDepartureModes.Validate(); err != nil {
			return err
		}
		normalized := req.AllowedDepartureModes.Normalize()
		req.AllowedDepartureModes = &normalized
	}
	return nil
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

	// Validate guardians created alongside the student.
	if err := validateGuardianRelationships(req.Guardians); err != nil {
		return err
	}

	// Validate weekly schedules with the same rules as the bulk-update
	// endpoints so the atomic create path cannot persist invalid times.
	if err := validateArrivalScheduleItems(req.ArrivalSchedules); err != nil {
		return err
	}
	if err := validatePickupScheduleItems(req.PickupSchedules); err != nil {
		return err
	}
	if err := req.normalizeDayFields(); err != nil {
		return err
	}
	if err := validateDepartureCompanionNote(req.DepartureCompanionNote); err != nil {
		return err
	}
	// A day that allows the accompanied ("Mit anderem Kind") departure mode is
	// only complete with the coupled "mit wem" note. Reject it at the request
	// boundary so a stale or direct API caller gets a 400 rather than the model's
	// later invariant surfacing as a 500 on persist. On create there is no stored
	// note to fall back on, so the request itself must carry it (#1694).
	//
	// NOTE: linked companions do NOT satisfy this requirement. The same rule is
	// enforced one layer down as a model invariant (users.Student.Validate), so
	// relaxing it here alone would just move the 400 later. Whether a "läuft
	// mit" link should replace the free-text note is a business decision that
	// belongs with the rule's owner, not a side effect of this feature.
	if departureModesAllowAccompanied(req.AllowedDepartureModes, req.DepartureDays) &&
		isBlankCompanionNote(req.DepartureCompanionNote) {
		return users.ErrDepartureCompanionNoteRequired
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
	if err := req.normalizeDayFields(); err != nil {
		return err
	}
	if err := validateDepartureCompanionNote(req.DepartureCompanionNote); err != nil {
		return err
	}
	if err := validateCompanionEntries(req.Companions); err != nil {
		return err
	}
	if err := validateCompanionsFingerprint(req.Companions, req.CompanionsFingerprint); err != nil {
		return err
	}
	// Guardian fields are deprecated - allow empty strings for clearing
	// Empty strings will be converted to nil in the update handler
	return nil
}

// absenceOnlyUpdateFields lists the UpdateStudentRequest fields that carry
// nothing but an absence report for today. Everything else on the struct is
// Stammdaten, Betreuungsplanung, or photo consent.
var absenceOnlyUpdateFields = map[string]bool{
	"Sick":       true,
	"SickReason": true,
	"Excused":    true,
}

// isAbsenceOnly reports whether this payload does nothing but report or clear
// today's absence. It is what lets PUT /students/{id} fall back to the
// action-scoped absence gate in a school without fixed groups (#2232) — so it
// has to be exact: one Stammdaten field riding along must make it false.
//
// Deliberately reflection-based rather than a hand-written field list. A new
// field added to UpdateStudentRequest is then automatically treated as "not an
// absence field", instead of silently slipping through the weaker gate until
// somebody remembers to extend a negative list here.
func (req *UpdateStudentRequest) isAbsenceOnly() bool {
	if req == nil {
		return false
	}
	value := reflect.ValueOf(*req)
	structType := value.Type()
	carriesAbsence := false
	for i := range structType.NumField() {
		if value.Field(i).IsZero() {
			continue
		}
		if !absenceOnlyUpdateFields[structType.Field(i).Name] {
			return false
		}
		carriesAbsence = true
	}
	return carriesAbsence
}

// hasCompanionUpdate reports whether the request carries a "läuft mit" list.
// A nil pointer means "leave the links alone"; an empty slice clears them.
func (req *UpdateStudentRequest) hasCompanionUpdate() bool {
	return req.Companions != nil
}

func (req *UpdateStudentRequest) hasDeparturePlanUpdate() bool {
	return req.DepartureDays != nil ||
		req.AllowedDepartureModes != nil ||
		req.PickupStatus != nil ||
		req.PickupDays != nil ||
		req.Bus != nil ||
		req.BusDays != nil
}

// touchesCompanions reports whether this request could have changed the child's
// "läuft mit" links — either by submitting a list, or by writing a departure
// plan, which TRIMS the links on a weekday the new plan no longer allows.
//
// It is a NECESSARY condition, not a sufficient one: the forms resubmit the
// whole departure plan on every save, so a name-only edit answers true here.
// applyCompanionUpdate uses it to skip the before-state read and then decides
// from the actual write whether student_companions_changed is announced — a
// client reacting to that event discards or blocks an in-progress companion
// edit, so an event for an unchanged list costs somebody their work.
//
// The legacy bus/pickup inputs count: they resolve into the same plan, so they
// can take the accompanied mode (and with it the links) away. Everything else —
// a name, an address, a photo, a sick flag — cannot touch a link at all.
func (req *UpdateStudentRequest) touchesCompanions() bool {
	return req.hasCompanionUpdate() ||
		req.DepartureCompanionNote != nil ||
		req.hasDeparturePlanUpdate()
}

func (req *CreateStudentStatusDaysRequest) Bind(_ *http.Request) error {
	req.Status = strings.TrimSpace(req.Status)
	if !isValidStudentStatusDayStatus(req.Status) {
		return errors.New("status must be sick, excused, or class_trip")
	}
	return validateStudentStatusDayDates(req.Dates)
}

func validateStudentStatusDayDates(dates []string) error {
	if len(dates) == 0 {
		return errors.New("dates are required")
	}
	if len(dates) > maxStudentStatusDayCreateDays {
		return errors.New("dates cannot exceed 366 items")
	}

	seen := make(map[string]struct{}, len(dates))
	var earliest, latest timezone.Date
	for i, rawDate := range dates {
		date, parsed, err := normalizeStudentStatusDayDate(rawDate)
		if err != nil {
			return err
		}
		if earliest.IsZero() || parsed.Before(earliest) {
			earliest = parsed
		}
		if latest.IsZero() || parsed.After(latest) {
			latest = parsed
		}
		if _, ok := seen[date]; ok {
			return errors.New("duplicate dates are not allowed")
		}
		seen[date] = struct{}{}
		dates[i] = date
	}
	if earliest.DaysUntil(latest) >= maxStudentStatusDayCreateDays {
		return errors.New("dates cannot span more than 366 days")
	}
	return nil
}

func normalizeStudentStatusDayDate(rawDate string) (string, timezone.Date, error) {
	date := strings.TrimSpace(rawDate)
	if date == "" {
		return "", timezone.Date{}, errors.New("date cannot be empty")
	}
	parsed, err := timezone.ParseDate(date)
	if err != nil {
		return "", timezone.Date{}, errors.New("invalid date format, expected YYYY-MM-DD")
	}
	return date, parsed, nil
}

func (req *BulkCreateStudentStatusDaysRequest) Bind(_ *http.Request) error {
	req.Status = strings.TrimSpace(req.Status)
	if !isValidStudentStatusDayStatus(req.Status) {
		return errors.New("status must be sick, excused, or class_trip")
	}
	if len(req.StudentIDs) == 0 {
		return errors.New("student_ids are required")
	}
	if len(req.StudentIDs) > 500 {
		return errors.New("student_ids cannot exceed 500 items")
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
