package mealplan_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
)

type recordingEngine struct {
	available bool
	entries   []mealplan.Entry
	replaced  mealplan.ReplaceDay
	cleared   mealplan.Date
}

func (e *recordingEngine) Available(context.Context) (bool, error) { return e.available, nil }
func (e *recordingEngine) Week(context.Context, string) ([]mealplan.Entry, error) {
	return e.entries, nil
}
func (e *recordingEngine) Replace(_ context.Context, date string, dishes []mealplan.Dish) error {
	e.replaced = mealplan.ReplaceDay{Date: mealplan.Date(date), Dishes: dishes}
	return nil
}
func (e *recordingEngine) Clear(context.Context, string) error { return nil }

func TestModuleRejectsWeekendWritesAtPublicSeam(t *testing.T) {
	t.Parallel()

	date, dateErr := mealplan.ParseDate("2026-09-05")
	if dateErr != nil {
		t.Fatal(dateErr)
	}
	module := mealplan.NewModule(&recordingEngine{available: true})
	err := module.ReplaceDay(context.Background(), mealplan.ReplaceDay{
		Date:   date,
		Dishes: []mealplan.Dish{{Dish: "Suppe"}},
	})

	if !errors.Is(err, mealplan.ErrInvalidMealDate) {
		t.Fatalf("ReplaceDay() error = %v, want ErrInvalidMealDate", err)
	}
}

func TestModuleNormalizesDishesAtPublicSeam(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{available: true}
	module := mealplan.NewModule(engine)
	date, dateErr := mealplan.ParseDate("2026-09-07")
	if dateErr != nil {
		t.Fatal(dateErr)
	}
	note := "  vegetarisch  "
	err := module.ReplaceDay(context.Background(), mealplan.ReplaceDay{
		Date: date,
		Dishes: []mealplan.Dish{
			{Dish: "  Nudeln  ", Note: &note},
			{Dish: "   "},
		},
	})
	if err != nil {
		t.Fatalf("ReplaceDay() error = %v", err)
	}
	if len(engine.replaced.Dishes) != 1 || engine.replaced.Dishes[0].Dish != "Nudeln" {
		t.Fatalf("normalized dishes = %#v", engine.replaced.Dishes)
	}
	if engine.replaced.Dishes[0].Note == nil || *engine.replaced.Dishes[0].Note != "vegetarisch" {
		t.Fatalf("normalized note = %#v", engine.replaced.Dishes[0].Note)
	}
}
