package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	fmt.Println("  1 closing day, 2 weeks of meal plan and 1 staff shift created")
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

// demoMeals is lunch for two weeks, Monday to Friday. The tenant meal plan
// and the parents portal open on the current week, and the parents portal
// also offers the next one (#3894).
var demoMeals = [10][]map[string]any{
	{{"dish": "Gemüsenudeln", "note": "Auch ohne Milch erhältlich"}, {"dish": "Obst und Wasser"}},
	{{"dish": "Hähnchen mit Reis und Erbsen", "note": "Vegetarisch: Gemüsebratling"}},
	{{"dish": "Kartoffelsuppe mit Brötchen"}, {"dish": "Joghurt mit Beeren"}},
	{{"dish": "Spaghetti Bolognese", "note": "Vegetarisch: Linsen-Bolognese"}},
	{{"dish": "Fischstäbchen mit Kartoffelpüree", "note": "Vegetarisch: Gemüsestäbchen"}, {"dish": "Obst"}},
	{{"dish": "Milchreis mit Zimt und Kirschen"}},
	{{"dish": "Gemüsecurry mit Reis"}, {"dish": "Apfelschnitze"}},
	{{"dish": "Linseneintopf mit Würstchen", "note": "Vegetarisch: ohne Würstchen"}},
	{{"dish": "Gemüselasagne"}, {"dish": "Apfelkompott"}},
	{{"dish": "Pfannkuchen mit Apfelmus"}, {"dish": "Rohkost"}},
}

// seedMealPlan publishes lunch for every weekday of the week today lies in
// and of the week after it. On a weekend the current week is the one that
// just ended; that is the week the meal plan opens on.
func seedMealPlan(rt *Runtime, today seedDate) error {
	monday := seedDate{Time: mostRecentWeekday(today.Time, time.Monday)}
	for index, dishes := range demoMeals {
		day := monday.AddDays(index + 2*(index/5)).String()
		if _, err := rt.Client.Put("/api/meal-plan/"+day, map[string]any{"dishes": dishes}); err != nil {
			return fmt.Errorf("seed meal plan %s: %w", day, err)
		}
	}
	return nil
}

func seedStaffShift(rt *Runtime, today seedDate, staffID int64) error {
	shiftTypes, err := createDefaultShiftTypes(rt)
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
		"shift_type_id": shiftTypes[0].ID,
		"notes":         "Frühdienst und Gruppenbetreuung",
	}); err != nil {
		return fmt.Errorf("seed staff shift: %w", err)
	}
	// A second Schichtart in the same week, so the Dienstplan's hours per
	// Schichtart (#3819) show a split instead of a single figure. The defaults
	// come back sorted by name, so the Verfügungszeit is found by its name.
	preparationID := int64(0)
	for _, shiftType := range shiftTypes {
		if strings.Contains(shiftType.Name, "Verfügung") {
			preparationID = shiftType.ID
		}
	}
	if preparationID == 0 {
		return nil
	}
	if _, err := rt.Client.Post("/api/staff-shifts", map[string]any{
		"staff_id":      staffID,
		"date":          shiftDate.AddDate(0, 0, 1).Format(seedDateLayout),
		"start_time":    "13:00",
		"end_time":      "15:00",
		"break_minutes": 0,
		"shift_type_id": preparationID,
		"notes":         "Vorbereitung und Dienstversammlung",
	}); err != nil {
		return fmt.Errorf("seed second staff shift: %w", err)
	}
	return nil
}

// seedShiftType is one default Schichtart as the defaults endpoint returns it.
type seedShiftType struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func createDefaultShiftTypes(rt *Runtime) ([]seedShiftType, error) {
	raw, err := rt.Client.Post("/api/shift-types/defaults", nil)
	if err != nil {
		return nil, fmt.Errorf("create default shift types: %w", err)
	}
	var response struct {
		Data []seedShiftType `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil || len(response.Data) == 0 || response.Data[0].ID == 0 {
		return nil, fmt.Errorf("parse default shift types response")
	}
	return response.Data, nil
}
