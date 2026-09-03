package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type seedOperationsDemoStep struct{}

func (seedOperationsDemoStep) Name() string { return "Seeding operations demo data" }

func (seedOperationsDemoStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.FixedSeeder == nil {
		return fmt.Errorf("operations demo prerequisites not available")
	}
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if len(staffIDs) == 0 {
		return fmt.Errorf("operations demo staff not available")
	}
	rt.Client.BindAuth(rt.TenantAuth)

	today := todaySeedDate()
	if err := enableMealRegistration(rt); err != nil {
		return err
	}
	if err := seedClosingDay(rt, today); err != nil {
		return err
	}
	if err := seedMealPlan(rt, today); err != nil {
		return err
	}
	if err := seedStaffShift(rt, today, staffIDs[0]); err != nil {
		return err
	}
	fmt.Println("  1 closing day, 1 meal plan and 1 staff shift created")
	return nil
}

func enableMealRegistration(rt *Runtime) error {
	for _, key := range []string{
		"operations.meal_plan_enabled",
		"operations.meal_registration_enabled",
	} {
		if _, err := rt.Client.Put("/api/settings/values/"+key, map[string]any{"value": true}); err != nil {
			return fmt.Errorf("enable %s: %w", key, err)
		}
	}
	return nil
}

func seedClosingDay(rt *Runtime, today seedDate) error {
	closingDay := today.AddDays(40)
	if _, err := rt.Client.Post("/api/timetable/closing-days", map[string]any{
		"start_date": closingDay.String(),
		"end_date":   closingDay.String(),
		"reason":     "Pädagogischer Tag",
	}); err != nil {
		return fmt.Errorf("seed closing day: %w", err)
	}
	return nil
}

func seedMealPlan(rt *Runtime, today seedDate) error {
	if _, err := rt.Client.Put("/api/meal-plan/"+today.String(), map[string]any{
		"dishes": []map[string]any{
			{"dish": "Gemüsenudeln", "note": "Auch ohne Milch erhältlich"},
			{"dish": "Obst und Wasser"},
		},
	}); err != nil {
		return fmt.Errorf("seed meal plan: %w", err)
	}
	return nil
}

func seedStaffShift(rt *Runtime, today seedDate, staffID int64) error {
	shiftTypeID, err := createDefaultShiftTypes(rt)
	if err != nil {
		return err
	}
	shiftDate := nextWeekday(today.UTCMidnight().AddDate(0, 0, 1), time.Monday)
	if _, err := rt.Client.Post("/api/staff-shifts", map[string]any{
		"staff_id":      staffID,
		"date":          shiftDate.Format(seedDateLayout),
		"start_time":    "08:00",
		"end_time":      "16:30",
		"break_minutes": 30,
		"shift_type_id": shiftTypeID,
		"notes":         "Frühdienst und Gruppenbetreuung",
	}); err != nil {
		return fmt.Errorf("seed staff shift: %w", err)
	}
	return nil
}

func createDefaultShiftTypes(rt *Runtime) (int64, error) {
	raw, err := rt.Client.Post("/api/shift-types/defaults", nil)
	if err != nil {
		return 0, fmt.Errorf("create default shift types: %w", err)
	}
	var response struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil || len(response.Data) == 0 || response.Data[0].ID == 0 {
		return 0, fmt.Errorf("parse default shift types response")
	}
	return response.Data[0].ID, nil
}
