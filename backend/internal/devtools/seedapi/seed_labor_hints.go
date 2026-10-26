package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// seedLaborHintsStep turns one past week of one staff member into a week the
// ArbZG hints mark (#4804): more than 48 hours in the week, one day with more
// than 10 hours, one day with too short a break. The other history keeps
// compliant breaks, so the hints stand out instead of covering every row.
//
// The step corrects the blocks the history already wrote, as an admin would,
// through PUT /api/staff/{id}/time-tracking/sessions/{sessionId}. It picks the
// most recent full week before today in which the person worked all five
// weekdays, so a sick day or a holiday never leaves the week short of 48 hours.
type seedLaborHintsStep struct{}

func (seedLaborHintsStep) Name() string { return "Seeding a week with working-time hints" }

// laborHintWeekdays are the corrected blocks of the week, Monday first, as
// Berlin wall clock times. Net work: 10 + 10 + 11 + 10 + 10 = 51 hours.
var laborHintWeekdays = []struct {
	checkIn, checkOut [2]int
	breakMinutes      int
}{
	{checkIn: [2]int{7, 0}, checkOut: [2]int{17, 45}, breakMinutes: 45},
	{checkIn: [2]int{7, 0}, checkOut: [2]int{17, 45}, breakMinutes: 45},
	{checkIn: [2]int{6, 45}, checkOut: [2]int{18, 30}, breakMinutes: 45}, // 11 h
	{checkIn: [2]int{7, 0}, checkOut: [2]int{17, 15}, breakMinutes: 15},  // 45 min needed
	{checkIn: [2]int{7, 0}, checkOut: [2]int{17, 45}, breakMinutes: 45},
}

// laborHintWeeksBack bounds the search for a week without a gap.
const laborHintWeeksBack = 6

func (seedLaborHintsStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil || len(rt.FixedSeeder.staffCredentials) == 0 {
		return fmt.Errorf("staff credentials not available")
	}
	staffOrder, staffIDByEmail, err := resolvedStaffOrder(rt)
	if err != nil {
		return err
	}
	staffID, ok := laborHintStaffID(staffOrder, staffIDByEmail)
	if !ok {
		return nil
	}
	rt.Client.BindAuth(rt.TenantAuth)
	// The latest Monday whose Friday lies before today.
	monday := mostRecentWeekday(todaySeedDate().UTCMidnight().AddDate(0, 0, -5), time.Monday)
	for range laborHintWeeksBack {
		sessionIDs, err := laborHintWeekSessions(rt.Client, staffID, monday)
		if err != nil {
			return err
		}
		if sessionIDs != nil {
			return correctLaborHintWeek(rt.Client, staffID, monday, sessionIDs)
		}
		monday = monday.AddDate(0, 0, -7)
	}
	fmt.Printf("  no full week found for staff %d, working-time hints skipped\n", staffID)
	return nil
}

// laborHintStaffID is the third staff member who is not external: the first
// carries the opening balance and the second the Freizeitausgleich Fridays.
func laborHintStaffID(staffOrder []StaffCredentials, staffIDByEmail map[string]int64) (int64, bool) {
	var candidates []int64
	for _, cred := range staffOrder {
		if cred.Position != "Extern" {
			candidates = append(candidates, staffIDByEmail[cred.Email])
		}
	}
	if len(candidates) == 0 {
		return 0, false
	}
	return candidates[min(2, len(candidates)-1)], true
}

// laborHintWeekSessions returns the block of each weekday Monday to Friday,
// or nil when a day has none or more than one.
func laborHintWeekSessions(client *Client, staffID int64, monday time.Time) ([]int64, error) {
	friday := monday.AddDate(0, 0, 4)
	resp, err := client.Get(fmt.Sprintf("/api/staff/%d/time-tracking/history?from=%s&to=%s", staffID, toDateKey(monday), toDateKey(friday)))
	if err != nil {
		return nil, fmt.Errorf("read history of staff %d: %w", staffID, err)
	}
	var payload struct {
		Data struct {
			Sessions []struct {
				ID   json.Number `json:"id"`
				Date string      `json:"date"`
			} `json:"sessions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &payload); err != nil {
		return nil, fmt.Errorf("decode history of staff %d: %w", staffID, err)
	}
	byDay := make(map[string][]int64)
	for _, session := range payload.Data.Sessions {
		id, err := strconv.ParseInt(session.ID.String(), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("session id %q: %w", session.ID, err)
		}
		day := session.Date
		if len(day) > 10 {
			day = day[:10]
		}
		byDay[day] = append(byDay[day], id)
	}
	ids := make([]int64, 0, len(laborHintWeekdays))
	for offset := range laborHintWeekdays {
		blocks := byDay[toDateKey(monday.AddDate(0, 0, offset))]
		if len(blocks) != 1 {
			return nil, nil
		}
		ids = append(ids, blocks[0])
	}
	return ids, nil
}

func correctLaborHintWeek(client *Client, staffID int64, monday time.Time, sessionIDs []int64) error {
	loc := seedBerlinLocation()
	for offset, plan := range laborHintWeekdays {
		day := monday.AddDate(0, 0, offset)
		checkIn := time.Date(day.Year(), day.Month(), day.Day(), plan.checkIn[0], plan.checkIn[1], 0, 0, loc)
		checkOut := time.Date(day.Year(), day.Month(), day.Day(), plan.checkOut[0], plan.checkOut[1], 0, 0, loc)
		if _, err := client.Put(fmt.Sprintf("/api/staff/%d/time-tracking/sessions/%d", staffID, sessionIDs[offset]), map[string]any{
			"check_in_time":  checkIn.Format(time.RFC3339),
			"check_out_time": checkOut.Format(time.RFC3339),
			"break_minutes":  plan.breakMinutes,
			"notes":          "Elternabend und Teamsitzung",
		}); err != nil {
			return fmt.Errorf("correct session %d of staff %d on %s: %w", sessionIDs[offset], staffID, toDateKey(day), err)
		}
	}
	fmt.Printf("  week of %s for staff %d marked with working-time hints\n", toDateKey(monday), staffID)
	return nil
}
