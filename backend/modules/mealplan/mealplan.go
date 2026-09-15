// Package mealplan is the public Meal Plan capability.
package mealplan

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

type Date string

func ParseDate(value string) (Date, error) {
	date, err := timezone.ParseDate(value)
	if err != nil {
		return "", err
	}
	return Date(date.String()), nil
}

func (d Date) German() (string, error) {
	date, err := timezone.ParseDate(string(d))
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

type Fault struct {
	Code string
}

func (f *Fault) Error() string { return f.Code }

var (
	ErrDisabled             = &Fault{Code: "meal_plan_disabled"}
	ErrInvalidMealDate      = &Fault{Code: "invalid_meal_date"}
	ErrInvalidDishes        = &Fault{Code: "invalid_dishes"}
	ErrRegistrationDisabled = &Fault{Code: "meal_registration_disabled"}
	ErrInvalidParticipation = &Fault{Code: "invalid_meal_participation"}
	ErrParticipationCutoff  = &Fault{Code: "meal_participation_cutoff_passed"}
)

type engine interface {
	Available(context.Context) (bool, error)
	Week(context.Context, string) ([]Entry, error)
	Replace(context.Context, string, []Dish) error
	Clear(context.Context, string) error
	RegistrationAvailable(context.Context) (bool, error)
	Participation(context.Context, int64, string, string) (ParticipationPlan, error)
	ReplaceParticipationSchedule(context.Context, int64, int64, []Weekday) (Date, error)
	SetParticipationDay(context.Context, int64, int64, string, bool) error
	ClearParticipationDay(context.Context, int64, int64, string) error
	DailyParticipants(context.Context, string) (DailyList, error)
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("meal plan: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) Available(ctx context.Context) (bool, error) {
	return m.engine.Available(ctx)
}

func (m *Module) Week(ctx context.Context, weekStart Date) ([]Entry, error) {
	date, err := timezone.ParseDate(string(weekStart))
	if err != nil {
		return nil, ErrInvalidMealDate
	}
	monday := date.StartOfISOWeek()
	return m.engine.Week(ctx, monday.String())
}

func (m *Module) ReplaceDay(ctx context.Context, command ReplaceDay) error {
	date, err := timezone.ParseDate(string(command.Date))
	if err != nil {
		return ErrInvalidMealDate
	}
	if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		return ErrInvalidMealDate
	}

	dishes := make([]Dish, 0, len(command.Dishes))
	for _, input := range command.Dishes {
		dish := strings.TrimSpace(input.Dish)
		if dish == "" {
			continue
		}
		dishes = append(dishes, Dish{Dish: dish, Note: trimOptional(input.Note)})
	}
	return m.engine.Replace(ctx, date.String(), dishes)
}

func (m *Module) ClearDay(ctx context.Context, date Date) error {
	parsed, err := timezone.ParseDate(string(date))
	if err != nil {
		return ErrInvalidMealDate
	}
	return m.engine.Clear(ctx, parsed.String())
}

func (m *Module) RegistrationAvailable(ctx context.Context) (bool, error) {
	return m.engine.RegistrationAvailable(ctx)
}

func (m *Module) Participation(ctx context.Context, studentID int64, from, to Date) (ParticipationPlan, error) {
	start, startErr := timezone.ParseDate(string(from))
	end, endErr := timezone.ParseDate(string(to))
	if studentID <= 0 || startErr != nil || endErr != nil || end.Before(start) || start.DaysUntil(end) > 31 {
		return ParticipationPlan{}, ErrInvalidParticipation
	}
	return m.engine.Participation(ctx, studentID, start.String(), end.String())
}

func (m *Module) ReplaceParticipationSchedule(ctx context.Context, command ReplaceParticipationSchedule) (Date, error) {
	if command.StudentID <= 0 || command.GuardianAccountID <= 0 {
		return "", ErrInvalidParticipation
	}
	weekdays, ok := normalizeWeekdays(command.Weekdays)
	if !ok {
		return "", ErrInvalidParticipation
	}
	return m.engine.ReplaceParticipationSchedule(ctx, command.StudentID, command.GuardianAccountID, weekdays)
}

func (m *Module) SetParticipationForDay(ctx context.Context, command SetParticipationDay) error {
	date, err := participationDate(command.Date)
	if command.StudentID <= 0 || command.GuardianAccountID <= 0 || err != nil {
		return ErrInvalidParticipation
	}
	return m.engine.SetParticipationDay(ctx, command.StudentID, command.GuardianAccountID, date.String(), command.Participating)
}

func (m *Module) ClearParticipationForDay(ctx context.Context, command SetParticipationDay) error {
	date, err := participationDate(command.Date)
	if command.StudentID <= 0 || command.GuardianAccountID <= 0 || err != nil {
		return ErrInvalidParticipation
	}
	return m.engine.ClearParticipationDay(ctx, command.StudentID, command.GuardianAccountID, date.String())
}

func (m *Module) DailyList(ctx context.Context, date Date) (DailyList, error) {
	parsed, err := participationDate(date)
	if err != nil {
		return DailyList{}, ErrInvalidParticipation
	}
	return m.engine.DailyParticipants(ctx, parsed.String())
}

func participationDate(value Date) (timezone.Date, error) {
	date, err := timezone.ParseDate(string(value))
	if err != nil || date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		return "", ErrInvalidParticipation
	}
	return date, nil
}

func normalizeWeekdays(values []Weekday) ([]Weekday, bool) {
	seen := make(map[Weekday]struct{}, len(values))
	result := make([]Weekday, 0, len(values))
	for _, weekday := range values {
		if weekday < Monday || weekday > Friday {
			return nil, false
		}
		if _, exists := seen[weekday]; exists {
			continue
		}
		seen[weekday] = struct{}{}
		result = append(result, weekday)
	}
	slices.Sort(result)
	return result, true
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
