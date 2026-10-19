package mealplan_test

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
	weekOf    string
	// effectiveFrom is the school's date of a pattern change; a parent's
	// change leaves it empty.
	effectiveFrom string
	bySchool      bool
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
func (e *recordingEngine) ReplaceParticipationSchedule(_ context.Context, _ int64, _ int64, weekdays []mealplan.Weekday, effectiveFrom string) (mealplan.Date, error) {
	e.effectiveFrom = effectiveFrom
	e.weekdays = weekdays
	return "2026-09-07", nil
}
func (e *recordingEngine) SetParticipationDay(_ context.Context, _ int64, _ int64, _ string, _ bool, bySchool bool) error {
	e.bySchool = bySchool
	return nil
}
func (e *recordingEngine) SetParticipationWeek(_ context.Context, _ int64, _ int64, monday string, _ bool) (mealplan.WeekChange, error) {
	e.weekOf = monday
	return mealplan.WeekChange{}, nil
}
func (e *recordingEngine) ClearParticipationDay(context.Context, int64, int64, string, bool) error {
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
	module := mealplan.NewModule(&recordingEngine{available: true})
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
	module := mealplan.NewModule(engine)
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

	module := mealplan.NewModule(&recordingEngine{available: true})
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

func TestModuleAnchorsWeekChangesOnMonday(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{available: true}
	module := mealplan.NewModule(engine)
	if _, err := module.SetParticipationForWeek(context.Background(), mealplan.SetParticipationWeek{
		StudentID: 42, GuardianAccountID: 7, WeekOf: "2026-09-10", Participating: false,
	}); err != nil {
		t.Fatalf("SetParticipationForWeek() error = %v", err)
	}
	if engine.weekOf != "2026-09-07" {
		t.Fatalf("week anchored on %s, want the Monday 2026-09-07", engine.weekOf)
	}
	for _, command := range []mealplan.SetParticipationWeek{
		{StudentID: 0, GuardianAccountID: 7, WeekOf: "2026-09-10"},
		{StudentID: 42, GuardianAccountID: 0, WeekOf: "2026-09-10"},
		{StudentID: 42, GuardianAccountID: 7, WeekOf: "10.09.2026"},
	} {
		if _, err := module.SetParticipationForWeek(context.Background(), command); !errors.Is(err, mealplan.ErrInvalidParticipation) {
			t.Fatalf("SetParticipationForWeek(%+v) error = %v, want ErrInvalidParticipation", command, err)
		}
	}
}

func TestModuleRoutesSchoolChangesAroundTheDeadlines(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{available: true}
	module := mealplan.NewModule(engine)
	if err := module.SetParticipationBySchool(context.Background(), mealplan.SchoolChange{StudentID: 42, StaffAccountID: 9, Date: "2026-09-10", Participating: true}); err != nil {
		t.Fatalf("SetParticipationBySchool() error = %v", err)
	}
	if !engine.bySchool {
		t.Fatal("a school change must reach the engine as one")
	}
	if err := module.SetParticipationForDay(context.Background(), mealplan.SetParticipationDay{StudentID: 42, GuardianAccountID: 7, Date: "2026-09-10", Participating: true}); err != nil || engine.bySchool {
		t.Fatalf("a parent change must not reach the engine as the school's (err %v)", err)
	}
	if _, err := module.ReplaceScheduleBySchool(context.Background(), mealplan.SchoolSchedule{StudentID: 42, StaffAccountID: 9, Weekdays: []mealplan.Weekday{mealplan.Monday}, EffectiveFrom: "2026-09-14"}); err != nil {
		t.Fatalf("ReplaceScheduleBySchool() error = %v", err)
	}
	if engine.effectiveFrom != "2026-09-14" {
		t.Fatalf("school pattern starts %q, want 2026-09-14", engine.effectiveFrom)
	}
	for _, command := range []mealplan.SchoolSchedule{
		{StudentID: 42, StaffAccountID: 9, Weekdays: []mealplan.Weekday{mealplan.Monday}},
		{StudentID: 42, StaffAccountID: 0, Weekdays: []mealplan.Weekday{mealplan.Monday}, EffectiveFrom: "2026-09-14"},
		{StudentID: 42, StaffAccountID: 9, Weekdays: []mealplan.Weekday{6}, EffectiveFrom: "2026-09-14"},
	} {
		if _, err := module.ReplaceScheduleBySchool(context.Background(), command); !errors.Is(err, mealplan.ErrInvalidParticipation) {
			t.Fatalf("ReplaceScheduleBySchool(%+v) error = %v, want ErrInvalidParticipation", command, err)
		}
	}
}

func (e *recordingEngine) SettleOrders(context.Context) error { return nil }

func (e *recordingEngine) Orders(context.Context, string, string) (mealplan.OrderWeek, error) {
	return mealplan.OrderWeek{}, nil
}
