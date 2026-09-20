package careplan

import (
	"time"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ArrivalNoteData struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
}

type EffectiveArrivalTime struct {
	Date        timezone.Date     `json:"date"`
	ArrivalTime *time.Time        `json:"arrival_time"`
	WeekdayName string            `json:"weekday_name"`
	IsException bool              `json:"is_exception"`
	Notes       string            `json:"notes,omitempty"`
	DayNotes    []ArrivalNoteData `json:"day_notes,omitempty"`
	// ChangedAt is when the overriding day exception was recorded; set only
	// together with IsException.
	ChangedAt *time.Time `json:"changed_at,omitempty"`
	// ClassException names the class-wide day exception the time comes from
	// (#2962). Nil when the time is the regular one or a per-child exception
	// overrides the day.
	ClassException *ClassArrivalExceptionInfo `json:"class_exception,omitempty"`
}

// ClassArrivalExceptionInfo is what readers show next to a time that a
// class-wide day exception set (#2962).
type ClassArrivalExceptionInfo struct {
	SchoolClass string `json:"school_class"`
	ArrivalTime string `json:"arrival_time"`
	// Label is the ready-made line, e.g. "Klasse 4a: Unterricht fällt aus".
	Label string `json:"label"`
}

type NoteData struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
}

type EffectivePickupTime struct {
	Date        timezone.Date `json:"date"`
	PickupTime  *time.Time    `json:"pickup_time"`
	WeekdayName string        `json:"weekday_name"`
	IsException bool          `json:"is_exception"`
	Notes       string        `json:"notes,omitempty"`
	DayNotes    []NoteData    `json:"day_notes,omitempty"`
	// RegularPickupTime is the recurring plan's time for that weekday, kept
	// next to the effective one so readers can name the deviation instead of
	// only flagging it (#2294). Nil when the plan carries no time that day.
	RegularPickupTime *time.Time `json:"regular_pickup_time,omitempty"`
	// ChangedAt is when the overriding day exception was recorded; set only
	// together with IsException.
	ChangedAt *time.Time `json:"changed_at,omitempty"`
}
