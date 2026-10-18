package domain

import (
	"sort"
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

// ParticipationDay is one meal day of a child as the parent portal shows
// it. RegisterUntil and CancelUntil are the deadlines of that day; the Can
// flags say whether the parent may still use them.
type ParticipationDay struct {
	Date          Date
	Participating bool
	Source        ParticipationSource
	CanRegister   bool
	CanCancel     bool
	RegisterUntil time.Time
	CancelUntil   time.Time
}

// Changeable reports whether the parent may still change the day at all.
func (d ParticipationDay) Changeable() bool { return d.CanRegister || d.CanCancel }

type ParticipationPlan struct {
	Weekdays      []Weekday
	EffectiveFrom Date
	CutoffTime    string
	// Model is the school's registration model; ScheduleChangeable says
	// whether the parent may still change the weekly pattern.
	Model              RegistrationModel
	ScheduleChangeable bool
	// Sick says whether an absence after the cancellation deadline still
	// cancels the meal.
	Sick SickMode
	Days []ParticipationDay
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

// SortDailyParticipants orders the kitchen list by class, last name, first
// name and student ID.
func SortDailyParticipants(participants []DailyParticipant) {
	sort.SliceStable(participants, func(left, right int) bool {
		a, b := participants[left], participants[right]
		if a.SchoolClass != b.SchoolClass {
			return a.SchoolClass < b.SchoolClass
		}
		if a.LastName != b.LastName {
			return a.LastName < b.LastName
		}
		if a.FirstName != b.FirstName {
			return a.FirstName < b.FirstName
		}
		return a.StudentID < b.StudentID
	})
}

func CutoffAt(date Date, cutoffTime string) time.Time {
	clock, _ := time.Parse("15:04", cutoffTime)
	return time.Date(date.Year(), date.Month(), date.Day(), clock.Hour(), clock.Minute(), 0, 0, calendar.Berlin)
}
