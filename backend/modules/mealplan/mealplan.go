// Package mealplan is the public Meal Plan capability.
package mealplan

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type Date string

func ParseDate(value string) (Date, error) {
	date, err := calendar.ParseDate(value)
	if err != nil {
		return "", err
	}
	return Date(date.String()), nil
}

func (d Date) German() (string, error) {
	date, err := calendar.ParseDate(string(d))
	if err != nil {
		return "", err
	}
	return date.Format("02.01.2006"), nil
}

type Dish struct {
	Dish      string
	Note      *string
	Allergens []Allergen
}

type Entry struct {
	Date      Date
	Position  int
	Dish      string
	Note      *string
	Allergens []Allergen
}

type Weekday int

const (
	Monday Weekday = iota + 1
	Tuesday
	Wednesday
	Thursday
	Friday
)

type ParticipationSource string

const (
	ParticipationNone     ParticipationSource = "none"
	ParticipationRegular  ParticipationSource = "regular"
	ParticipationOverride ParticipationSource = "override"
	ParticipationSick     ParticipationSource = "sick"
	// ParticipationClosed is a statutory holiday or closing day: no meal.
	ParticipationClosed ParticipationSource = "closed"
)

type ParticipationDay struct {
	Date          Date
	Participating bool
	Source        ParticipationSource
	// Changeable says the one change the day allows is still open:
	// cancelling when Participating, registering otherwise.
	Changeable bool
}

// Cutoffs are the school's change cutoffs.
type Cutoffs struct {
	CutoffTime string
	// CutoffLeadDays is how many school days before the meal day the
	// CutoffTime applies; 0 is the meal day itself. Statutory holidays and
	// closing days do not count.
	CutoffLeadDays int
	// CutoffWeekday, when set, ends the changes of a whole week at
	// CutoffTime on this weekday of the week before; CutoffLeadDays is then
	// unused.
	CutoffWeekday Weekday
	// CancellationCutoffTime, when set, is a later cutoff for taking a
	// child off a day, CancellationCutoffLeadDays school days before it.
	CancellationCutoffTime     string
	CancellationCutoffLeadDays int
}

type ParticipationPlan struct {
	Cutoffs
	Weekdays      []Weekday
	EffectiveFrom Date
	Days          []ParticipationDay
}

type ReplaceParticipationSchedule struct {
	StudentID         int64
	GuardianAccountID int64
	Weekdays          []Weekday
}

type SetParticipationDay struct {
	StudentID         int64
	GuardianAccountID int64
	Date              Date
	Participating     bool
}

type DailyParticipant struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
}

// StaffParticipationChange is a change the team makes for a family, for
// example after a phone call. It ignores the cutoffs but not past or
// closed days.
type StaffParticipationChange struct {
	StudentID int64
	AccountID int64
	Date      Date
	// Participating nil returns the day to the child's fixed days.
	Participating *bool
}

type DailyList struct {
	Cutoffs
	Date Date
	// Closed is a statutory holiday or closing day without a meal.
	Closed       bool
	Participants []DailyParticipant
	// Available are the children of the day who are not participating;
	// the team can sign them up.
	Available []DailyParticipant
}

// WeeklyList holds the DailyList of each school day, Monday to Friday.
type WeeklyList struct {
	Cutoffs
	WeekStart Date
	Days      []DailyList
}

type ReplaceDay struct {
	Date   Date
	Dishes []Dish
}

// Fault is a refused Meal Plan request. Code names it; Detail, when set, is
// the client text the response carries instead of the code.
type Fault struct {
	Code   string
	Detail string
}

func (f *Fault) Error() string { return f.Code }

var (
	ErrDisabled             = &Fault{Code: "meal_plan_disabled", Detail: "feature_disabled"}
	ErrInvalidMealDate      = &Fault{Code: "invalid_meal_date", Detail: "meal plan covers weekdays only (Monday-Friday)"}
	ErrInvalidDishes        = &Fault{Code: "invalid_dishes"}
	ErrInvalidAllergens     = &Fault{Code: "invalid_allergens", Detail: "allergens must be codes of the 14 main allergens"}
	ErrRegistrationDisabled = &Fault{Code: "meal_registration_disabled", Detail: "meal_registration_disabled"}
	ErrInvalidParticipation = &Fault{Code: "invalid_meal_participation"}
	ErrParticipationCutoff  = &Fault{Code: "meal_participation_cutoff_passed", Detail: "meal_participation_cutoff_passed"}
)

type Capability interface {
	Available(context.Context) (bool, error)
	Week(context.Context, Date) ([]Entry, error)
	ReplaceDay(context.Context, ReplaceDay) error
	ClearDay(context.Context, Date) error
	RegistrationAvailable(context.Context) (bool, error)
	Participation(context.Context, int64, Date, Date) (ParticipationPlan, error)
	ReplaceParticipationSchedule(context.Context, ReplaceParticipationSchedule) (Date, error)
	SetParticipationForDay(context.Context, SetParticipationDay) error
	ClearParticipationForDay(context.Context, SetParticipationDay) error
	DailyList(context.Context, Date) (DailyList, error)
	WeeklyList(context.Context, Date) (WeeklyList, error)
	ChangeParticipationAsStaff(context.Context, StaffParticipationChange) error
}
