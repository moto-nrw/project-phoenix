package enrollment

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Places of the holiday weeks (#4699, decision E-e). A week has a capacity
// across its time models or none. A registration's bookings get their place
// when the parents send the form, in the order the registrations came in:
// received and confirmed registrations count. A booking without a free
// place waits on the waiting list of its week and moves up, bindingly and
// with a mail to the parents, when a place becomes free: by a cancellation,
// a rebooking out of the week or a higher capacity. The order of the
// waiting list is the time the registration came in, its number, then the
// position of the child in the form. Siblings may get places in different
// weeks. Only placed bookings reach the host school and the contribution.
//
// Every placement decision holds the week's row with SELECT ... FOR UPDATE
// in READ COMMITTED, weeks in ascending ID. A submission inserts its
// registration first and locks the weeks after, the Träger-Büro locks the
// period, then the registration, then the weeks: no lock order runs the
// other way round. A booking of days (#4701) or an extra day (#4817) lies
// in one week and takes the same lock.

// Placements of a booking.
const (
	HolidayPlacementPlaced     = "placed"
	HolidayPlacementWaitlisted = "waitlisted"
)

// Actions of the audit trail of the places (#4699): a booking moved up, a
// capacity changed, the places of a period were read.
const (
	HolidayAuditPromote  = "promote"
	HolidayAuditCapacity = "capacity"
	HolidayAuditPlaces   = "places"
)

// HolidayWeekCapacityMax bounds the capacity of one week.
const HolidayWeekCapacityMax = 10000

// MailKindHolidayPlaceGranted tells the parents that a child moved up from
// the waiting list (#4699), a mail of the Träger.
const MailKindHolidayPlaceGranted = "holiday_place_granted"

// ErrHolidayWeekNotFound reports a week that is not a week of the period.
var ErrHolidayWeekNotFound = errors.New("holiday week not found")

// HolidayReceiptPlacement is the placement of one booking of a sent
// registration. ChildPosition is the child's position in the form, from 1.
type HolidayReceiptPlacement struct {
	ChildPosition int    `json:"child_position"`
	WeekID        int64  `json:"week_id"`
	Placement     string `json:"placement"`
}

// HolidayPromotion is one booking that moved up from the waiting list.
type HolidayPromotion struct {
	// BookingID identifies the persisted booking generation for mail deduplication.
	BookingID      int64 `json:"-"`
	RegistrationID int64 `json:"registration_id"`
	ChildID        int64 `json:"child_id"`
	WeekID         int64 `json:"week_id"`
	// ChildPosition is the child's position in the form, from 1.
	ChildPosition int `json:"-"`
}

// HolidayWaitlistEntry is one booking on the waiting list of a week, in its
// order. It names the child, the registration and when it came in.
type HolidayWaitlistEntry struct {
	Position           int       `json:"position"`
	RegistrationID     int64     `json:"registration_id"`
	RegistrationStatus string    `json:"registration_status"`
	SubmittedAt        time.Time `json:"submitted_at"`
	ChildID            int64     `json:"child_id"`
	ChildFirstName     string    `json:"child_first_name"`
	ChildLastName      string    `json:"child_last_name"`
	TimeModelName      string    `json:"time_model_name"`
	// Deferred marks a booking that waits behind the others because the
	// child has no holiday care wish (#4703).
	Deferred bool `json:"deferred"`
}

// HolidayWeekPlaces are the places of one week: its capacity, nil for
// unlimited, the placed bookings of registrations that count and the
// waiting list.
type HolidayWeekPlaces struct {
	WeekID       int64                  `json:"week_id"`
	StartsOn     calendar.Date          `json:"starts_on"`
	EndsOn       calendar.Date          `json:"ends_on"`
	HostSchoolID int64                  `json:"host_school_id"`
	Capacity     *int                   `json:"capacity"`
	Placed       int                    `json:"placed"`
	Waitlist     []HolidayWaitlistEntry `json:"waitlist"`
}

// HolidayCapacityChange is a week after its capacity changed, with the
// bookings that moved up.
type HolidayCapacityChange struct {
	Week       HolidayWeekPlaces  `json:"week"`
	Promotions []HolidayPromotion `json:"promotions"`
}

// HolidayPlaceGrantedMail is the payload of the mail that a child moved up.
// It names the weeks with their dates, school and time model, no date of
// birth and no health information.
type HolidayPlaceGrantedMail struct {
	Recipient    string `json:"recipient_email"`
	GuardianName string `json:"guardian_name"`
	TraegerName  string `json:"traeger_name"`
	PeriodName   string `json:"period_name"`
	Binding      bool   `json:"binding"`
	// Children are the children that moved up, each with the weeks it now
	// has a place in.
	Children []HolidayRegistrationMailChild `json:"children"`
}

// HolidayPlaces lets the Träger-Büro see and steer the places of a period.
// Every operation runs in the Träger transaction of the reviewer's Träger.
type HolidayPlaces interface {
	// HolidayWeekPlaces lists the places and the waiting list of every week
	// of the period and records the read in the period's audit trail.
	HolidayWeekPlaces(ctx context.Context, reviewer HolidayReviewer, periodID int64) ([]HolidayWeekPlaces, error)
	// SetHolidayWeekCapacity sets the capacity of a week, nil for unlimited,
	// and moves bookings up from the waiting list into the places it frees,
	// with a mail to their parents. A lower capacity takes no place away.
	// A capacity outside 0 to HolidayWeekCapacityMax is a validation error
	// on capacity; a week of another period ErrHolidayWeekNotFound.
	SetHolidayWeekCapacity(ctx context.Context, reviewer HolidayReviewer, periodID, weekID int64, capacity *int) (HolidayCapacityChange, error)
}
