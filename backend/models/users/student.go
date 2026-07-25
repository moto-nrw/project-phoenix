package users

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// StudentStatus represents the lifecycle status of a student.
// Set on creation by the parent-enrollment flow; transitions are driven by
// the activate-students scheduler tick (pending→active when enrolled_from
// arrives, active→inactive when enrolled_until passes). The "alumnus" value
// is set by the grade-transition graduation flow (soft delete): the row is
// kept so a transition revert can restore the student, but alumni are
// filtered out of all staff-facing read paths and kiosk check-in.
type StudentStatus string

const (
	StudentStatusPending  StudentStatus = "pending"
	StudentStatusActive   StudentStatus = "active"
	StudentStatusInactive StudentStatus = "inactive"
	StudentStatusAlumnus  StudentStatus = "alumnus"
)

// IsAlumnus reports whether the student is a graduated soft-delete. Nil-safe on
// purpose: callers hold students out of unfiltered map lookups (FindByIDs) where
// a missing row is a legitimate nil, and a nil row is not an alumnus — the
// caller's own not-found handling decides what to do with it. Shared so the
// staff-facing gates (review queues, request decisions, roster writes) all spell
// the check the same way (#405).
func (s *Student) IsAlumnus() bool {
	return s != nil && s.Status == StudentStatusAlumnus
}

// MaxDepartureCompanionNoteLen caps the free-text "mit wem" companion note for
// the accompanied departure mode (#1694). The column is TEXT; this bound keeps
// staff and parent free-text from storing an unbounded payload.
const MaxDepartureCompanionNoteLen = 255

// ErrDepartureCompanionNoteRequired is returned by Validate when a day allows
// the accompanied ("Mit anderem Kind") departure mode but no companion note is
// set. Exported as a sentinel so HTTP handlers can map this client-input
// violation to a 400 instead of leaking the model error as a 500 (#1694).
var ErrDepartureCompanionNoteRequired = errors.New("departure_companion_note is required when a day allows the accompanied departure mode")

