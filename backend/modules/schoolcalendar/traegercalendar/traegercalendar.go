// Package traegercalendar is School Calendar's contract for the closing days
// of a Träger (#4702) and the Träger's view of its schools' calendars.
package traegercalendar

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Closing days of a Träger (#4702). The Träger-Büro plans a closing day,
// such as a bridge day or a staff outing, for some or all of its schools.
// The rows belong to the Träger and are written in its Träger transaction
// (ADR 0105); each named school reads them as its own closing days, so they
// act like a closing day the school entered itself: no Soll, no Regeltermin,
// marked in the Dienstplan and the calendars, and no bookable holiday care.
// The statutory holidays still come from each school's federal state
// (operations.federal_state), which only the moto team sets; the Träger sees
// it here and cannot change it.

var (
	// ErrClosingDayNotFound reports a closing day that does not exist
	// for the Träger of the session.
	ErrClosingDayNotFound = errors.New("closing day of the Träger not found")
	// ErrSchoolOutside reports a named school that is not
	// an undeleted school of the Träger.
	ErrSchoolOutside = errors.New("the school is not a school of the Träger")
	// ErrInvalidClosingDay reports a closing day without dates,
	// reason or schools, or with its end before its start.
	ErrInvalidClosingDay = errors.New("invalid closing day of the Träger")
)

// ClosingDay is one closing day range of a Träger with the schools it
// applies to. A single day has StartDate == EndDate.
type ClosingDay struct {
	ID        int64         `json:"id"`
	StartDate calendar.Date `json:"start_date"`
	EndDate   calendar.Date `json:"end_date"`
	Reason    string        `json:"reason"`
	SchoolIDs []int64       `json:"school_ids"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// ClosingDayInput is a closing day as the Träger-Büro writes it.
type ClosingDayInput struct {
	StartDate calendar.Date
	EndDate   calendar.Date
	Reason    string
	SchoolIDs []int64
}

// School is a school of the Träger with the federal state its
// statutory holidays come from.
type School struct {
	ID           int64                        `json:"id"`
	Name         string                       `json:"name"`
	FederalState schoolcalendar.HolidayRegion `json:"federal_state"`
}

// Holidays are the statutory holidays of one federal state in the
// window of a calendar overview.
type Holidays struct {
	Region   schoolcalendar.HolidayRegion `json:"region"`
	Holidays []schoolcalendar.Holiday     `json:"holidays"`
}

// Calendar is what the Träger-Büro sees of its schools' calendars in
// a window: its closing days, its schools with their federal state, and the
// statutory holidays of each state.
type Calendar struct {
	ClosingDays []ClosingDay `json:"closing_days"`
	Schools     []School     `json:"schools"`
	Holidays    []Holidays   `json:"holidays"`
}

// ClosingDays lets the Träger-Büro plan closing days. Every operation
// names the Träger of the session and runs in its Träger transaction, which
// the caller opens.
type ClosingDays interface {
	Calendar(ctx context.Context, organizationID int64, from, to calendar.Date) (Calendar, error)
	CreateClosingDay(ctx context.Context, organizationID int64, input ClosingDayInput) (ClosingDay, error)
	UpdateClosingDay(ctx context.Context, organizationID, closingDayID int64, input ClosingDayInput) (ClosingDay, error)
	DeleteClosingDay(ctx context.Context, organizationID, closingDayID int64) error
}

// SchoolState is a school of a Träger with its federal-state ISO code, as
// the root reads it from Organisation & Tenancy and the settings.
type SchoolState struct {
	ID           int64
	Name         string
	FederalState string
}

// Schools is School Calendar's port onto the undeleted schools of a
// Träger and their federal state (operations.federal_state). The root binds
// it; the read runs outside the Träger transaction, whose row security hides
// the schools' settings.
type Schools interface {
	TraegerSchools(ctx context.Context, organizationID int64) ([]SchoolState, error)
}
