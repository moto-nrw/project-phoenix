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
	{{"dish": "Gemüsenudeln", "note": "Auch ohne Milch erhältlich", "allergens": []string{"gluten", "eggs", "milk"}}, {"dish": "Obst und Wasser", "note": "Obst aus der Region"}},
	{{"dish": "Hähnchen mit Reis und Erbsen", "note": "Vegetarisch: Gemüsebratling", "allergens": []string{"celery"}}},
	{{"dish": "Kartoffelsuppe mit Brötchen", "note": "Vegetarisch", "allergens": []string{"gluten", "celery", "milk"}}, {"dish": "Joghurt mit Beeren", "note": "Laktosefrei auf Nachfrage", "allergens": []string{"milk"}}},
	{{"dish": "Spaghetti Bolognese", "note": "Vegetarisch: Linsen-Bolognese", "allergens": []string{"gluten", "celery"}}},
	{{"dish": "Fischstäbchen mit Kartoffelpüree", "note": "Vegetarisch: Gemüsestäbchen", "allergens": []string{"gluten", "fish", "milk"}}, {"dish": "Obst", "note": "Äpfel und Bananen"}},
	{{"dish": "Milchreis mit Zimt und Kirschen", "note": "Laktosefrei auf Nachfrage", "allergens": []string{"milk"}}},
	{{"dish": "Gemüsecurry mit Reis", "note": "Vegan, leicht scharf", "allergens": []string{"mustard", "nuts"}}, {"dish": "Apfelschnitze", "note": "Aus der Region"}},
	{{"dish": "Linseneintopf mit Würstchen", "note": "Vegetarisch: ohne Würstchen", "allergens": []string{"celery", "mustard", "sulphites"}}},
	{{"dish": "Gemüselasagne", "note": "Vegetarisch", "allergens": []string{"gluten", "eggs", "milk"}}, {"dish": "Apfelkompott", "note": "Ohne Zuckerzusatz"}},
	{{"dish": "Pfannkuchen mit Apfelmus", "note": "Vegetarisch", "allergens": []string{"gluten", "eggs", "milk"}}, {"dish": "Rohkost", "note": "Gurke, Möhre und Paprika"}},
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
		"start_time":    lateDutyStart,
		"end_time":      lateDutyEnd,
		"break_minutes": regularDutyBreakMinutes,
		"shift_type_id": shiftTypes[0].ID,
		"notes":         "Gruppenbetreuung und Freies Spiel",
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

// seedDutyTypes are the Schichtarten the Dienstplan shows on its chips: the
// default for time with the children and the school's own Früh- and
// Spätdienst.
type seedDutyTypes struct {
	care, early, late int64
}

// seedDutyShiftTypes returns the Schichtarten of the seeded Dienste. The
// defaults endpoint is idempotent and lists every Schichtart of the school,
// so the Früh- and Spätdienst are created only when it does not list them
// yet: the series step and the week step both ask.
func seedDutyShiftTypes(rt *Runtime) (seedDutyTypes, error) {
	shiftTypes, err := createDefaultShiftTypes(rt)
	if err != nil {
		return seedDutyTypes{}, err
	}
	types := seedDutyTypes{care: shiftTypes[0].ID}
	for _, shiftType := range shiftTypes {
		switch {
		case strings.HasPrefix(shiftType.Name, "Betreuung"):
			types.care = shiftType.ID
		case shiftType.Name == "Frühdienst":
			types.early = shiftType.ID
		case shiftType.Name == "Spätdienst":
			types.late = shiftType.ID
		}
	}
	for _, missing := range []struct {
		id          *int64
		name, color string
	}{{&types.early, "Frühdienst", "#F59E0B"}, {&types.late, "Spätdienst", "#8B5CF6"}} {
		if *missing.id != 0 {
			continue
		}
		raw, err := rt.Client.Post("/api/shift-types/", map[string]any{"name": missing.name, "color": missing.color, "is_active": true})
		if err != nil {
			return seedDutyTypes{}, fmt.Errorf("create shift type %s: %w", missing.name, err)
		}
		if *missing.id, err = decodeSeedEntityID(raw); err != nil || *missing.id == 0 {
			return seedDutyTypes{}, fmt.Errorf("parse shift type %s response", missing.name)
		}
	}
	return types, nil
}