// Student represents a student in the system
type Student struct {
	base.Model `bun:"schema:users,table:students"`
	base.TenantModel
	PersonID          int64   `bun:"person_id,notnull" json:"person_id"`
	SchoolClass       string  `bun:"school_class,notnull" json:"school_class"`
	GuardianName      *string `bun:"guardian_name" json:"guardian_name,omitempty"`       // Optional: Legacy field, use guardian_profiles instead
	GuardianContact   *string `bun:"guardian_contact" json:"guardian_contact,omitempty"` // Optional: Legacy field, use guardian_profiles instead
	GuardianEmail     *string `bun:"guardian_email" json:"guardian_email,omitempty"`
	GuardianPhone     *string `bun:"guardian_phone" json:"guardian_phone,omitempty"`
	GroupID           *int64  `bun:"group_id" json:"group_id,omitempty"`
	AddressStreet     *string `bun:"address_street" json:"address_street,omitempty"`
	AddressCity       *string `bun:"address_city" json:"address_city,omitempty"`
	AddressPostalCode *string `bun:"address_postal_code" json:"address_postal_code,omitempty"`
	ExtraInfo         *string `bun:"extra_info" json:"extra_info,omitempty"`
	SupervisorNotes   *string `bun:"supervisor_notes" json:"supervisor_notes,omitempty"`
	HealthInfo        *string `bun:"health_info" json:"health_info,omitempty"`
	PickupStatus      *string `bun:"pickup_status" json:"pickup_status,omitempty"`
	// DepartureDays is the single source of truth for how a child leaves each
	// weekday (#1610): alone / bus / pickup. It unifies the formerly independent
	// BusDays (#1582) and PickupDays maps so a day can no longer contradict
	// itself. The repository persists departure_days and derives bus_days,
	// pickup_days and the legacy pickup_status from it; on read it hydrates
	// DepartureDays and populates the derived BusDays/PickupDays fields below
	// for compatibility. The legacy boolean bus column was dropped in migration
	// 1.15.119; the API derives a compatibility bus flag from BusDays.HasAny().
	DepartureDays DepartureDays `bun:"departure_days,type:jsonb,scanonly" json:"departure_days,omitempty"`
	// AllowedDepartureModes is the preferred source of truth for parent
	// enrollment: every allowed way a child may leave per weekday. It can carry
	// multiple values per day (for example bus + pickup).
	AllowedDepartureModes AllowedDepartureModes `bun:"allowed_departure_modes,type:jsonb,scanonly" json:"allowed_departure_modes,omitempty"`
	// DepartureCompanionNote is the free-text "mit wem" detail for the
	// DepartureAccompanied mode: who the child leaves with (sibling, friend,
	// named person). It is scanonly — like the JSON departure columns above —
	// so the generic create/update/select column set never references it; the
	// repository writes it explicitly in persistDepartureDays and hydrates it in
	// hydrateBusDaysForStudents, both behind a departure_companion_note
	// column-existence guard. That keeps create/update/select working against the
	// historical schemas (pre-1.15.138, and post-rollback, which drops the
	// column) that the migration tests exercise. It may later be formalized into
	// a departure group (#1694).
	DepartureCompanionNote *string `bun:"departure_companion_note,scanonly" json:"departure_companion_note,omitempty"`
	// DepartureCompanionDays is NOT persisted. It carries the weekday keys on
	// which the child has (or is about to have) a companion link in
	// users.student_companions, which answers the "mit wem" question better than
	// the free-text note and therefore satisfies the same Validate requirement —
	// but only for the days it actually covers. A per-DAY set rather than a
	// single flag on purpose: a Monday link says nothing about who the child
	// leaves with on Tuesday. Kept off the table on purpose: the links are the
	// source of truth, and a denormalized copy would drift the moment one is
	// removed.
	DepartureCompanionDays map[string]bool `bun:"-" json:"-"`
	// DepartureBaseline is NOT persisted. Reads that hydrate the departure plan
	// record the plan they loaded here, so a later Update can tell a plan the
	// caller INTENTIONALLY changed from one that merely rode along on a hydrated
	// read. Without that distinction a caller which loaded the child before an
	// unrelated concurrent companion edit committed (sickness auto-clear, status
	// days, imports — none of which touch the plan) would re-persist its stale
	// copy on top of the committed edit, reverting the plan or tripping the
	// stranding check. nil means "not hydrated": no rebase is possible and the
	// supplied fields are taken at face value, exactly as before (#1694).
	DepartureBaseline *DeparturePlanSnapshot `bun:"-" json:"-"`
	// PickupDays / BusDays are derived views of DepartureDays kept for consumers
	// (and API response fields) that have not yet migrated to departure_days.
	PickupDays    PickupDays     `bun:"pickup_days,type:jsonb,scanonly" json:"pickup_days,omitempty"` // Weekdays on which the child is picked up ("wird abgeholt")
	BusDays       BusDays        `bun:"bus_days,type:jsonb,scanonly" json:"bus_days,omitempty"`       // Weekdays on which the child rides the bus
	Sick          *bool          `bun:"sick" json:"sick,omitempty"`                                   // true = currently sick
	SickSince     *time.Time     `bun:"sick_since" json:"sick_since,omitempty"`                       // When sickness was reported
	Excused       *bool          `bun:"excused" json:"excused,omitempty"`                             // true = currently excused (not attending today)
	ExcusedSince  *time.Time     `bun:"excused_since" json:"excused_since,omitempty"`                 // When excused status was reported
	Status        StudentStatus  `bun:"status,notnull,default:'active'" json:"status"`
	EnrolledFrom  *timezone.Date `bun:"enrolled_from,type:date" json:"enrolled_from,omitempty"`
	EnrolledUntil *timezone.Date `bun:"enrolled_until,type:date" json:"enrolled_until,omitempty"`

	// Photo (optional, gated by operations.student_photos_enabled setting +
	// per-student parental consent recorded in photo_consent_given_at).
	// PhotoPath stores the on-disk path; the JSON-facing photo_url is filled
	// by the service layer mapping the path to a tenant-scoped /uploads URL.
	PhotoPath           *string    `bun:"photo_path" json:"-"`
	PhotoConsentGivenAt *time.Time `bun:"photo_consent_given_at" json:"photo_consent_given_at,omitempty"`
	PhotoConsentGivenBy *int64     `bun:"photo_consent_given_by" json:"photo_consent_given_by,omitempty"`

	// Other consents the parent ticked at enrollment time. Populated
	// by the decision service on approval from request.consent_flags.
	// NULL = no consent recorded (parent either declined or the row
	// predates the consent capture). Timestamp = consent given on
	// that date. Mirrors the photo_consent_given_at shape so the
	// student detail page can render all four uniformly.
	AGBAcceptedAt            *time.Time `bun:"agb_accepted_at" json:"agb_accepted_at,omitempty"`
	DataProcessingAcceptedAt *time.Time `bun:"data_processing_accepted_at" json:"data_processing_accepted_at,omitempty"`
	EmailContactAcceptedAt   *time.Time `bun:"email_contact_accepted_at" json:"email_contact_accepted_at,omitempty"`

	// Relations
	Person *Person `bun:"rel:belongs-to,join:person_id=id" json:"person,omitempty"`
	// Group relation is loaded dynamically to avoid import cycle
}

