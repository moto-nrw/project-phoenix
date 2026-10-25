package api

import (
	"encoding/json"
	"fmt"
	"time"
)

// Past vacation of the time-tracking history: without it every colleague
// still had all 30 days in October and the history showed no Urlaub at all.
// Most staff took three to four weeks in the summer, staggered by a week so
// the Ferienbetreuung kept people; the Leitung books it directly, the way a
// school enters approved vacation after the fact (#3256).
const (
	// pastVacationEarliest keeps the vacation clear of the break block on the
	// first working day of the window (breakSessionDay).
	pastVacationEarliest = 75
	// pastVacationLatest keeps it clear of the recent Freizeitausgleich
	// Fridays and the Regenerationstag of the second staff member.
	pastVacationLatest = 28
)

// pastVacationWorkdays is how many working days a staff member took, by
// index modulo three: 30 entitled days leave 10, 13 and 12.
var pastVacationWorkdays = [...]int{20, 17, 18}

// pastVacation returns the first and last day of the summer vacation of the
// staff member at idx, or false for nobody. The first staff member carries
// the opening balance and a vacation still ahead, and every fifth one saved
// the days. A block that would reach into the previous year is left out: its
// days would count against a quota nobody seeded.
func pastVacation(today time.Time, idx int) (time.Time, time.Time, bool) {
	if idx == 0 || idx%5 == 4 {
		return time.Time{}, time.Time{}, false
	}
	start := nextWeekday(today.AddDate(0, 0, -pastVacationEarliest), time.Monday).AddDate(0, 0, 7*(idx%3))
	workdays := pastVacationWorkdays[idx%3] - 1
	last := start.AddDate(0, 0, 7*(workdays/5)+workdays%5)
	if start.Year() != today.Year() || !last.Before(today.AddDate(0, 0, -pastVacationLatest)) {
		return time.Time{}, time.Time{}, false
	}
	return start, last, true
}

// seedStaffWithVacation writes the history of one staff member with the
// summer vacation in it: the days of the vacation get no session, then the
// Leitung books it.
func (p timeTrackingHistoryPlan) seedStaffWithVacation(client, admin *Client, idx int, cred StaffCredentials, staffID int64) (int, int, error) {
	first, last, ok := pastVacation(p.today, idx)
	if !ok {
		return p.seedStaff(client, admin, idx, cred, staffID)
	}
	staffPlan := p
	staffPlan.holidays = make(map[string]bool, len(p.holidays))
	for day := range p.holidays {
		staffPlan.holidays[day] = true
	}
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		staffPlan.holidays[toDateKey(day)] = true
	}
	sessions, absences, err := staffPlan.seedStaff(client, admin, idx, cred, staffID)
	if err != nil {
		return sessions, absences, err
	}
	booked, err := bookPastVacation(admin, staffID, first, last)
	if err != nil {
		return sessions, absences, fmt.Errorf("seed past vacation for staff %d: %w", staffID, err)
	}
	return sessions, absences + booked, nil
}

// bookPastVacation books first to last as vacation, as the Leitung. Direct
// vacation may not overlap another absence, and the history may have drawn a
// sick day inside the block, so the block is split around the days already
// taken. It returns the number of absences it created.
func bookPastVacation(admin *Client, staffID int64, first, last time.Time) (int, error) {
	taken, err := takenAbsenceDays(admin, staffID, first, last)
	if err != nil {
		return 0, err
	}
	booked := 0
	for _, part := range freeVacationRanges(first, last, taken) {
		if _, err := admin.Post(fmt.Sprintf("/api/staff/%d/absences", staffID), map[string]any{
			"absence_type": "vacation",
			"date_start":   toDateKey(part[0]),
			"date_end":     toDateKey(part[1]),
			"note":         "Sommerurlaub",
		}); err != nil {
			return booked, err
		}
		booked++
	}
	return booked, nil
}

// takenAbsenceDays reads the days between first and last that already carry
// an absence of the staff member.
func takenAbsenceDays(admin *Client, staffID int64, first, last time.Time) (map[string]bool, error) {
	raw, err := admin.Get(fmt.Sprintf("/api/staff/%d/absences?from=%s&to=%s", staffID, toDateKey(first), toDateKey(last)))
	if err != nil {
		return nil, fmt.Errorf("read absences: %w", err)
	}
	var response struct {
		Data []struct {
			DateStart string `json:"date_start"`
			DateEnd   string `json:"date_end"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("parse absences: %w", err)
	}
	taken := map[string]bool{}
	for _, absence := range response.Data {
		start, startErr := time.Parse(time.DateOnly, absence.DateStart)
		end, endErr := time.Parse(time.DateOnly, absence.DateEnd)
		if startErr != nil || endErr != nil {
			return nil, fmt.Errorf("parse absence range %q to %q", absence.DateStart, absence.DateEnd)
		}
		for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
			taken[toDateKey(day)] = true
		}
	}
	return taken, nil
}

// freeVacationRanges splits first to last at the taken days into the ranges
// that still contain a working day. Weekends stay inside a range, as in a
// vacation someone enters by hand.
func freeVacationRanges(first, last time.Time, taken map[string]bool) [][2]time.Time {
	var ranges [][2]time.Time
	var start time.Time
	open := false
	flush := func(end time.Time) {
		if open && hasWeekday(start, end) {
			ranges = append(ranges, [2]time.Time{start, end})
		}
		open = false
	}
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		if taken[toDateKey(day)] {
			flush(day.AddDate(0, 0, -1))
			continue
		}
		if !open {
			start, open = day, true
		}
	}
	flush(last)
	return ranges
}

func hasWeekday(first, last time.Time) bool {
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		if day.Weekday() != time.Saturday && day.Weekday() != time.Sunday {
			return true
		}
	}
	return false
}
