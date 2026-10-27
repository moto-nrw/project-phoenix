package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

type seedPlanningDemoStep struct{}

type seedPlanningInstance struct {
	ID              int64  `json:"id"`
	Title           string `json:"title"`
	Status          string `json:"status"`
	ActivityGroupID *int64 `json:"activity_group_id"`
	Staff           []struct {
		StaffID int64 `json:"staff_id"`
	} `json:"staff"`
}

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
	// The duties materialize from Monday like the blocks above, so they come
	// before the later templates, whose rosters start today.
	if err := seedDutyBlocks(rt, categoryID, trackID, staffIDs); err != nil {
		return err
	}
	if err := seedPlanningException(rt); err != nil {
		return err
	}
	if err := seedPickupExtensionDemo(rt, roomID, categoryID, trackID, studentIDs, staffIDs); err != nil {
		return err
	}

	fmt.Printf("  %d phone numbers, %d arrival plans and 7 recurring plans created\n", phoneCount, arrivalCount)
	return nil
}

// seedPlanningException plans the deviations of the coming days; today's
// substitution follows the sick report (seedTodaysAbsencesStep). The second
// staff member is at a Fortbildung on one Frühbetreuung and the colleague in
// the Frühdienst stands in, the last Frühbetreuung of the window is cancelled
// for a Teamfortbildung and the one before it deleted.
func seedPlanningException(rt *Runtime) error {
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if len(staffIDs) <= substituteTodayStaffIndex {
		return fmt.Errorf("planning deviation variants not materialized")
	}
	recurringIDs, deviationID, err := recurringPlanningInstanceIDs(rt, staffIDs[1])
	if err != nil {
		return err
	}
	otherIDs := make([]int64, 0, len(recurringIDs)-1)
	for _, id := range recurringIDs {
		if id != deviationID {
			otherIDs = append(otherIDs, id)
		}
	}
	if len(otherIDs) < 2 {
		return fmt.Errorf("planning deviation variants not materialized")
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/timetable/instances/%d/deviations", otherIDs[len(otherIDs)-1]), map[string]any{
		"cancel": true, "cancel_reason": "Teamfortbildung",
	}); err != nil {
		return fmt.Errorf("cancel one recurring planning instance: %w", err)
	}
	if _, err := rt.Client.Delete(fmt.Sprintf("/api/timetable/instances/%d", otherIDs[len(otherIDs)-2])); err != nil {
		return fmt.Errorf("delete one recurring planning instance: %w", err)
	}
	return seedPlanningStaffDeviation(rt, deviationID, staffIDs[1], staffIDs[substituteTodayStaffIndex])
}