// DeparturePlanSnapshot is a normalized copy of the four departure-plan fields
// as they were read from the database. It is pure data: it records what was
// loaded, it decides nothing.
type DeparturePlanSnapshot struct {
	DepartureDays         DepartureDays
	AllowedDepartureModes AllowedDepartureModes
	BusDays               BusDays
	PickupDays            PickupDays
}

// SnapshotDeparturePlan records the student's current in-memory departure plan
// as the hydrated baseline. Every value is normalized, which also deep-copies
// the maps, so a caller mutating a plan map in place cannot silently move the
// baseline with it.
func (s *Student) SnapshotDeparturePlan() {
	s.DepartureBaseline = &DeparturePlanSnapshot{
		DepartureDays:         s.DepartureDays.Normalize(),
		AllowedDepartureModes: s.AllowedDepartureModes.Normalize(),
		BusDays:               s.BusDays.Normalize(),
		PickupDays:            s.PickupDays.Normalize(),
	}
}

// Validate ensures student data is valid
func (s *Student) Validate() error {
	if s.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	if s.SchoolClass == "" {
		return errors.New("school class is required")
	}

	s.SchoolClass = strings.TrimSpace(s.SchoolClass)

	// Normalize optional legacy guardian fields
	trimPtrString(s.GuardianName)
	trimPtrStringOrNil(&s.GuardianContact)

	// Validate optional contact fields
	if err := validatePtrEmail(s.GuardianEmail, "guardian email"); err != nil {
		return err
	}
	if err := validatePtrPhone(s.GuardianPhone, "guardian phone"); err != nil {
		return err
	}

	trimPtrStringOrNil(&s.AddressStreet)
	trimPtrStringOrNil(&s.AddressCity)
	trimPtrStringOrNil(&s.AddressPostalCode)

	if err := s.DepartureDays.Validate(); err != nil {
		return err
	}

	if err := s.AllowedDepartureModes.Validate(); err != nil {
		return err
	}

	if err := s.BusDays.Validate(); err != nil {
		return err
	}

	if err := s.PickupDays.Validate(); err != nil {
		return err
	}

	// Bound the free-text "mit wem" companion note at the model boundary every
	// write funnels through, so the cap holds even for callers that bypass the
	// API Bind / enrollment-truncation edges (#1694). Trim and drop an empty
	// note to nil so it never persists as "".
	if s.DepartureCompanionNote != nil {
		trimmed := strings.TrimSpace(*s.DepartureCompanionNote)
		switch {
		case trimmed == "":
			s.DepartureCompanionNote = nil
		case utf8.RuneCountInString(trimmed) > MaxDepartureCompanionNoteLen:
			return fmt.Errorf("departure_companion_note must be at most %d characters", MaxDepartureCompanionNoteLen)
		default:
			s.DepartureCompanionNote = &trimmed
		}
	}

	// A "Mit anderem Kind" departure (accompanied mode) is only complete with the
	// free-text "mit wem" companion note — an accompanied plan with no "with whom"
	// detail defeats the point and misleads staff. The React forms make the note
	// required when accompanied is selected; enforce the same coupling here at the
	// model boundary every write funnels through, so callers that bypass the forms
	// (direct student API, imported rows, crafted enrollment payloads) cannot
	// persist an accompanied day with a blank note (#1694). The note block above
	// has already nilled a blank/whitespace note, so a nil pointer here means no
	// usable companion detail. Note: accompanied can only enter via these two
	// fields (the legacy bus/pickup maps cannot express it), so checking them
	// covers every path that introduces it; the reverse coupling (a note with no
	// accompanied day) is handled by note-clearing in the repository, which is the
	// only layer that sees the resolved plan for partial/legacy updates.
	//
	// A structured companion link (users.student_companions) satisfies the same
	// requirement and is strictly better information than typed text, so a
	// caller that has one records its weekdays in DepartureCompanionDays. The
	// cover is checked PER DAY: a child that walks with Mia on Monday still has
	// no answer for a Tuesday the plan also marks accompanied, so a link on one
	// day must not vouch for another. The set defaults to empty, which keeps
	// every path that knows nothing about links (imports, enrollment payloads,
	// direct API writes) on the original note requirement — this fails closed,
	// never open.
	if s.DepartureCompanionNote == nil {
		// Both maps are walked by their own keys rather than through
		// AccompaniedWeekdays, so an accompanied day under a key outside
		// Mon..Fri — which no link can ever cover — still demands the note.
		for day, mode := range s.DepartureDays {
			if mode == DepartureAccompanied && !s.DepartureCompanionDays[day] {
				return ErrDepartureCompanionNoteRequired
			}
		}
		for day, modes := range s.AllowedDepartureModes {
			for _, mode := range modes {
				if mode == DepartureAccompanied && !s.DepartureCompanionDays[day] {
					return ErrDepartureCompanionNoteRequired
				}
			}
		}
	}

	return nil
}

