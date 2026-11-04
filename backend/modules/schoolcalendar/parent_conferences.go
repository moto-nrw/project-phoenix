package schoolcalendar

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Elternsprechtag (#4801): a Lehrkraft offers one day for one of her
// classes, the families book one slot per child. The slots follow from the
// day's window, slot length and breaks; they are never stored one by one.
// Minutes count from midnight in the school's wall clock.

const (
	// MinParentConferenceSlotMinutes and MaxParentConferenceSlotMinutes bound
	// the length of one conversation.
	MinParentConferenceSlotMinutes = 5
	MaxParentConferenceSlotMinutes = 120
	// MaxParentConferenceBreaks keeps the form and the slot expansion small.
	MaxParentConferenceBreaks = 10
	minutesPerDay             = 24 * 60
)

// ErrInvalidParentConference wraps every validation failure of a day.
var ErrInvalidParentConference = errors.New("invalid parent conference")

var (
	ErrParentConferenceNotFound = errors.New("parent conference not found")
	// ErrParentConferenceSlotTaken is the double-booking guard: another child
	// holds the slot, also when both families booked at the same moment.
	ErrParentConferenceSlotTaken = errors.New("parent conference slot already booked")
	// ErrParentConferenceStudentBooked means the child already holds a slot
	// on this day; a family moves it instead of booking a second one.
	ErrParentConferenceStudentBooked = errors.New("child already holds a slot at this parent conference")
)

// ParentConferenceBreak is a pause in the day, [StartMinute, EndMinute).
type ParentConferenceBreak struct {
	StartMinute int `json:"start_minute"`
	EndMinute   int `json:"end_minute"`
}

// ParentConferenceFields are the values a Lehrkraft enters for a day.
type ParentConferenceFields struct {
	OrganizerStaffID int64
	// OrganizerAccountID is the account the Lehrkraft hears about bookings
	// on; zero once that account is gone.
	OrganizerAccountID int64
	SchoolClass        string
	Date               calendar.Date
	StartMinute        int
	EndMinute          int
	SlotMinutes        int
	Breaks             []ParentConferenceBreak
	Location           *string
}

// ParentConference is one stored Elternsprechtag.
type ParentConference struct {
	ID       int64 `json:"id"`
	TenantID int64 `json:"tenant_id"`
	ParentConferenceFields
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ParentConferenceBooking is the slot one child holds. AppointmentID is the
// calendar appointment that shows it to the family and the Lehrkraft.
type ParentConferenceBooking struct {
	ID                int64     `json:"id"`
	TenantID          int64     `json:"tenant_id"`
	ConferenceID      int64     `json:"conference_id"`
	StudentID         int64     `json:"student_id"`
	GuardianProfileID *int64    `json:"guardian_profile_id,omitempty"`
	SlotStartMinute   int       `json:"slot_start_minute"`
	AppointmentID     *int64    `json:"appointment_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// ParentConferenceFilter narrows a list. A zero OrganizerStaffID lists every
// Lehrkraft's days; a zero From lists past days as well.
type ParentConferenceFilter struct {
	OrganizerStaffID int64
	From             calendar.Date
}

// ParentConferences stores the days and their bookings of the school in
// context. A write runs in the caller's tenant unit of work.
type ParentConferences interface {
	CreateParentConference(ctx context.Context, fields ParentConferenceFields) (*ParentConference, error)
	UpdateParentConference(ctx context.Context, id int64, fields ParentConferenceFields) (*ParentConference, error)
	// FindParentConference returns ErrParentConferenceNotFound for a missing
	// day; lock holds the row until the unit of work ends.
	FindParentConference(ctx context.Context, id int64, lock bool) (*ParentConference, error)
	ListParentConferences(ctx context.Context, filter ParentConferenceFilter) ([]*ParentConference, error)
	DeleteParentConference(ctx context.Context, id int64) error
	ListParentConferenceBookings(ctx context.Context, conferenceIDs []int64) ([]*ParentConferenceBooking, error)
	// CreateParentConferenceBooking reports ErrParentConferenceSlotTaken or
	// ErrParentConferenceStudentBooked from the database constraints.
	CreateParentConferenceBooking(ctx context.Context, booking ParentConferenceBooking) (*ParentConferenceBooking, error)
	// MoveParentConferenceBooking reports ErrParentConferenceSlotTaken when
	// another child holds the new slot.
	MoveParentConferenceBooking(ctx context.Context, bookingID int64, slotStartMinute int) error
	// LinkParentConferenceBookingAppointment records the calendar
	// appointment that shows the booking.
	LinkParentConferenceBookingAppointment(ctx context.Context, bookingID, appointmentID int64) error
	DeleteParentConferenceBookings(ctx context.Context, ids []int64) error
	// DeleteParentConferencesBefore deletes the days before cutoff with their
	// bookings: the end of the school year ends every booking (#4801). The
	// appointments that showed the bookings go through Appointments'
	// retention. It returns the number of deleted bookings and days together.
	DeleteParentConferencesBefore(ctx context.Context, cutoff calendar.Date) (int, error)
}

// ValidateParentConferenceFields trims the free text and checks the window,
// the slot length and the breaks. Breaks come back sorted.
func ValidateParentConferenceFields(fields ParentConferenceFields) (ParentConferenceFields, error) {
	fields.SchoolClass = strings.TrimSpace(fields.SchoolClass)
	if fields.OrganizerStaffID <= 0 || fields.OrganizerAccountID < 0 {
		return fields, invalidParentConference("organizer is required")
	}
	if fields.SchoolClass == "" {
		return fields, invalidParentConference("school_class is required")
	}
	if _, err := calendar.ParseDate(fields.Date.String()); err != nil {
		return fields, invalidParentConference("date is required")
	}
	if fields.StartMinute < 0 || fields.EndMinute > minutesPerDay || fields.StartMinute >= fields.EndMinute {
		return fields, invalidParentConference("end must be after start on the same day")
	}
	if fields.SlotMinutes < MinParentConferenceSlotMinutes || fields.SlotMinutes > MaxParentConferenceSlotMinutes {
		return fields, invalidParentConference(fmt.Sprintf("slot length must be between %d and %d minutes",
			MinParentConferenceSlotMinutes, MaxParentConferenceSlotMinutes))
	}
	if fields.Location != nil {
		location := strings.TrimSpace(*fields.Location)
		fields.Location = &location
		if location == "" {
			fields.Location = nil
		}
	}
	breaks, err := validateParentConferenceBreaks(fields)
	if err != nil {
		return fields, err
	}
	fields.Breaks = breaks
	if len(ParentConferenceSlots(fields)) == 0 {
		return fields, invalidParentConference("the day has no slot")
	}
	return fields, nil
}

func validateParentConferenceBreaks(fields ParentConferenceFields) ([]ParentConferenceBreak, error) {
	if len(fields.Breaks) > MaxParentConferenceBreaks {
		return nil, invalidParentConference(fmt.Sprintf("at most %d breaks", MaxParentConferenceBreaks))
	}
	breaks := slices.Clone(fields.Breaks)
	slices.SortFunc(breaks, func(a, b ParentConferenceBreak) int { return a.StartMinute - b.StartMinute })
	for i, pause := range breaks {
		if pause.StartMinute >= pause.EndMinute {
			return nil, invalidParentConference("a break must end after it starts")
		}
		if pause.StartMinute <= fields.StartMinute || pause.EndMinute >= fields.EndMinute {
			return nil, invalidParentConference("a break must lie inside the day")
		}
		if i > 0 && pause.StartMinute < breaks[i-1].EndMinute {
			return nil, invalidParentConference("breaks must not overlap")
		}
	}
	if breaks == nil {
		breaks = []ParentConferenceBreak{}
	}
	return breaks, nil
}

// ParentConferenceSlots returns the start minute of every slot: one after
// the other from the start, never across a break and never past the end.
// After a break the slots continue at its end.
func ParentConferenceSlots(fields ParentConferenceFields) []int {
	if fields.SlotMinutes <= 0 {
		return nil
	}
	slots := []int{}
	for start := fields.StartMinute; start+fields.SlotMinutes <= fields.EndMinute; {
		end := start + fields.SlotMinutes
		if pause, ok := overlappingBreak(fields.Breaks, start, end); ok {
			start = pause.EndMinute
			continue
		}
		slots = append(slots, start)
		start = end
	}
	return slots
}

func overlappingBreak(breaks []ParentConferenceBreak, start, end int) (ParentConferenceBreak, bool) {
	for _, pause := range breaks {
		if start < pause.EndMinute && pause.StartMinute < end {
			return pause, true
		}
	}
	return ParentConferenceBreak{}, false
}

// HasParentConferenceSlot reports whether the day offers a slot starting at
// minute.
func HasParentConferenceSlot(fields ParentConferenceFields, minute int) bool {
	return slices.Contains(ParentConferenceSlots(fields), minute)
}

// ParentConferenceSlotStart is the instant a slot begins.
func ParentConferenceSlotStart(date calendar.Date, minute int) time.Time {
	midnight := date.BerlinMidnight()
	return time.Date(midnight.Year(), midnight.Month(), midnight.Day(), minute/60, minute%60, 0, 0, midnight.Location())
}

func invalidParentConference(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidParentConference, reason)
}
