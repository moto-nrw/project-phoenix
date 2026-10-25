package presenceprojectionhttp

import (
	time "time"

	departure "github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// StudentResponse represents a student response
type StudentResponse struct {
	ID                 int64                `json:"id"`
	PersonID           int64                `json:"person_id"`
	FirstName          string               `json:"first_name"`
	LastName           string               `json:"last_name"`
	TagID              string               `json:"tag_id,omitempty"`
	Birthday           string               `json:"birthday,omitempty"` // Date in YYYY-MM-DD format
	SchoolClass        string               `json:"school_class"`
	Location           string               `json:"current_location"`
	LocationSince      *time.Time           `json:"location_since,omitempty"`     // When student entered current location
	RoomColor          *string              `json:"current_room_color,omitempty"` // Hex of the current room when set; nil for status-only locations or rooms without override
	GroupID            int64                `json:"group_id,omitempty"`
	GroupName          string               `json:"group_name,omitempty"`
	AddressStreet      string               `json:"address_street,omitempty"`
	AddressCity        string               `json:"address_city,omitempty"`
	AddressPostalCode  string               `json:"address_postal_code,omitempty"`
	ExtraInfo          string               `json:"extra_info,omitempty"`
	HealthInfo         string               `json:"health_info,omitempty"`
	PickupStatus       string               `json:"pickup_status,omitempty"`
	PickupDays         departure.PickupDays `json:"pickup_days,omitempty"`          // Weekdays on which the child is picked up
	PickupTime         *string              `json:"pickup_time,omitempty"`          // Today's effective pickup time (HH:MM)
	PickupIsException  bool                 `json:"pickup_is_exception,omitempty"`  // True if today's pickup time is an exception
	PickupNotes        string               `json:"pickup_notes,omitempty"`         // Exception reason or schedule notes
	ArrivalTime        *string              `json:"arrival_time,omitempty"`         // Today's effective arrival time (HH:MM)
	ArrivalIsException bool                 `json:"arrival_is_exception,omitempty"` // True if today's arrival time is an exception
	ArrivalNotes       string               `json:"arrival_notes,omitempty"`        // Exception reason or schedule notes
	ActualArrivalTime  *string              `json:"actual_arrival_time,omitempty"`  // Today's actual arrival time from attendance (HH:MM)
	ActualPickupTime   *string              `json:"actual_pickup_time,omitempty"`   // Today's actual pickup time from attendance (HH:MM)
	// ActualPickupNote is why the child went home earlier than planned today
	// (#3324). Only the detail view shows it; the list projection drops it.
	ActualPickupNote *string `json:"actual_pickup_note,omitempty"`
	// DepartureDays is the authoritative per-weekday departure mode
	// (alone/bus/pickup). Bus, BusDays and PickupDays are derived from it and
	// kept for backward compatibility with clients not yet on departure_days.
	DepartureDays           departure.DepartureDays         `json:"departure_days,omitempty"`
	AllowedDepartureModes   departure.AllowedDepartureModes `json:"allowed_departure_modes,omitempty"`
	DepartureRuleConfigured bool                            `json:"-"`
	// DayDepartureMode is the Abholart the OGS set or approved for the listed
	// day (#3945). It overrides the weekly plan in the day projections.
	DayDepartureMode string `json:"-"`
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
	DepartureCompanions []departure.CompanionLink `json:"-"`
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
	CompanionsChanged *bool             `json:"companions_changed,omitempty"`
	Bus               bool              `json:"bus"`
	BusDays           departure.BusDays `json:"bus_days,omitempty"`
	Sick              bool              `json:"sick"`
	SickSince         *time.Time        `json:"sick_since,omitempty"`
	Excused           bool              `json:"excused"`
	ExcusedSince      *time.Time        `json:"excused_since,omitempty"`
	ClassTrip         bool              `json:"class_trip"`
	ClassTripSince    *time.Time        `json:"class_trip_since,omitempty"`
	DayPlanningStatus string            `json:"day_planning_status,omitempty"`
	DayPlanningReason string            `json:"day_planning_reason,omitempty"`
	DayPlanningLabel  string            `json:"day_planning_label,omitempty"`
	// PendingExcusedNote is set (#1845) when the child has a parent excused-absence
	// request awaiting office approval that covers the planning day. The child
	// stays "expected" (this does NOT change DayPlanningStatus); the planning
	// views render it as an informational "entschuldigt – Freigabe ausstehend"
	// badge carrying the parent's note so staff can decide it in context.
	PendingExcusedNote *string `json:"pending_excused_note,omitempty"`
	// ClassHandover is set on today's list when the Lehrkraft handed the
	// child over to the OGS (#4794), with whether she marked it "nicht da".
	ClassHandover *ClassHandover `json:"class_handover,omitempty"`

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

	// CareEndsOn is the child's LAST care day (#2487) — the inclusive upper
	// bound of the enrollment interval — as "YYYY-MM-DD", or empty when no end
	// of care is recorded. CareEnded says whether that day has already passed.
	//
	// Both travel on the ordinary student payload because every list has to
	// render the two states differently: a planned exit stays in the list with
	// "Betreuung endet am …", an effective one only appears in the archive
	// view. The exit REASON is NOT here — it is read behind users:delete.
	CareEndsOn string `json:"care_ends_on,omitempty"`
	CareEnded  bool   `json:"care_ended,omitempty"`
	// CareExitRecorded says the end of care was ENTERED by the school
	// ("Betreuung beenden"), as opposed to the interval simply carrying the end
	// of an enrolment phase. Only such an exit can be changed or cancelled in
	// the child management, so the two must not be told apart by how far the
	// date lies ahead. The exit REASON stays behind users:delete.
	CareExitRecorded bool `json:"care_exit_recorded,omitempty"`

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
	HasFullAccess  bool                     `json:"has_full_access"`
	HasWriteAccess bool                     `json:"has_write_access"`
	Consents       []StudentConsentResponse `json:"consents,omitempty"`
	// HasAbsenceWriteAccess retains the class-trip write gate. Sick and excused
	// actions additionally require HasSickExcusedWriteAccess. Neither flag
	// grants Stammdaten writes or authority to review a parent request.
	HasAbsenceWriteAccess     bool                `json:"has_absence_write_access"`
	HasSickExcusedWriteAccess bool                `json:"has_sick_excused_write_access"`
	GroupSupervisors          []SupervisorContact `json:"group_supervisors,omitempty"`
	AttendanceLogEnabled      bool                `json:"attendance_log_enabled"`
	FeedbackEnabled           bool                `json:"feedback_enabled"`
}

type StudentConsentResponse struct {
	Key       string     `json:"key"`
	State     string     `json:"state"`
	ChangedAt *time.Time `json:"changed_at,omitempty"`
}
