package carecontract

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The evaluation of overruns over all schools of a Träger (#4683, User
// Story 57, ADR 0156): the Träger-Büro sees per school how often children
// stayed longer than booked, so it can talk to the parents or change the
// booking. The comparison itself is Soll und Ist of each school (ADR 0139),
// whose Soll is the contract where one is in force. By default the
// evaluation shows only figures per school; the children by name need the
// right "Verträge verwalten" (ADR 0084 section 4, Spec #4611 story 118).

// MaxOverrunWindowDays bounds one evaluation: every school computes its
// comparison over the window, so it stays one quarter at most.
const MaxOverrunWindowDays = 92

// ErrOverrunWindowInvalid reports a missing, reversed, future or too long
// window.
var ErrOverrunWindowInvalid = errors.New("the evaluation window is invalid")

// OverrunFilter selects the window and optionally one school.
type OverrunFilter struct {
	From     calendar.Date
	To       calendar.Date
	SchoolID int64
}

// OverrunFigures totals the overruns of children. ExtraMinutes is the time
// beyond the booking: earlier or later than booked, and the whole stay on a
// day without a booking.
type OverrunFigures struct {
	Children     int `json:"children"`
	OverrunDays  int `json:"overrun_days"`
	UnbookedDays int `json:"unbooked_days"`
	ExtraMinutes int `json:"extra_minutes"`
}

// SchoolOverruns are the figures of one school. DataFrom is the first day
// the school compared: its retention of check-ins may cut the window short.
type SchoolOverruns struct {
	SchoolID   int64          `json:"school_id"`
	SchoolName string         `json:"school_name"`
	DataFrom   calendar.Date  `json:"data_from"`
	Figures    OverrunFigures `json:"figures"`
}

// OverrunChild is one child with at least one overrun in the window.
type OverrunChild struct {
	SchoolID     int64  `json:"school_id"`
	SchoolName   string `json:"school_name"`
	StudentID    int64  `json:"student_id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	SchoolClass  string `json:"school_class"`
	OverrunDays  int    `json:"overrun_days"`
	UnbookedDays int    `json:"unbooked_days"`
	ExtraMinutes int    `json:"extra_minutes"`
}

// OverrunEvaluation answers the evaluation. Children is empty in the summary.
type OverrunEvaluation struct {
	From             calendar.Date    `json:"from"`
	To               calendar.Date    `json:"to"`
	ToleranceMinutes int              `json:"tolerance_minutes"`
	Total            OverrunFigures   `json:"total"`
	Schools          []SchoolOverruns `json:"schools"`
	Children         []OverrunChild   `json:"children"`
}

// SchoolOverrunRow is one child of a school's overrun list.
type SchoolOverrunRow struct {
	StudentID    int64
	FirstName    string
	LastName     string
	SchoolClass  string
	OverrunDays  int
	UnbookedDays int
	ExtraMinutes int
}

// SchoolOverrunList is a school's overrun list over the window.
type SchoolOverrunList struct {
	DataFrom         calendar.Date
	ToleranceMinutes int
	Rows             []SchoolOverrunRow
}

// SchoolCareTimes reads the overrun list of the school whose tenant
// transaction the caller holds (Student Presence, ADR 0139). The read
// records itself in the school's access log with the reviewer's account.
type SchoolCareTimes interface {
	Overruns(ctx context.Context, reviewer Reviewer, from, to calendar.Date) (SchoolOverrunList, error)
}

// OverrunEvaluations evaluate the overruns over the schools of the Träger.
// They read each school in its own tenant transaction and open their units
// themselves, so the caller holds no transaction.
type OverrunEvaluations interface {
	// OverrunSummary answers the figures per school without names.
	OverrunSummary(ctx context.Context, reviewer Reviewer, filter OverrunFilter) (OverrunEvaluation, error)
	// OverrunChildren answers the figures and every child with an overrun,
	// and records the children shown in the access log of each school.
	OverrunChildren(ctx context.Context, reviewer Reviewer, filter OverrunFilter) (OverrunEvaluation, error)
}

// SchoolAccessOverruns is a read of the children with overruns of a school
// by the Träger-Büro.
const SchoolAccessOverruns = "care_time_overruns"
