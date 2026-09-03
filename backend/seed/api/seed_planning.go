package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
)

type seedPlanningDemoStep struct{}

func (seedPlanningDemoStep) Name() string { return "Seeding planning demo data" }

func (seedPlanningDemoStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.FixedSeeder == nil || rt.Client == nil {
		return fmt.Errorf("planning demo prerequisites not available")
	}
	fs := rt.FixedSeeder
	groupID := fs.groupIDs["sternengruppe"]
	roomID := fs.roomIDs["OGS-Raum 1"]
	categoryID := fs.categoryIDs["Gruppenraum"]
	studentIDs := orderedSeedStudentIDs(fs)
	staffIDs := orderedSeedStaffIDs(fs)
	if groupID == 0 || roomID == 0 || categoryID == 0 || len(studentIDs) == 0 || len(staffIDs) == 0 {
		return fmt.Errorf("planning demo references not available")
	}

	phoneCount, err := seedGuardianPhoneNumbers(rt)
	if err != nil {
		return err
	}
	arrivalCount, err := seedArrivalSchedules(rt, studentIDs)
	if err != nil {
		return err
	}
	if err := seedCarePlanDeviations(rt, studentIDs); err != nil {
		return err
	}
	if _, err := rt.Client.Post("/api/timetable/periods/bootstrap", nil); err != nil {
		return fmt.Errorf("bootstrap planning periods: %w", err)
	}
	trackID, err := createSeedPlanningTrack(rt)
	if err != nil {
		return err
	}
	if err := createSeedPlanningTemplate(rt, groupID, roomID, categoryID, trackID, studentIDs, staffIDs); err != nil {
		return err
	}
	if err := createSeedTargetVariants(rt, roomID, categoryID, trackID, studentIDs, staffIDs); err != nil {
		return err
	}
	if err := seedPlanningException(rt); err != nil {
		return err
	}

	fmt.Printf("  %d phone numbers, %d arrival plans and 3 recurring plans created\n", phoneCount, arrivalCount)
	return nil
}

func seedPlanningException(rt *Runtime) error {
	recurringIDs, err := recurringPlanningInstanceIDs(rt)
	if err != nil {
		return err
	}
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if len(recurringIDs) < 3 || len(staffIDs) < 4 {
		return fmt.Errorf("planning deviation variants not materialized")
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/timetable/instances/%d/deviations", recurringIDs[0]), map[string]any{
		"cancel": true, "cancel_reason": "Teamfortbildung",
	}); err != nil {
		return fmt.Errorf("cancel one recurring planning instance: %w", err)
	}
	if _, err := rt.Client.Delete(fmt.Sprintf("/api/timetable/instances/%d", recurringIDs[1])); err != nil {
		return fmt.Errorf("delete one recurring planning instance: %w", err)
	}
	return seedPlanningStaffDeviation(rt, recurringIDs[2], staffIDs[1], staffIDs[3])
}

