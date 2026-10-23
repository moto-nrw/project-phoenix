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
	Deadline      Deadline
	Days          []ParticipationDay
}

type DailyParticipation struct {
	Regular        bool
	Override       *bool
	SickReportedAt *time.Time
	SickClearedAt  *time.Time
}

// Deadline is the school's change cutoff for a meal day: Time (HH:MM,
// Berlin) on the school day LeadDays before it. LeadDays 0 anchors it on the
// meal day itself. Saturdays and Sundays are not counted.
type Deadline struct {
	Time     string
	LeadDays int
}

// At is the instant after which the day's registration is fixed.
func (d Deadline) At(date Date) time.Time {
	anchor := date
	for remaining := d.LeadDays; remaining > 0; {
		anchor = anchor.AddDays(-1)
		if !weekend(anchor) {
			remaining--
		}
	}
	clock, _ := time.Parse("15:04", d.Time)
	return time.Date(anchor.Year(), anchor.Month(), anchor.Day(), clock.Hour(), clock.Minute(), 0, 0, calendar.Berlin)
}

// Changeable reports whether the day may still be changed at now.
func (d Deadline) Changeable(now time.Time, date Date) bool {
	today := Date(now.In(calendar.Berlin).Format("2006-01-02"))
	return !date.Before(today) && !now.After(d.At(date))
}

// NextEffectiveDate is the first school day whose deadline has not passed
// at now; a changed weekly pattern applies from there.
func (d Deadline) NextEffectiveDate(now time.Time) Date {
	date := Date(now.In(calendar.Berlin).Format("2006-01-02"))
	for weekend(date) || now.After(d.At(date)) {
		date = date.AddDays(1)
	}
	return date
}

func weekend(date Date) bool {
	return date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
}
