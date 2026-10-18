// Package mealplan is the public Meal Plan capability.
package mealplan

import (
	"context"
	"slices"
	"strings"
	"time"

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
	// ParticipationAbsent: excused or on a class trip. The reason stays
	// with the school.
	ParticipationAbsent ParticipationSource = "absent"
	// ParticipationClosed: a closing day or public holiday; nobody eats.
	ParticipationClosed ParticipationSource = "closed"
)

// RegistrationModel is how a school's parents register for lunch (#3873).
type RegistrationModel string

const (
	RegistrationModelSingle RegistrationModel = "ongoing_single"
	RegistrationModelSplit  RegistrationModel = "ongoing_split"
	RegistrationModelAnnual RegistrationModel = "annual"
)

// ParticipationDay is one meal day of a child. Changeable is true while the
// parent may still register or cancel it; RegisterUntil and CancelUntil are
// the deadlines of the day.
type ParticipationDay struct {
	Date          Date
	Participating bool
	// Regular is what the weekly pattern alone gives on the day: false in
	// school holidays and on closed days (#3873).
	Regular       bool
	Source        ParticipationSource
	Changeable    bool
	CanRegister   bool
	CanCancel     bool
	RegisterUntil time.Time
	CancelUntil   time.Time
}

type ParticipationPlan struct {
	Weekdays           []Weekday
	EffectiveFrom      Date
	CutoffTime         string
	Model              RegistrationModel
	ScheduleChangeable bool
	// SickCancels is true when an absence reported after the cancellation
	// deadline still cancels the meal.
	SickCancels bool
	Days        []ParticipationDay
}

type ReplaceParticipationSchedule struct {
	StudentID         int64
	GuardianAccountID int64
	Weekdays          []Weekday
}

// SetParticipationWeek registers or cancels the remaining weekdays of the
// week of WeekOf.
type SetParticipationWeek struct {
	StudentID         int64
	GuardianAccountID int64
	WeekOf            Date
	Participating     bool
}

// WeekChange reports which days a week change changed and which it left
// because their deadline had passed.
type WeekChange struct {
	Changed []Date
	Locked  []Date
}

type SetParticipationDay struct {
	StudentID         int64
	GuardianAccountID int64
	Date              Date
	Participating     bool
}

// SchoolChange is a change of a child's registration by the school (#3873).
// The school changes at any time, deadlines included, but never a past or
// closed day.
type SchoolChange struct {
	StudentID      int64
	StaffAccountID int64
	Date           Date
	Participating  bool
}

// SchoolSchedule replaces a child's regular days from EffectiveFrom on, at
// the earliest today. The school uses it after the annual registration.
type SchoolSchedule struct {
	StudentID      int64
	StaffAccountID int64
	Weekdays       []Weekday
	EffectiveFrom  Date
}

type DailyParticipant struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
}

