package careplanhttp

import (
	errors "errors"
	fmt "fmt"
	http "net/http"
	reflect "reflect"
	strings "strings"
	utf8 "unicode/utf8"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	departure "github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// validateDepartureCompanionNote bounds the free-text "mit wem" companion note
// length server-side. The column is TEXT and both the staff and enrollment
// inputs are user free-text, so without this an unbounded payload could be
// stored and echoed back in the student API/exports (#1694).
func validateDepartureCompanionNote(note *string) error {
	if note != nil && utf8.RuneCountInString(*note) > departure.MaxDepartureCompanionNoteLen {
		return fmt.Errorf("departure_companion_note must be at most %d characters", departure.MaxDepartureCompanionNoteLen)
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
func departureModesAllowAccompanied(allowed *departure.AllowedDepartureModes, days *departure.DepartureDays) bool {
	if allowed != nil {
		return allowed.HasMode(departure.DepartureAccompanied)
	}
	if days != nil {
		return days.HasMode(departure.DepartureAccompanied)
	}
	return false
}

// isBlankCompanionNote reports whether the companion note is missing or
// whitespace-only.
func isBlankCompanionNote(note *string) bool {
	return note == nil || strings.TrimSpace(*note) == ""
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

	// Optional fields
	GroupID           *int64  `json:"group_id,omitempty"`
	AddressStreet     string  `json:"address_street,omitempty"`
	AddressCity       string  `json:"address_city,omitempty"`
	AddressPostalCode string  `json:"address_postal_code,omitempty"`
	ExtraInfo         *string `json:"extra_info,omitempty"`  // Extra information visible to supervisors
	HealthInfo        *string `json:"health_info,omitempty"` // Static health and medical information
	// DepartureDays is the authoritative per-weekday departure mode
	// (alone/bus/pickup). When provided it supersedes the legacy PickupStatus /
	// PickupDays / Bus / BusDays inputs below, which remain accepted for clients
	// not yet migrated.
	DepartureDays         *departure.DepartureDays         `json:"departure_days,omitempty"`
	AllowedDepartureModes *departure.AllowedDepartureModes `json:"allowed_departure_modes,omitempty"`
	// DepartureCompanionNote is the free-text "mit wem" for the accompanied
	// departure mode (#1694).
	DepartureCompanionNote *string               `json:"departure_companion_note,omitempty"`
	PickupStatus           *string               `json:"pickup_status,omitempty"` // How the child gets home (legacy)
	PickupDays             *departure.PickupDays `json:"pickup_days,omitempty"`   // Weekdays on which the child is picked up (legacy)
	Bus                    *bool                 `json:"bus,omitempty"`           // Administrative permission flag (Buskind, legacy)
	BusDays                *departure.BusDays    `json:"bus_days,omitempty"`      // Weekdays on which the child is a Buskind (legacy)

	// Guardians created together with the student in one atomic transaction
	// (guardian_profiles system). Optional and independent of the legacy
	// scalar guardian_* fields above.
	Guardians []peopleModule.NewStudentGuardian `json:"guardians,omitempty"`

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
	GroupID           *int64  `json:"group_id,omitempty"`
	AddressStreet     *string `json:"address_street,omitempty"`
	AddressCity       *string `json:"address_city,omitempty"`
	AddressPostalCode *string `json:"address_postal_code,omitempty"`
	HealthInfo        *string `json:"health_info,omitempty"` // Static health and medical information
	ExtraInfo         *string `json:"extra_info,omitempty"`  // Extra information visible to supervisors
	// AllowedDepartureModes supersedes the legacy exclusive departure/pickup/bus
	// inputs below when provided.
	AllowedDepartureModes *departure.AllowedDepartureModes `json:"allowed_departure_modes,omitempty"`
	DepartureDays         *departure.DepartureDays         `json:"departure_days,omitempty"`
	// DepartureCompanionNote is the free-text "mit wem" for the accompanied
	// departure mode (#1694). A pointer so omitting it leaves the stored note
	// untouched on update.
	DepartureCompanionNote *string               `json:"departure_companion_note,omitempty"`
	PickupStatus           *string               `json:"pickup_status,omitempty"` // How the child gets home (legacy)
	PickupDays             *departure.PickupDays `json:"pickup_days,omitempty"`   // Weekdays on which the child is picked up (legacy)
	Bus                    *bool                 `json:"bus,omitempty"`           // Administrative permission flag (Buskind, legacy)
	BusDays                *departure.BusDays    `json:"bus_days,omitempty"`      // Weekdays on which the child is a Buskind (legacy)
	Sick                   *bool                 `json:"sick,omitempty"`          // true = currently sick
	SickReason             *string               `json:"sick_reason,omitempty"`   // optional free-text reason stamped on today's sick day
	Excused                *bool                 `json:"excused,omitempty"`       // true = currently excused (not attending today)

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
	// before it built Companions (models/departure.CompanionLinksFingerprint,
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

// validateGuardianRelationships guards that every guardian created alongside the
// student carries a relationship type. Email/phone format and other field-level
// rules are enforced at the model layer on insert (which rolls back the whole
// transaction on failure); the relationship type has no default and is required
// to link.
func validateGuardianRelationships(guardians []peopleModule.NewStudentGuardian) error {
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
	// enforced one layer down as a model invariant (Student.Validate), so
	// relaxing it here alone would just move the 400 later. Whether a "läuft
	// mit" link should replace the free-text note is a business decision that
	// belongs with the rule's owner, not a side effect of this feature.
	if departureModesAllowAccompanied(req.AllowedDepartureModes, req.DepartureDays) &&
		isBlankCompanionNote(req.DepartureCompanionNote) {
		return departure.ErrDepartureCompanionNoteRequired
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
