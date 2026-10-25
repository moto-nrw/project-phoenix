package api

import "fmt"

// Demo duties (#3822): staff tasks without children that take part in
// absences and substitutions. „Essensausgabe“ runs in the Mensa with two
// people required, during lunch and before their Lernzeit at 13:00;
// „Busaufsicht“ watches the bus children waiting in the Aula and lies in the
// Spätdienst of the fourth staff member (dutySeries). The yard would be the
// natural place, but "Schulhof" is the reserved system room that exists only
// with the kiosk Schulhof button (ADR 0019), which the demo leaves off.
func seedDutyBlocks(rt *Runtime, categoryID, trackID int64, staffIDs []int64) error {
	if len(staffIDs) < 4 {
		return fmt.Errorf("duty demo needs four staff")
	}
	mensaID, aulaID := rt.FixedSeeder.roomIDs["Mensa"], rt.FixedSeeder.roomIDs["Aula"]
	if mensaID == 0 || aulaID == 0 {
		return fmt.Errorf("duty demo needs the Mensa and the Aula")
	}
	duties := []map[string]any{
		{
			"name": "Essensausgabe", "start_time": "12:00", "end_time": "13:00", "room_id": mensaID,
			"staff_ids": staffIDs[2:4], "primary_staff_id": staffIDs[2], "required_staff": 2,
		},
		{
			"name": "Busaufsicht", "start_time": "16:00", "end_time": "16:30", "room_id": aulaID,
			"staff_ids": staffIDs[3:4], "primary_staff_id": staffIDs[3], "required_staff": 1,
		},
	}
	for _, body := range duties {
		body["type"], body["target_group_type"] = "duty", "none"
		body["weekdays"], body["week_pattern"] = []int{1, 2, 3, 4, 5}, 0
		body["category_id"], body["planning_track_id"] = categoryID, trackID
		planFromMonday(body, 13)
		if _, err := rt.Client.Post("/api/timetable/templates", body); err != nil {
			return fmt.Errorf("create duty %s: %w", body["name"], err)
		}
	}
	return nil
}
