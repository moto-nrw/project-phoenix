// Package mealplan is the public Meal Plan capability.
package mealplan

import (
	"context"
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

type ReplaceDay struct {
	Date   Date
	Dishes []Dish
}

type Fault struct {
	Code string
}

func (f *Fault) Error() string { return f.Code }

var (
	ErrDisabled        = &Fault{Code: "meal_plan_disabled"}
	ErrInvalidMealDate = &Fault{Code: "invalid_meal_date"}
	ErrInvalidDishes   = &Fault{Code: "invalid_dishes"}
)

type engine interface {
	Available(context.Context) (bool, error)
	Week(context.Context, string) ([]Entry, error)
	Replace(context.Context, string, []Dish) error
	Clear(context.Context, string) error
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
