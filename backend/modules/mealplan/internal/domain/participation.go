package domain

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type Weekday int

type ParticipationSource string

const (
	ParticipationNone     ParticipationSource = "none"
	ParticipationRegular  ParticipationSource = "regular"
	ParticipationOverride ParticipationSource = "override"
	ParticipationSick     ParticipationSource = "sick"
	// ParticipationClosed is a statutory holiday or closing day.
	ParticipationClosed ParticipationSource = "closed"
)

type ParticipationSchedule struct {
	EffectiveFrom Date
	Weekdays      []Weekday
}

type DayOverride struct {
	Date          Date
	Participating bool
}

type SickDay struct {
	ID         int64
	Date       Date
	ChangedAt  time.Time
	ReportedAt time.Time
	ClearedAt  *time.Time
}

type ParticipationData struct {
	Schedules []ParticipationSchedule
	Overrides []DayOverride
	SickDays  []SickDay
}

type ParticipationDay struct {
	Date          Date
	Participating bool
	Source        ParticipationSource
	Changeable    bool
}

type ParticipationPlan struct {
	Weekdays      []Weekday
	EffectiveFrom Date
	Rules         Rules
	Days          []ParticipationDay
}

type DailyParticipation struct {
	Regular        bool
	Override       *bool
	SickReportedAt *time.Time
	SickClearedAt  *time.Time
}

// Deadline is one change cutoff for a meal day: Time (HH:MM, Berlin) on
// the school day LeadDays before it. LeadDays 0 anchors it on the meal day
// itself. Saturdays, Sundays and Closed days are not counted. A Weekday
// (Monday to Friday) instead anchors the cutoff of every day of a week on
// that weekday of the week before ("Mittwoch für die Folgewoche").
type Deadline struct {
	Time     string
	LeadDays int
	Weekday  Weekday
	// Closed holds the statutory holidays and closing days around the
	// requested dates. There is no meal on them, and they do not count as
	// school days of the lead.
	Closed map[Date]bool
}

// At is the instant after which the day's registration is fixed.
func (d Deadline) At(date Date) time.Time {
	anchor := date
	if d.Weekday >= 1 && d.Weekday <= 5 {
		anchor = date.StartOfISOWeek().AddDays(int(d.Weekday) - 1 - 7)
	} else {
		for remaining := d.LeadDays; remaining > 0; {
			anchor = anchor.AddDays(-1)
			if !weekend(anchor) && !d.Closed[anchor] {
				remaining--
			}
		}
	}
	clock, _ := time.Parse("15:04", d.Time)
	return time.Date(anchor.Year(), anchor.Month(), anchor.Day(), clock.Hour(), clock.Minute(), 0, 0, calendar.Berlin)
}

// Changeable reports whether the day may still be changed at now. A closed
// day has no meal and is never changeable.
func (d Deadline) Changeable(now time.Time, date Date) bool {
	return open(now, date, d.At(date)) && !d.Closed[date]
}

// NextEffectiveDate is the first school day whose deadline has not passed
// at now; a changed weekly pattern applies from there.
func (d Deadline) NextEffectiveDate(now time.Time) Date {
	date := Date(now.In(calendar.Berlin).Format("2006-01-02"))
	for weekend(date) || d.Closed[date] || now.After(d.At(date)) {
		date = date.AddDays(1)
	}
	return date
}

// Rules are the school's cutoffs: Registration for signing a child up and
// for every change of the weekly pattern, Cancellation for taking a child
// off a day. A Cancellation without Time is the Registration deadline; one
// that would end earlier never does.
type Rules struct {
	Registration Deadline
	Cancellation Deadline
}

// Closed reports whether the day has no meal.
func (r Rules) Closed(date Date) bool { return r.Registration.Closed[date] }

// CancellationAt is the instant after which the child can no longer be
// taken off the day; a sick note after it no longer changes the kitchen
// list.
func (r Rules) CancellationAt(date Date) time.Time {
	registration := r.Registration.At(date)
	if r.Cancellation.Time == "" {
		return registration
	}
	cancellation := r.Cancellation.At(date)
	if cancellation.Before(registration) {
		return registration
	}
	return cancellation
}

// CanCancel reports whether the child may still be taken off the day.
func (r Rules) CanCancel(now time.Time, date Date) bool {
	return open(now, date, r.CancellationAt(date)) && !r.Closed(date)
}

func open(now time.Time, date Date, cutoff time.Time) bool {
	today := Date(now.In(calendar.Berlin).Format("2006-01-02"))
	return !date.Before(today) && !now.After(cutoff)
}

func weekend(date Date) bool {
	return date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
}
