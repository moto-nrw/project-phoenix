package api

import (
	"encoding/json"
	"fmt"
	"time"
)

// Demo data for later pickup times without a block (#3261). "Freies Spiel"
// is the afternoon block with children from the call, "Teamsitzung" an office
// appointment without children that must never be offered. Two children stay
// longer than before: one on a single day (day task), one on every Thursday
// (weekday task). Both show up as to-dos on the home page.
const (
	seedFreePlayTitle       = "Freies Spiel"
	seedFreePlayEnd         = "17:00"
	seedLaterPickupDayIndex = 7
	seedLaterPickupWeekIdx  = 9
)

func seedPickupExtensionDemo(rt *Runtime, roomID, categoryID, trackID int64, studentIDs, staffIDs []int64) error {
	if len(studentIDs) < 16 || len(staffIDs) < 2 {
		return fmt.Errorf("pickup extension demo needs sixteen students and two staff")
	}
	today := todaySeedDate()
	templates := []map[string]any{
		{
			"name": seedFreePlayTitle, "type": "care", "start_time": "14:45", "end_time": seedFreePlayEnd,
			"student_ids": studentIDs[10:16], "staff_ids": staffIDs[:1], "primary_staff_id": staffIDs[0],
		},
		{
			"name": "Teamsitzung", "type": "external", "start_time": "15:00", "end_time": "15:45",
			"staff_ids": staffIDs[1:2], "primary_staff_id": staffIDs[1],
		},
	}
	for _, body := range templates {
		body["weekdays"], body["week_pattern"], body["target_group_type"] = []int{1, 2, 3, 4, 5}, 0, "none"
		body["room_id"], body["category_id"], body["planning_track_id"] = roomID, categoryID, trackID
		// Two weeks, so the pending parent request a week out has blocks too.
		body["materialize_from"], body["materialize_to"] = today.String(), today.AddDays(13).String()
		if _, err := rt.Client.Post("/api/timetable/templates", body); err != nil {
			return fmt.Errorf("create %s template: %w", body["name"], err)
		}
	}
	if err := seedLaterDayPickup(rt, today); err != nil {
		return err
	}
	return seedLaterWeekdayPickup(rt)
}

// seedLaterDayPickup moves one day of a child with pickup pattern 2 (Tue
// 15:30, Wed 14:00, Fri 15:00) to 16:30, so the extra time overlaps
// "Freies Spiel" and the child is on no block for it.
func seedLaterDayPickup(rt *Runtime, today seedDate) error {
	studentID, ok := rt.FixedSeeder.studentIDByIndex[seedLaterPickupDayIndex]
	if !ok {
		return fmt.Errorf("later pickup day student not available")
	}
	date := today.AddDays(1)
	for !isSeedWeekday(date, time.Tuesday, time.Wednesday, time.Friday) {
		date = date.AddDays(1)
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/students/%d/pickup-exceptions", studentID), map[string]any{
		"exception_date": date.String(), "pickup_time": "16:30", "reason": "Spielt länger mit Freunden",
	}); err != nil {
		return fmt.Errorf("seed later day pickup: %w", err)
	}
	return nil
}

// seedLaterWeekdayPickup moves Thursday of a child with pickup pattern 4
// from 15:30 to 16:30 for good, like the Tuesday change from the call. The
// weekly editor replaces the whole week, so the other days are sent as is.
func seedLaterWeekdayPickup(rt *Runtime) error {
	studentID, ok := rt.FixedSeeder.studentIDByIndex[seedLaterPickupWeekIdx]
	if !ok {
		return fmt.Errorf("later pickup weekday student not available")
	}
	schedules := []map[string]any{
		{"weekday": 1, "pickup_time": "15:00"},
		{"weekday": 2, "pickup_time": "15:30"},
		{"weekday": 3, "pickup_time": "16:00", "notes": "Papa holt ab"},
		{"weekday": 4, "pickup_time": "16:30", "notes": "Donnerstags länger im Ganztag"},
		{"weekday": 5, "pickup_time": "14:00", "notes": "Fußballtraining"},
	}
	if _, err := rt.Client.Put(fmt.Sprintf("/api/students/%d/pickup-schedules", studentID), map[string]any{
		"schedules": schedules,
	}); err != nil {
		return fmt.Errorf("seed later weekday pickup: %w", err)
	}
	return nil
}

// seedPendingLaterPickupChange files an open one-day request that keeps a
// child until 17:00. Approving it in the requests overview opens the block
// choice (#3261). It takes the first demo parent whose child has a regular
// pickup before that time on a weekday a week or more out; the day skips
// the one the decided demo request already uses.
func (s parentEnrollmentSeedStep) seedPendingLaterPickupChange(rt *Runtime, adminAuth AuthRef, parents []ParentCredentials, parentAuths map[string]AuthRef) error {
	from, to := todaySeedDate().AddDays(8), todaySeedDate().AddDays(13)
	for _, parent := range parents {
		parentAuth, ok := parentAuths[parent.Email]
		if !ok || len(parent.StudentIDs) == 0 {
			continue
		}
		studentID := parent.StudentIDs[0]
		raw, err := rt.Client.GetWithAuth(adminAuth, fmt.Sprintf("/api/students/%d/pickup-schedules?from=%s&to=%s", studentID, from.String(), to.String()))
		if err != nil {
			return fmt.Errorf("load pickup plan for pending later pickup: %w", err)
		}
		date, found, err := firstDayPickedUpBefore(raw, seedFreePlayEnd)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		if _, err := rt.Client.PostWithAuth(parentAuth, fmt.Sprintf("/parent/me/children/%d/care-exception", studentID), map[string]any{
			"date":        date,
			"pickup_time": seedFreePlayEnd,
			"reason":      "Möchte länger mit den Freunden spielen",
		}); err != nil {
			return fmt.Errorf("create pending later pickup request: %w", err)
		}
		return nil
	}
	// Without a regular pickup there is no "longer than before", so a
	// request would open no choice. Say so instead of failing the run.
	fmt.Println("  Kein offener Antrag für spätere Abholung: kein Elternkind mit passender Abholzeit")
	return nil
}

func firstDayPickedUpBefore(raw []byte, limit string) (string, bool, error) {
	var envelope struct {
		Data struct {
			EffectiveSchedules []struct {
				Date     string `json:"date"`
				Schedule *struct {
					PickupTime string `json:"pickup_time"`
				} `json:"schedule"`
			} `json:"effective_schedules"`
			Exceptions []struct {
				ExceptionDate string `json:"exception_date"`
			} `json:"exceptions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", false, fmt.Errorf("parse pickup plan: %w", err)
	}
	taken := make(map[string]bool, len(envelope.Data.Exceptions))
	for _, exception := range envelope.Data.Exceptions {
		taken[exception.ExceptionDate] = true
	}
	for _, day := range envelope.Data.EffectiveSchedules {
		if day.Schedule != nil && day.Schedule.PickupTime < limit && !taken[day.Date] {
			return day.Date, true, nil
		}
	}
	return "", false, nil
}

func isSeedWeekday(date seedDate, weekdays ...time.Weekday) bool {
	for _, weekday := range weekdays {
		if date.Weekday() == weekday {
			return true
		}
	}
	return false
}
