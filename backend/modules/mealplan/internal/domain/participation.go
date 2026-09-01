package domain

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
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
	CutoffTime    string
	Days          []ParticipationDay
}

type DailyCandidate struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
}

type DailyParticipation struct {
	Regular        bool
	Override       *bool
	SickReportedAt *time.Time
	SickClearedAt  *time.Time
}

type DailyParticipant struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
}

type DailyList struct {
	Date         Date
	CutoffTime   string
	Participants []DailyParticipant
}

func CutoffAt(date Date, cutoffTime string) time.Time {
	clock, _ := time.Parse("15:04", cutoffTime)
	return time.Date(date.Year(), date.Month(), date.Day(), clock.Hour(), clock.Minute(), 0, 0, timezone.Berlin)
}

func Changeable(now time.Time, date Date, cutoffTime string) bool {
	now = now.In(timezone.Berlin)
	today := Date(now.Format("2006-01-02"))
	return !date.Before(today) && !now.After(CutoffAt(date, cutoffTime))
}

func NextEffectiveDate(now time.Time, cutoffTime string) Date {
	now = now.In(timezone.Berlin)
	date := Date(now.Format("2006-01-02"))
	if now.After(CutoffAt(date, cutoffTime)) {
		date = date.AddDays(1)
	}
	for date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		date = date.AddDays(1)
	}
	return date
}
