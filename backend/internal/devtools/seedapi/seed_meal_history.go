package api

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

type PastMealDay struct {
	StudentID int64
	Date      calendar.Date
}

// seedMealHistory fills the last ended month, before its release. Reuse the
// presence planner so the later 30-day room history agrees on arrival and
// departure. Only lunch registrations and attendance before the seed day
// are written; current and future meal schedules stay unchanged.
func seedMealHistory(ctx context.Context, rt *Runtime, seeder *Seeder) error {
	if seeder == nil || seeder.options.MealHistory == nil {
		return nil
	}
	write := presenceHistoryWriterFor(seeder.options, rt)
	if write == nil {
		return fmt.Errorf("mensa history needs the presence history writer")
	}
	today, err := rt.historyToday()
	if err != nil {
		return err
	}
	until := calendar.NewDate(today.Year(), today.Month(), 1)
	from := calendar.NewDate(until.Year(), until.Month()-1, 1)
	input, err := pastCareInputFrom(rt, seeder.definition, until, from)
	if err != nil {
		return err
	}
	input.deviceID, err = seedTerminalID(rt, DemoDevices[0].DeviceID)
	if err != nil {
		return err
	}
	plan := planPresenceHistory(input)
	lunch, _ := rt.Values[regularLunchDaysKey].(map[int64][]int)
	var presence []PastPresenceDay
	var meals []PastMealDay
	for _, day := range plan.days {
		if slices.Contains(lunch[day.StudentID], int(day.Date.Weekday())) {
			presence = append(presence, day)
			meals = append(meals, PastMealDay{StudentID: day.StudentID, Date: day.Date})
		}
	}
	if len(meals) == 0 {
		return fmt.Errorf("mensa history has no registered care days in %s", from)
	}
	if _, err := write(ctx, rt.Bootstrap.SchoolID, presence); err != nil {
		return fmt.Errorf("record Mensa attendance: %w", err)
	}
	if err := seeder.options.MealHistory(ctx, rt.Bootstrap.SchoolID, meals); err != nil {
		return fmt.Errorf("record Mensa registrations: %w", err)
	}
	return nil
}
