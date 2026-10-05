package api

import "fmt"

// Demo duties (#3822): staff tasks without children that take part in
// absences and substitutions. „Essensausgabe“ runs in the Mensa with two
// people required, „Busaufsicht“ has no room at all.
func seedDutyBlocks(rt *Runtime, categoryID, trackID int64, staffIDs []int64) error {
	if len(staffIDs) < 4 {
		return fmt.Errorf("duty demo needs four staff")
	}
	mensaID := rt.FixedSeeder.roomIDs["Mensa"]
	if mensaID == 0 {
		return fmt.Errorf("duty demo needs the Mensa room")
	}
	today := todaySeedDate()
	duties := []map[string]any{
		{
			"name": "Essensausgabe", "start_time": "12:00", "end_time": "13:30", "room_id": mensaID,
			"staff_ids": staffIDs[2:4], "primary_staff_id": staffIDs[2], "required_staff": 2,
		},
		{
			"name": "Busaufsicht", "start_time": "16:00", "end_time": "16:30",
			"staff_ids": staffIDs[1:2], "primary_staff_id": staffIDs[1], "required_staff": 1,
		},
	}
	for _, body := range duties {
		body["type"], body["target_group_type"] = "duty", "none"
		body["weekdays"], body["week_pattern"] = []int{1, 2, 3, 4, 5}, 0
		body["category_id"], body["planning_track_id"] = categoryID, trackID
		body["materialize_from"], body["materialize_to"] = today.String(), today.AddDays(13).String()
		if _, err := rt.Client.Post("/api/timetable/templates", body); err != nil {
			return fmt.Errorf("create duty %s: %w", body["name"], err)
		}
	}
	return nil
}
