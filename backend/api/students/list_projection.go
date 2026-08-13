package students

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Values accepted by the `view` query parameter of GET /api/students.
const (
	// StudentListViewFull is the historical full StudentResponse projection and
	// stays the default — every existing consumer keeps its payload unchanged.
	StudentListViewFull = "full"
	// StudentListViewSlim is the Kindersuche projection (#2097).
	StudentListViewSlim = "slim"
)

// SlimStudentResponse is the list projection for the Kindersuche
// (frontend students/search): every field its cards, client-side
// filters/sorting/grouping and the presence badge actually read, and nothing
// else.
//
// Why (#2097): the full StudentResponse ships ~55 fields, including the
// address block, health info, supervisor notes, the four weekday departure
// maps, guardian contacts, enrollment consent timestamps and row timestamps.
// The Kindersuche renders none of them — the child detail page fetches the
// full record separately — yet the page requests up to 1000 rows in one trip,
// which measured 51.9 KB on average and 198.4 KB at peak in production
// (benchmark 2026-07-30, #2063).
//
// This is a WIRE projection only: the handler still builds, filters, sorts and
// paginates full StudentResponse values, so day-planning, administrative
// filters and access redaction behave identically in both views. Only the
// final marshalling differs.
//
// Adding a field here is cheap; removing one is a breaking change for the
// Kindersuche. Before adding, check that the page actually reads it —
// TestListStudents_SlimProjection pins the forbidden set.
type SlimStudentResponse struct {
	ID          int64  `json:"id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`

	// Live location + the badge context it is rendered with. The *_since
	// timestamps are part of StudentLocationContext (frontend
	// lib/location-helper.ts): the cards do not pass showLocationSince today,
	// but the badge reads them, so dropping them would silently break the
	// "seit HH:MM" label the moment a card enables it. They cost <1% of the
	// payload (only set for children who are actually present/absent).
	Location       string     `json:"current_location"`
	LocationSince  *time.Time `json:"location_since,omitempty"`
	RoomColor      *string    `json:"current_room_color,omitempty"`
	GroupID        int64      `json:"group_id,omitempty"`
	GroupName      string     `json:"group_name,omitempty"`
	Sick           bool       `json:"sick"`
	SickSince      *time.Time `json:"sick_since,omitempty"`
	Excused        bool       `json:"excused"`
	ExcusedSince   *time.Time `json:"excused_since,omitempty"`
	ClassTrip      bool       `json:"class_trip"`
	ClassTripSince *time.Time `json:"class_trip_since,omitempty"`

	// Day planning. day_planning_reason is deliberately absent: the page
	// renders day_planning_label and derives everything else from status.
	DayPlanningStatus  string  `json:"day_planning_status,omitempty"`
	DayPlanningLabel   string  `json:"day_planning_label,omitempty"`
	PendingExcusedNote *string `json:"pending_excused_note,omitempty"`

	// Planned + actual times (the arrival/pickup rows and their sort/group
	// modes). Only populated for full-access rows and when the caller passes
	// include_pickup_times / include_arrival_times.
	DepartureModes     []users.DepartureMode `json:"departure_modes,omitempty"`
	DepartureLabel     string                `json:"departure_label,omitempty"`
	PickupTime         *string               `json:"pickup_time,omitempty"`
	PickupIsException  bool                  `json:"pickup_is_exception,omitempty"`
	PickupNotes        string                `json:"pickup_notes,omitempty"`
	ArrivalTime        *string               `json:"arrival_time,omitempty"`
	ArrivalIsException bool                  `json:"arrival_is_exception,omitempty"`
	ArrivalNotes       string                `json:"arrival_notes,omitempty"`
	ActualArrivalTime  *string               `json:"actual_arrival_time,omitempty"`
	ActualPickupTime   *string               `json:"actual_pickup_time,omitempty"`

	// "Nach Laufgemeinschaft" grouping; only with include_companions=true.
	CompanionStudentIDs []int64 `json:"companion_student_ids,omitempty"`

	// Photo + consent. photo_consent_given is carried because the page probes
	// its PRESENCE (not its value) to decide whether the "Fotoerlaubnis" filter
	// is offered at all — omitting it would silently remove that filter.
	PhotoURL          string `json:"photo_url,omitempty"`
	PhotoConsentGiven *bool  `json:"photo_consent_given,omitempty"`

	HasFullAccess bool `json:"has_full_access"`
}

// slimStudentResponses projects the fully built, filtered and paginated list
// onto the Kindersuche wire shape.
func slimStudentResponses(responses []StudentResponse, planningDate timezone.Date) []SlimStudentResponse {
	slim := make([]SlimStudentResponse, len(responses))
	for i := range responses {
		r := &responses[i]
		departure := dailyDepartureForDate(*r, planningDate)
		slim[i] = SlimStudentResponse{
			ID:                  r.ID,
			FirstName:           r.FirstName,
			LastName:            r.LastName,
			SchoolClass:         r.SchoolClass,
			Location:            r.Location,
			LocationSince:       r.LocationSince,
			RoomColor:           r.RoomColor,
			GroupID:             r.GroupID,
			GroupName:           r.GroupName,
			Sick:                r.Sick,
			SickSince:           r.SickSince,
			Excused:             r.Excused,
			ExcusedSince:        r.ExcusedSince,
			ClassTrip:           r.ClassTrip,
			ClassTripSince:      r.ClassTripSince,
			DayPlanningStatus:   r.DayPlanningStatus,
			DayPlanningLabel:    r.DayPlanningLabel,
			PendingExcusedNote:  r.PendingExcusedNote,
			DepartureModes:      departure.Modes,
			DepartureLabel:      departure.LegacyLabel,
			PickupTime:          r.PickupTime,
			PickupIsException:   r.PickupIsException,
			PickupNotes:         r.PickupNotes,
			ArrivalTime:         r.ArrivalTime,
			ArrivalIsException:  r.ArrivalIsException,
			ArrivalNotes:        r.ArrivalNotes,
			ActualArrivalTime:   r.ActualArrivalTime,
			ActualPickupTime:    r.ActualPickupTime,
			CompanionStudentIDs: r.CompanionStudentIDs,
			PhotoURL:            r.PhotoURL,
			PhotoConsentGiven:   r.PhotoConsentGiven,
			HasFullAccess:       r.HasFullAccess,
		}
	}
	return slim
}