// recurringPlanningInstanceIDs lists the Frühbetreuung from tomorrow on, so
// no deviation planned ahead touches today.
func recurringPlanningInstanceIDs(rt *Runtime, deviationStaffID int64) ([]int64, int64, error) {
	today := todaySeedDate()
	raw, err := rt.Client.Get(fmt.Sprintf("/api/timetable/instances?from=%s&to=%s", today.AddDays(1).String(), today.AddDays(6).String()))
	if err != nil {
		return nil, 0, fmt.Errorf("list recurring planning instances: %w", err)
	}
	var envelope struct {
		Data struct {
			Instances []seedPlanningInstance `json:"instances"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, 0, fmt.Errorf("parse recurring planning instances: %w", err)
	}
	recurringIDs, deviationID := selectRecurringPlanningInstances(envelope.Data.Instances, deviationStaffID)
	if len(recurringIDs) < 3 {
		return nil, 0, fmt.Errorf("fewer than three recurring planning instances materialized")
	}
	if deviationID == 0 {
		return nil, 0, fmt.Errorf("no recurring planning instance belongs to deviation staff")
	}
	return recurringIDs, deviationID, nil
}

func selectRecurringPlanningInstances(instances []seedPlanningInstance, deviationStaffID int64) ([]int64, int64) {
	var recurringIDs []int64
	var deviationID int64
	for _, instance := range instances {
		if instance.ID > 0 && instance.ActivityGroupID != nil && instance.Title == "Frühbetreuung" {
			recurringIDs = append(recurringIDs, instance.ID)
			if deviationID == 0 {
				for _, staff := range instance.Staff {
					if staff.StaffID == deviationStaffID {
						deviationID = instance.ID
						break
					}
				}
			}
		}
	}
	return recurringIDs, deviationID
}

func seedPlanningStaffDeviation(rt *Runtime, instanceID, absentStaffID, substituteStaffID int64) error {
	if _, err := rt.Client.Post(fmt.Sprintf("/api/timetable/instances/%d/deviations", instanceID), map[string]any{
		"absences": []map[string]any{{"staff_id": absentStaffID, "reason": "Fortbildung", "instance_ids": []int64{instanceID}}},
		"substitutions": []map[string]any{{
			"absent_staff_id": absentStaffID, "substitute_staff_id": substituteStaffID,
			"reason": "Vertretung wegen Fortbildung", "instance_ids": []int64{instanceID},
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
		{
			// A recurring weekday note needs no pickup time (#3369): it shows
			// on the tile even on a day the child is not expected.
			path: fmt.Sprintf("/api/students/%d/pickup-notes", studentIDs[1]),
			body: map[string]any{"weekday": 5, "content": "Freitags bei den Großeltern."},
		},
	}
	for _, request := range requests {
		if _, err := rt.Client.Post(request.path, request.body); err != nil {
			return fmt.Errorf("seed care-plan deviation %s: %w", request.path, err)
		}
	}
	// The OGS sets the Abholart of one day itself (#3945), today, so the
	// Kinderkarte and the class day show it on every dev machine. The
	// Abholart changes only on a care day: on a weekend without a weekend
	// plan (the product screenshots) it lands on Monday.
	departurePath := fmt.Sprintf("/api/students/%d/departure-exceptions/%s", studentIDs[0], seedWeekdayOnOrAfter(today).String())
	if _, err := rt.Client.Put(departurePath, map[string]any{
		"departure_mode": "pickup", "reason": "Die Mutter hat angerufen und holt heute selbst ab.",
	}); err != nil {
		return fmt.Errorf("seed care-plan deviation %s: %w", departurePath, err)
	}
	return seedTodayHomeChanges(rt, studentIDs)
}

// seedTodayHomeChanges gives two children a one-day change of their way home,
// so the strip on the Kinderkarte shows on every dev machine (#3946): index 5
// is picked up at 13:30 instead of its Gehzeit, index 4, who goes home alone
// every weekday, is picked up today. Index 0 above shows its day Abholart
// too. On a weekend the changes land on Monday, which has a plan to deviate
// from.
func seedTodayHomeChanges(rt *Runtime, studentIDs []int64) error {
	if len(studentIDs) < 6 {
		return fmt.Errorf("today home change students not available")
	}
	day := seedWeekdayOnOrAfter(todaySeedDate())
	if _, err := rt.Client.Post(fmt.Sprintf("/api/students/%d/pickup-exceptions", studentIDs[5]), map[string]any{
		"exception_date": day.String(),
		"pickup_time":    "13:30",
		"reason":         "Arzttermin",
	}); err != nil {
		return fmt.Errorf("seed today pickup change: %w", err)
	}
	if _, err := rt.Client.Put(fmt.Sprintf("/api/students/%d/departure-exceptions/%s", studentIDs[4], day.String()), map[string]any{
		"departure_mode": "pickup", "reason": "Die Oma holt heute ab.",
	}); err != nil {
		return fmt.Errorf("seed today departure change: %w", err)
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

// seedArrivalSchedules plans when each child arrives: the twelve children of
// the Frühbetreuung (seedPlanningTemplateBody) before school, everybody else
// after the morning lessons, spread over the end of the class periods.
func seedArrivalSchedules(rt *Runtime, studentIDs []int64) (int, error) {
	for index, studentID := range studentIDs {
		arrival := fmt.Sprintf("12:%02d", (index%3)*15)
		if isEarlyCareStudent(index) {
			arrival = fmt.Sprintf("07:%02d", 30+(index%3)*10)
		} else if index%4 == 0 {
			arrival = "11:45"
		}
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

// isEarlyCareStudent reports the twelve children of the Frühbetreuung: the
// first fourteen without Felix Schneider (0) and Baran Amini (4). The product
// screenshots open their pages, and the simulated day brings them at noon, so
// they keep the regular arrival after school.
func isEarlyCareStudent(index int) bool {
	return index < 14 && index != 0 && index != 4
}

// seedPlanningTemplateBody is the Frühbetreuung: the second and third staff
// member, both in the Frühdienst, lead it. On Tuesdays half of the children
// come and the third leads it alone.
func seedPlanningTemplateBody(groupID, roomID, categoryID, trackID int64, studentIDs, staffIDs []int64) (map[string]any, error) {
	selectedStudents := make([]int64, 0, 12)
	for index, studentID := range studentIDs {
		if isEarlyCareStudent(index) {
			selectedStudents = append(selectedStudents, studentID)
		}
	}
	if len(staffIDs) < 3 {
		return nil, fmt.Errorf("recurring planning template needs two staff variants")
	}
	selectedStaff := staffIDs[1:3]
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
	}
	planFromMonday(body, 6)
	return body, nil
}

// planFromMonday starts a template on Monday of today's week and
// materializes it from then until days after today, so the Dienstplan, the
// calendar and the Betreuungsplan show the whole week (Saturday and Sunday
// follow Friday, #3921). The roster is valid from the series start, so the
// past days have their children and staff too.
func planFromMonday(body map[string]any, days int) {
	today := todaySeedDate()
	monday := seedDate{Time: mostRecentWeekday(today.UTCMidnight(), time.Monday)}
	body["start_date"] = monday.String()
	body["materialize_from"], body["materialize_to"] = monday.String(), today.AddDays(days).String()
}

func createSeedTargetVariants(rt *Runtime, roomID, categoryID, trackID int64, studentIDs, staffIDs []int64) error {
	if len(studentIDs) < 10 || len(staffIDs) < 3 {
		return fmt.Errorf("planning target variants need ten students and three staff")
	}
	holidayCareStart, holidayCareEnd := holidayCareDays(todaySeedDate())
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
		{
			// Holiday care runs in the two Herbstferien weeks only and, like
			// any holiday care, on closing days too (#3594). It starts when
			// the Frühbetreuung would end. The weekly materialization plans
			// it once the Ferien come into its window.
			"name": "Ferienbetreuung", "type": "care",
			"target_group_type": "none", "include_closing_days": true,
			"start_date": holidayCareStart.String(), "end_date": holidayCareEnd.String(),
			"weekdays": []int{1, 2, 3, 4, 5}, "start_time": "08:30", "end_time": "12:00",
		},
	}
	for _, body := range variants {
		body["room_id"], body["category_id"], body["planning_track_id"] = roomID, categoryID, trackID
		body["week_pattern"], body["student_ids"], body["staff_ids"] = 0, studentIDs[:10], staffIDs[:3]
		if body["list_kind"] == "learning_time" {
			// The Sternengruppe's own caregiver joins its Lernzeit (#3922).
			body["staff_ids"] = withVisitorStaff(rt.FixedSeeder, staffIDs[:3])
		}
		body["primary_staff_id"] = staffIDs[0]
		if body["name"] != "Ferienbetreuung" {
			planFromMonday(body, 6)
		}
		if _, err := rt.Client.Post("/api/timetable/templates", body); err != nil {
			return fmt.Errorf("create %s planning variant: %w", body["target_group_type"], err)
		}
	}
	return nil
}

// holidayCareDays are the days of the seeded holiday care: the Herbstferien
// (seedHerbstferien), but never after the default school year (1 Aug to
// 31 Jul) that bootstraps the planning period.
func holidayCareDays(today seedDate) (seedDate, seedDate) {
	yearEnd := seedDate{Time: time.Date(today.Year(), time.July, 31, 0, 0, 0, 0, time.UTC)}
	if today.Month() >= time.August {
		yearEnd = seedDate{Time: yearEnd.AddDate(1, 0, 0)}
	}
	first, last := seedHerbstferien(today)
	if last.After(yearEnd.Time) {
		last = yearEnd
	}
	if first.After(last.Time) {
		first = last
	}
	return first, last
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
