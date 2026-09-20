package careplan

import (
	"context"
	"strings"
	"time"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ArrivalWeek maps ISO weekday to the row applicable that day.
type ArrivalWeek map[int]*ArrivalSchedule

// ArrivalPlanByDate keeps a full week per date because the care-day
// derivation must tell "no arrival on Monday" from "no plan on file at all".
type ArrivalPlanByDate map[timezone.Date]ArrivalWeek

// ArrivalPlansByStudent is the per-student projection.
type ArrivalPlansByStudent map[int64]ArrivalPlanByDate

// ArrivalBaselineProjection is the recurring arrival plan applicable on every
// date of a requested range.
type ArrivalBaselineProjection struct {
	WeeklyByStudentDate          ArrivalPlansByStudent
	DerivedByStudentDate         ArrivalPlansByStudent
	BookingsAuthoritative        bool
	ClassExceptionsByStudentDate map[int64]ClassArrivalExceptionsByDate
}

// ForDate returns the effective recurring row for the date's weekday. A
// class-wide day exception (#2962) is already folded in, because it changes
// the class time for that date; per-child day exceptions are deliberately
// outside this contract.
func (p *ArrivalBaselineProjection) ForDate(studentID int64, date timezone.Date) *ArrivalSchedule {
	if p == nil {
		return nil
	}
	row := p.WeeklyByStudentDate[studentID][date][baselineWeekday(date)]
	return classExceptionRow(row, p.ClassExceptionsByStudentDate[studentID][date])
}

// DerivedForDate returns the projected row hidden underneath a manual
// override, if one exists for the date's weekday.
func (p *ArrivalBaselineProjection) DerivedForDate(studentID int64, date timezone.Date) *ArrivalSchedule {
	if p == nil {
		return nil
	}
	return p.DerivedByStudentDate[studentID][date][baselineWeekday(date)]
}

// HasPlan reports whether any recurring arrival weekday exists on the date.
func (p *ArrivalBaselineProjection) HasPlan(studentID int64, date timezone.Date) bool {
	return p != nil && len(p.WeeklyByStudentDate[studentID][date]) > 0
}

// WeeklyForDate returns the full recurring week applicable on date. It never
// includes class day exceptions: callers use this payload to edit a child's
// recurring schedule, and a date-specific time must not become weekly data.
func (p *ArrivalBaselineProjection) WeeklyForDate(studentID int64, date timezone.Date) ArrivalWeek {
	if p == nil {
		return nil
	}
	return p.WeeklyByStudentDate[studentID][date]
}

// ArrivalBaselineReader is the single read boundary for regular arrival times.
type ArrivalBaselineReader interface {
	Project(ctx context.Context, studentIDs []int64, from, to timezone.Date) (*ArrivalBaselineProjection, error)
}

// PickupBaselineProjection is the recurring pickup plan applicable on every
// date in a requested range. WeeklyByStudentDate keeps all weekdays because a
// care-day read must distinguish "no pickup on Monday" from "no plan on file".
type PickupWeek map[int]*PickupSchedule
type PickupPlanByDate map[timezone.Date]PickupWeek
type PickupPlansByStudent map[int64]PickupPlanByDate

type PickupBaselineProjection struct {
	WeeklyByStudentDate   PickupPlansByStudent
	OfferingByStudentDate PickupPlansByStudent
	BookingsAuthoritative bool
	CareDays              CareDayIndex
}

// AllowsPickupForDate reports whether pickup data may affect the given day.
// In legacy mode pickup rows remain independent. In booking mode the approved
// care offering is the boundary, including for date-specific exceptions.
func (p *PickupBaselineProjection) AllowsPickupForDate(studentID int64, date timezone.Date) bool {
	return p.AllowsPickupForWeekday(studentID, date, baselineWeekday(date))
}

func (p *PickupBaselineProjection) AllowsPickupForWeekday(studentID int64, planDate timezone.Date, weekday int) bool {
	return p == nil || !p.BookingsAuthoritative || p.CareDays.Covers(studentID, planDate, weekday)
}

// ForDate returns the effective recurring row for the date's weekday. Day
// exceptions are deliberately outside this contract.
func (p *PickupBaselineProjection) ForDate(studentID int64, date timezone.Date) *PickupSchedule {
	if p == nil {
		return nil
	}
	return p.WeeklyByStudentDate[studentID][date][baselineWeekday(date)]
}

// OfferingForDate returns the booking-derived row hidden underneath a manual
// override, if one exists for the date's weekday.
func (p *PickupBaselineProjection) OfferingForDate(studentID int64, date timezone.Date) *PickupSchedule {
	if p == nil {
		return nil
	}
	return p.OfferingByStudentDate[studentID][date][baselineWeekday(date)]
}

// HasPlan reports whether any recurring pickup weekday exists on the date.
func (p *PickupBaselineProjection) HasPlan(studentID int64, date timezone.Date) bool {
	return p != nil && len(p.WeeklyByStudentDate[studentID][date]) > 0
}

// WeeklyForDate returns the full recurring week applicable on date.
func (p *PickupBaselineProjection) WeeklyForDate(studentID int64, date timezone.Date) PickupWeek {
	if p == nil {
		return nil
	}
	return p.WeeklyByStudentDate[studentID][date]
}

// OfferingWeeklyForDate returns only the booking-derived part of the plan.
// Write paths use it to avoid persisting an unchanged projected value.
func (p *PickupBaselineProjection) OfferingWeeklyForDate(studentID int64, date timezone.Date) PickupWeek {
	if p == nil {
		return nil
	}
	return p.OfferingByStudentDate[studentID][date]
}

// PickupBaselineReader is the single read boundary for regular pickup times.
// Stored staff rows override booking-derived offering rows; legacy materialized
// care_offering rows are ignored.
type PickupBaselineReader interface {
	Project(ctx context.Context, studentIDs []int64, from, to timezone.Date) (*PickupBaselineProjection, error)
	OfferingPickupForDate(ctx context.Context, studentID int64, date timezone.Date) (*PickupSchedule, error)
	HasBookedOfferingPickupForWeekday(ctx context.Context, studentID int64, weekday int) (bool, error)
}

type ApprovedBookingReader interface {
	ListApprovedByStudentIDsInRange(context.Context, []int64, timezone.Date, timezone.Date) ([]*ApprovedBooking, error)
}

type ArrivalBaselineException struct {
	SchoolClass string
	ArrivalTime time.Time
	Label       string
}
type ClassArrivalExceptionsByDate map[timezone.Date]*ArrivalBaselineException
type CareDayIndex map[int64]map[timezone.Date]map[int]bool

func (c CareDayIndex) Covers(studentID int64, date timezone.Date, weekday int) bool {
	return c[studentID][date][weekday]
}
func baselineWeekday(date timezone.Date) int { return (int(date.Weekday())+6)%7 + 1 }
func classExceptionRow(
	row *ArrivalSchedule,
	exception *ArrivalBaselineException,
) *ArrivalSchedule {
	if exception == nil || row == nil {
		return row
	}
	effective := *row
	effective.ExpectedArrival = timezone.NormalizeWallClock(exception.ArrivalTime)
	effective.Source = ScheduleSourceClassException
	effective.SourceClass = exception.SchoolClass
	effective.SourceLabel = exception.Label
	// Notes is what every reader already shows next to the time (Meine
	// Gruppe, Kinderkarte, parents portal), so the label travels there too.
	notes := effective.SourceLabel
	if row.Notes != nil && strings.TrimSpace(*row.Notes) != "" {
		notes += ", " + strings.TrimSpace(*row.Notes)
	}
	effective.Notes = &notes
	return &effective
}
