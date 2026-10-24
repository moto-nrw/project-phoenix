package application

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
)

type recordingEngine struct {
	available bool
	entries   []mealplan.Entry
	replaced  mealplan.ReplaceDay
	weekdays  []mealplan.Weekday
}

func (e *recordingEngine) Available(context.Context) (bool, error) { return e.available, nil }
func (e *recordingEngine) Week(context.Context, string) ([]mealplan.Entry, error) {
	return e.entries, nil
}
func (e *recordingEngine) Replace(_ context.Context, date string, dishes []mealplan.Dish) error {
	e.replaced = mealplan.ReplaceDay{Date: mealplan.Date(date), Dishes: dishes}
	return nil
}
func (e *recordingEngine) Clear(context.Context, string) error                 { return nil }
func (e *recordingEngine) RegistrationAvailable(context.Context) (bool, error) { return true, nil }
func (e *recordingEngine) Participation(context.Context, int64, string, string) (mealplan.ParticipationPlan, error) {
	return mealplan.ParticipationPlan{}, nil
}
func (e *recordingEngine) ReplaceParticipationSchedule(_ context.Context, _ int64, _ int64, weekdays []mealplan.Weekday) (mealplan.Date, error) {
	e.weekdays = weekdays
	return "2026-09-07", nil
}
func (e *recordingEngine) SetParticipationDay(context.Context, int64, int64, string, bool) error {
	return nil
}
func (e *recordingEngine) ClearParticipationDay(context.Context, int64, int64, string) error {
	return nil
}
func (e *recordingEngine) DailyParticipants(context.Context, string) (mealplan.DailyList, error) {
	return mealplan.DailyList{}, nil
}

func TestModuleRejectsWeekendWritesAtPublicSeam(t *testing.T) {
	t.Parallel()

	date, dateErr := mealplan.ParseDate("2026-09-05")
	if dateErr != nil {
		t.Fatal(dateErr)
	}
	module := newTestModule(&recordingEngine{available: true})
	err := module.ReplaceDay(context.Background(), mealplan.ReplaceDay{
		Date:   date,
		Dishes: []mealplan.Dish{{Dish: "Suppe"}},
	})

	if !errors.Is(err, mealplan.ErrInvalidMealDate) {
		t.Fatalf("ReplaceDay() error = %v, want ErrInvalidMealDate", err)
	}
}

func TestModuleNormalizesRegularParticipationWeekdays(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{available: true}
	module := newTestModule(engine)
	effectiveFrom, err := module.ReplaceParticipationSchedule(context.Background(), mealplan.ReplaceParticipationSchedule{
		StudentID: 42, GuardianAccountID: 7,
		Weekdays: []mealplan.Weekday{mealplan.Friday, mealplan.Monday, mealplan.Friday},
	})

	if err != nil {
		t.Fatalf("ReplaceParticipationSchedule() error = %v", err)
	}
	if effectiveFrom != "2026-09-07" {
		t.Fatalf("effective from = %s", effectiveFrom)
	}
	want := []mealplan.Weekday{mealplan.Monday, mealplan.Friday}
	if !slices.Equal(engine.weekdays, want) {
		t.Fatalf("weekdays = %v, want %v", engine.weekdays, want)
	}
}

func TestModuleRejectsWeekendParticipationOverride(t *testing.T) {
	t.Parallel()

	module := newTestModule(&recordingEngine{available: true})
	err := module.SetParticipationForDay(context.Background(), mealplan.SetParticipationDay{
		StudentID: 42, GuardianAccountID: 7, Date: "2026-09-05", Participating: true,
	})

	if !errors.Is(err, mealplan.ErrInvalidParticipation) {
		t.Fatalf("SetParticipationForDay() error = %v, want ErrInvalidParticipation", err)
	}
}

func TestModuleNormalizesDishesAtPublicSeam(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{available: true}
	module := newTestModule(engine)
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

func TestModuleNormalizesAllergensAtPublicSeam(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{available: true}
	module := newTestModule(engine)
	err := module.ReplaceDay(context.Background(), mealplan.ReplaceDay{
		Date: "2026-09-07",
		Dishes: []mealplan.Dish{
			{Dish: "Fischstäbchen", Allergens: []mealplan.Allergen{mealplan.AllergenMilk, mealplan.AllergenFish, mealplan.AllergenGluten, mealplan.AllergenFish}},
			{Dish: "Obst"},
		},
	})
	if err != nil {
		t.Fatalf("ReplaceDay() error = %v", err)
	}
	want := []mealplan.Allergen{mealplan.AllergenGluten, mealplan.AllergenFish, mealplan.AllergenMilk}
	if got := engine.replaced.Dishes[0].Allergens; !slices.Equal(got, want) {
		t.Fatalf("normalized allergens = %v, want %v", got, want)
	}
	if got := engine.replaced.Dishes[1].Allergens; got == nil || len(got) != 0 {
		t.Fatalf("unmarked dish allergens = %#v, want an empty list", got)
	}
}

func TestModuleRejectsUnknownAllergensBeforeWriting(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{available: true}
	module := newTestModule(engine)
	err := module.ReplaceDay(context.Background(), mealplan.ReplaceDay{
		Date: "2026-09-07",
		// A blank dish does not hide an unknown code either.
		Dishes: []mealplan.Dish{{Dish: " ", Allergens: []mealplan.Allergen{"strawberry"}}},
	})
	if !errors.Is(err, mealplan.ErrInvalidAllergens) {
		t.Fatalf("ReplaceDay() error = %v, want ErrInvalidAllergens", err)
	}
	if engine.replaced.Date != "" {
		t.Fatalf("an invalid day reached the store: %#v", engine.replaced)
	}
}
