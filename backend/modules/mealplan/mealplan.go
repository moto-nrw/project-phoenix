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
	Dish string
	Note *string
}

type Entry struct {
	Date     Date
	Position int
	Dish     string
	Note     *string
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
)

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

type DailyList struct {
	Date         Date
	CutoffTime   string
	Participants []DailyParticipant
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
}