// DailyList is the kitchen's list of a day. Deadline is when the day's
// order is fixed (#3873); CutoffTime is its Berlin clock time.
type DailyList struct {
	Date       Date
	CutoffTime string
	Deadline   time.Time
	// Closed is a closing day or public holiday: nobody eats.
	Closed       bool
	Participants []DailyParticipant
	// Others are the enrolled children who do not eat that day.
	Others []DailyParticipant
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

type engine interface {
	Available(context.Context) (bool, error)
	Week(context.Context, string) ([]Entry, error)
	Replace(context.Context, string, []Dish) error
	Clear(context.Context, string) error
	RegistrationAvailable(context.Context) (bool, error)
	Participation(context.Context, int64, string, string) (ParticipationPlan, error)
	// The empty effective date is a parent's change; a set one is the
	// school's.
	ReplaceParticipationSchedule(ctx context.Context, studentID, accountID int64, weekdays []Weekday, effectiveFrom string) (Date, error)
	SetParticipationDay(ctx context.Context, studentID, accountID int64, date string, participating, bySchool bool) error
	SetParticipationWeek(context.Context, int64, int64, string, bool) (WeekChange, error)
	ClearParticipationDay(ctx context.Context, studentID, accountID int64, date string, bySchool bool) error
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
	date, err := calendar.ParseDate(string(weekStart))
	if err != nil {
		return nil, ErrInvalidMealDate
	}
	monday := date.StartOfISOWeek()
	return m.engine.Week(ctx, monday.String())
}

func (m *Module) ReplaceDay(ctx context.Context, command ReplaceDay) error {
	date, err := calendar.ParseDate(string(command.Date))
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
	parsed, err := calendar.ParseDate(string(date))
	if err != nil {
		return ErrInvalidMealDate
	}
	return m.engine.Clear(ctx, parsed.String())
}

func (m *Module) RegistrationAvailable(ctx context.Context) (bool, error) {
	return m.engine.RegistrationAvailable(ctx)
}

func (m *Module) Participation(ctx context.Context, studentID int64, from, to Date) (ParticipationPlan, error) {
	start, startErr := calendar.ParseDate(string(from))
	end, endErr := calendar.ParseDate(string(to))
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
	return m.engine.ReplaceParticipationSchedule(ctx, command.StudentID, command.GuardianAccountID, weekdays, "")
}

// ReplaceScheduleBySchool sets a child's regular days for the school.
func (m *Module) ReplaceScheduleBySchool(ctx context.Context, command SchoolSchedule) (Date, error) {
	weekdays, ok := normalizeWeekdays(command.Weekdays)
	effectiveFrom, err := calendar.ParseDate(string(command.EffectiveFrom))
	if command.StudentID <= 0 || command.StaffAccountID <= 0 || !ok || err != nil {
		return "", ErrInvalidParticipation
	}
	return m.engine.ReplaceParticipationSchedule(ctx, command.StudentID, command.StaffAccountID, weekdays, effectiveFrom.String())
}

// SetParticipationBySchool registers or cancels a day for the school.
func (m *Module) SetParticipationBySchool(ctx context.Context, command SchoolChange) error {
	date, err := participationDate(command.Date)
	if command.StudentID <= 0 || command.StaffAccountID <= 0 || err != nil {
		return ErrInvalidParticipation
	}
	return m.engine.SetParticipationDay(ctx, command.StudentID, command.StaffAccountID, date.String(), command.Participating, true)
}

// ClearParticipationBySchool drops a day's exception for the school, so the
// regular days apply again.
func (m *Module) ClearParticipationBySchool(ctx context.Context, command SchoolChange) error {
	date, err := participationDate(command.Date)
	if command.StudentID <= 0 || command.StaffAccountID <= 0 || err != nil {
		return ErrInvalidParticipation
	}
	return m.engine.ClearParticipationDay(ctx, command.StudentID, command.StaffAccountID, date.String(), true)
}

func (m *Module) SetParticipationForDay(ctx context.Context, command SetParticipationDay) error {
	date, err := participationDate(command.Date)
	if command.StudentID <= 0 || command.GuardianAccountID <= 0 || err != nil {
		return ErrInvalidParticipation
	}
	return m.engine.SetParticipationDay(ctx, command.StudentID, command.GuardianAccountID, date.String(), command.Participating, false)
}

func (m *Module) SetParticipationForWeek(ctx context.Context, command SetParticipationWeek) (WeekChange, error) {
	date, err := calendar.ParseDate(string(command.WeekOf))
	if command.StudentID <= 0 || command.GuardianAccountID <= 0 || err != nil {
		return WeekChange{}, ErrInvalidParticipation
	}
	return m.engine.SetParticipationWeek(ctx, command.StudentID, command.GuardianAccountID, date.StartOfISOWeek().String(), command.Participating)
}

func (m *Module) ClearParticipationForDay(ctx context.Context, command SetParticipationDay) error {
	date, err := participationDate(command.Date)
	if command.StudentID <= 0 || command.GuardianAccountID <= 0 || err != nil {
		return ErrInvalidParticipation
	}
	return m.engine.ClearParticipationDay(ctx, command.StudentID, command.GuardianAccountID, date.String(), false)
}

func (m *Module) DailyList(ctx context.Context, date Date) (DailyList, error) {
	parsed, err := participationDate(date)
	if err != nil {
		return DailyList{}, ErrInvalidParticipation
	}
	return m.engine.DailyParticipants(ctx, parsed.String())
}

func participationDate(value Date) (calendar.Date, error) {
	date, err := calendar.ParseDate(string(value))
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