// MarkDepartureCompanionDays records weekdays whose "mit wem" is answered by a
// structured companion link. Purely additive: a caller that knows about an edge
// written later in the same transaction marks it here, and a subsequent probe of
// the STORED edges must never take that cover away again.
func (s *Student) MarkDepartureCompanionDays(days ...string) {
	if len(days) == 0 {
		return
	}
	if s.DepartureCompanionDays == nil {
		s.DepartureCompanionDays = make(map[string]bool, len(days))
	}
	for _, day := range days {
		s.DepartureCompanionDays[day] = true
	}
}

// trimPtrString trims whitespace from a non-nil string pointer
func trimPtrString(s *string) {
	if s != nil && *s != "" {
		*s = strings.TrimSpace(*s)
	}
}

// trimPtrStringOrNil trims whitespace and sets to nil if empty
func trimPtrStringOrNil(sp **string) {
	if *sp == nil || **sp == "" {
		return
	}
	trimmed := strings.TrimSpace(**sp)
	if trimmed == "" {
		*sp = nil
	} else {
		**sp = trimmed
	}
}

// validatePtrEmail validates an optional email pointer
func validatePtrEmail(email *string, fieldName string) error {
	if email == nil || *email == "" {
		return nil
	}
	*email = strings.TrimSpace(*email)
	// Pinned to the shared canonical pattern (email_validation.go) so the
	// rule enforced here at student creation matches enrollment submit-time
	// validation exactly — a value accepted at submit can't be rejected here.
	if !optionalEmailPattern.MatchString(*email) {
		return errors.New("invalid " + fieldName + " format")
	}
	return nil
}

// validatePtrPhone validates an optional phone pointer
func validatePtrPhone(phone *string, fieldName string) error {
	if phone == nil || *phone == "" {
		return nil
	}
	*phone = strings.TrimSpace(*phone)
	// Pinned to the canonical optionalPhonePattern (phone_validation.go)
	// so this student-creation check never diverges from the submit/edit
	// validation in the enrollment service.
	if !optionalPhonePattern.MatchString(*phone) {
		return errors.New("invalid " + fieldName + " format")
	}
	return nil
}

// SetPerson links this student to a person
func (s *Student) SetPerson(person *Person) {
	s.Person = person
	if person != nil {
		s.PersonID = person.ID
	}
}

// StudentWithGroupInfo represents a student with their group information
type StudentWithGroupInfo struct {
	*Student
	GroupName string `json:"group_name"`
}