func recurringPlanningInstanceIDs(rt *Runtime) ([]int64, error) {
	today := todaySeedDate()
	raw, err := rt.Client.Get(fmt.Sprintf("/api/timetable/instances?from=%s&to=%s", today.String(), today.AddDays(6).String()))
	if err != nil {
		return nil, fmt.Errorf("list recurring planning instances: %w", err)
	}
	var envelope struct {
		Data struct {
			Instances []struct {
				ID              int64  `json:"id"`
				Title           string `json:"title"`
				ActivityGroupID *int64 `json:"activity_group_id"`
			} `json:"instances"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parse recurring planning instances: %w", err)
	}
	var recurringIDs []int64
	for _, instance := range envelope.Data.Instances {
		if instance.ID > 0 && instance.ActivityGroupID != nil && instance.Title == "Frühbetreuung" {
			recurringIDs = append(recurringIDs, instance.ID)
		}
	}
	if len(recurringIDs) < 2 {
		return nil, fmt.Errorf("fewer than two recurring planning instances materialized")
	}
	return recurringIDs, nil
}

func seedPlanningStaffDeviation(rt *Runtime, instanceID, absentStaffID, substituteStaffID int64) error {
	if _, err := rt.Client.Post(fmt.Sprintf("/api/timetable/instances/%d/deviations", instanceID), map[string]any{
		"absences": []map[string]any{{"staff_id": absentStaffID, "reason": "Fortbildung", "instance_ids": []int64{instanceID}}},
		"substitutions": []map[string]any{{
			"absent_staff_id": absentStaffID, "substitute_staff_id": substituteStaffID,
			"reason": "Vertretung für die Demo", "instance_ids": []int64{instanceID},
		}},
	}); err != nil {
		return fmt.Errorf("seed planning staff deviation: %w", err)
	}
	return nil
}

func seedCarePlanDeviations(rt *Runtime, studentIDs []int64) error {
	if len(studentIDs) < 2 {
		return fmt.Errorf("care-plan deviation students not available")
	}
	today := todaySeedDate()
	requests := []struct {
		path string
		body map[string]any
	}{
		{
			path: fmt.Sprintf("/api/students/%d/arrival-exceptions", studentIDs[0]),
			body: map[string]any{"exception_date": today.AddDays(2).String(), "expected_arrival": "09:15", "reason": "Arzttermin"},
		},
		{
			path: fmt.Sprintf("/api/students/%d/arrival-notes", studentIDs[0]),
			body: map[string]any{"note_date": today.AddDays(2).String(), "content": "Kommt nach dem Arzttermin."},
		},
		{
			path: fmt.Sprintf("/api/students/%d/pickup-exceptions", studentIDs[1]),
			body: map[string]any{"exception_date": today.AddDays(3).String(), "pickup_time": "14:15", "reason": "Sporttermin"},
		},
		{
			path: fmt.Sprintf("/api/students/%d/pickup-notes", studentIDs[1]),
			body: map[string]any{"note_date": today.AddDays(3).String(), "content": "Heute holt die Tante das Kind ab."},
		},
	}
	for _, request := range requests {
		if _, err := rt.Client.Post(request.path, request.body); err != nil {
			return fmt.Errorf("seed care-plan deviation %s: %w", request.path, err)
		}
	}
	return nil
}

func seedGuardianPhoneNumbers(rt *Runtime) (int, error) {
	ids := make([]int64, 0, len(rt.FixedSeeder.guardianIDs))
	for _, id := range rt.FixedSeeder.guardianIDs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for index, id := range ids {
		if _, err := rt.Client.Post(fmt.Sprintf("/api/guardians/%d/phone-numbers", id), map[string]any{
			"phone_number": fmt.Sprintf("+49 221 555%04d", index+1),
			"phone_type":   "mobile",
			"is_primary":   true,
		}); err != nil {
			return index, fmt.Errorf("seed phone number for guardian %d: %w", id, err)
		}
	}
	return len(ids), nil
}

func seedArrivalSchedules(rt *Runtime, studentIDs []int64) (int, error) {
	for index, studentID := range studentIDs {
		arrival := fmt.Sprintf("%02d:%02d", 7+(index%2), 30+(index%3)*10)
		schedules := make([]map[string]any, 0, 5)
		for weekday := 1; weekday <= 5; weekday++ {
			schedules = append(schedules, map[string]any{
				"weekday": weekday, "expected_arrival": arrival,
			})
		}
		if _, err := rt.Client.Put(fmt.Sprintf("/api/students/%d/arrival-schedules", studentID), map[string]any{
			"schedules": schedules,
		}); err != nil {
			return index, fmt.Errorf("seed arrival plan for student %d: %w", studentID, err)
		}
	}
	return len(studentIDs), nil
}

func createSeedPlanningTrack(rt *Runtime) (int64, error) {
	raw, err := rt.Client.Post("/api/timetable/planning-tracks", map[string]any{
		"name": "Randzeiten", "color": "#5080D8", "sort_order": 0,
	})
	if err != nil {
		return 0, fmt.Errorf("create planning track: %w", err)
	}
	var response struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil || response.Data.ID == 0 {
		return 0, fmt.Errorf("parse planning track response")
	}
	return response.Data.ID, nil
}

func createSeedPlanningTemplate(
	rt *Runtime,
	groupID, roomID, categoryID, trackID int64,
	studentIDs, staffIDs []int64,
) error {
	body, err := seedPlanningTemplateBody(groupID, roomID, categoryID, trackID, studentIDs, staffIDs)
	if err != nil {
		return err
	}
	if _, err := rt.Client.Post("/api/timetable/templates", body); err != nil {
		return fmt.Errorf("create recurring planning template: %w", err)
	}
	return nil
}

func seedPlanningTemplateBody(groupID, roomID, categoryID, trackID int64, studentIDs, staffIDs []int64) (map[string]any, error) {
	today := todaySeedDate()
	selectedStudents := studentIDs
	if len(selectedStudents) > 12 {
		selectedStudents = selectedStudents[:12]
	}
	selectedStaff := staffIDs
	if len(selectedStaff) > 3 {
		selectedStaff = selectedStaff[:3]
	}
	if len(selectedStaff) < 2 {
		return nil, fmt.Errorf("recurring planning template needs two staff variants")
	}
	tuesdayStudents := make([]int64, 0, (len(selectedStudents)+1)/2)
	for index, studentID := range selectedStudents {
		if index%2 == 0 {
			tuesdayStudents = append(tuesdayStudents, studentID)
		}
	}
	body := map[string]any{
		"name":               "Frühbetreuung",
		"type":               "care",
		"list_kind":          "edge_hours",
		"weekdays":           []int{1, 2, 3, 4, 5},
		"start_time":         "07:30",
		"end_time":           "08:30",
		"room_id":            roomID,
		"category_id":        categoryID,
		"planning_track_id":  trackID,
		"education_group_id": groupID,
		"target_group_type":  "gruppe",
		"targets": []map[string]any{
			{"type": "gruppe", "education_group_id": groupID},
		},
		"week_pattern":     0,
		"student_ids":      selectedStudents,
		"staff_ids":        selectedStaff,
		"primary_staff_id": selectedStaff[0],
		"weekday_assignments": []map[string]any{
			{"weekday": 2, "staff_ids": selectedStaff[1:], "student_ids": tuesdayStudents, "primary_staff_id": selectedStaff[1]},
		},
		"materialize_from": today.String(),
		"materialize_to":   today.AddDays(6).String(),
	}
	return body, nil
}

func createSeedTargetVariants(rt *Runtime, roomID, categoryID, trackID int64, studentIDs, staffIDs []int64) error {
	if len(studentIDs) < 10 || len(staffIDs) < 3 {
		return fmt.Errorf("planning target variants need ten students and three staff")
	}
	today := todaySeedDate()
	variants := []map[string]any{
		{
			"name": "Lernzeit Jahrgang 1", "type": "care", "list_kind": "learning_time",
			"target_group_type": "jahrgang", "target_grade_level": 1,
			"targets":  []map[string]any{{"type": "jahrgang", "grade_level": 1}},
			"weekdays": []int{1, 3, 5}, "start_time": "13:00", "end_time": "14:00",
		},
		{
			"name": "AG Klasse 1a", "type": "activity", "list_kind": "activity",
			"target_group_type": "klasse", "target_school_class": "Klasse 1a",
			"targets":  []map[string]any{{"type": "klasse", "school_class": "Klasse 1a"}},
			"weekdays": []int{2, 4}, "start_time": "14:00", "end_time": "15:00",
		},
	}
	for _, body := range variants {
		body["room_id"], body["category_id"], body["planning_track_id"] = roomID, categoryID, trackID
		body["week_pattern"], body["student_ids"], body["staff_ids"] = 0, studentIDs[:10], staffIDs[:3]
		body["primary_staff_id"] = staffIDs[0]
		body["materialize_from"], body["materialize_to"] = today.String(), today.AddDays(6).String()
		if _, err := rt.Client.Post("/api/timetable/templates", body); err != nil {
			return fmt.Errorf("create %s planning variant: %w", body["target_group_type"], err)
		}
	}
	return nil
}

func orderedSeedStudentIDs(fs *FixedSeeder) []int64 {
	indices := make([]int, 0, len(fs.studentIDByIndex))
	for index := range fs.studentIDByIndex {
		indices = append(indices, index)
	}
	slices.Sort(indices)
	ids := make([]int64, 0, len(indices))
	for _, index := range indices {
		ids = append(ids, fs.studentIDByIndex[index])
	}
	return ids
}

func orderedSeedStaffIDs(fs *FixedSeeder) []int64 {
	ids := make([]int64, 0, len(fs.staffIDs))
	for _, staff := range DemoStaff {
		id := fs.staffIDs[fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)]
		if id != 0 {
			ids = append(ids, id)
		}
	}
	return ids
}
