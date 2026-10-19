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
	// ParticipationAbsent: excused or on a class trip (#3873).
	ParticipationAbsent ParticipationSource = "absent"
	// ParticipationClosed: a closing day or public holiday; nobody eats.
	ParticipationClosed ParticipationSource = "closed"
)

// AbsenceSick is the status of a sick day; every other absence status
// (excused, class trip) shows as absent.
const AbsenceSick = "sick"

type ParticipationSchedule struct {
	EffectiveFrom Date
	Weekdays      []Weekday
}

type DayOverride struct {
	Date          Date
	Participating bool
}

// SickDay is one recorded change of an absence status of a day. Despite
// its name it covers every absence status since #3873.
type SickDay struct {
	ID         int64
	Date       Date
	Status     string
	ChangedAt  time.Time
	ReportedAt time.Time
	ClearedAt  *time.Time
}

// CalendarDays are the school calendar's days in a window: Closed days
// (closing days, public holidays) on which nobody eats, and Holiday days
// (school holiday periods) on which the weekly pattern does not apply.
type CalendarDays struct {
	Closed  map[Date]bool
	Holiday map[Date]bool
}

// NewCalendarDays builds the calendar days from YYYY-MM-DD sets.
func NewCalendarDays(closed, holiday map[string]bool) CalendarDays {
	days := CalendarDays{Closed: make(map[Date]bool, len(closed)), Holiday: make(map[Date]bool, len(holiday))}
	for date := range closed {
		days.Closed[Date(date)] = true
	}
	for date := range holiday {
		days.Holiday[Date(date)] = true
	}
	return days
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
	// Registered is the registration before absences; Regular what the
	// weekly pattern alone gives on the day (#3873).
	Registered    bool
	Regular       bool
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

// DailyParticipation is one child's state on one day. Absences holds the
// latest recorded change of each absence status up to the cutoff.
type DailyParticipation struct {
	Regular  bool
	Override *bool
	// OverrideBy is the account of the day's exception, which a change of
	// the order records (#3873).
	OverrideBy *int64
	Absences   []SickDay
}

type DailyParticipant struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
}

// DailyList is the kitchen's list of a day. Deadline is when the day's
// order is fixed; CutoffTime is its clock time.
type DailyList struct {
	Date         Date
	CutoffTime   string
	Deadline     time.Time
	Closed       bool
	Participants []DailyParticipant
	// Others are the enrolled children who do not eat that day, for the
	// school's late changes.
	Others []DailyParticipant
}

// Add puts a child on the list of those who eat or of the others.
func (l *DailyList) Add(child DailyParticipant, participating bool) {
	if participating {
		l.Participants = append(l.Participants, child)
		return
	}
	l.Others = append(l.Others, child)
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
