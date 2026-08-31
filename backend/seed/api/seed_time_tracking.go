package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"
	"time"
)

// seedTimeTrackingHistoryStep populates Soll/Ist data for the demo tenant
// so the staff detail page (Übersicht KPIs, Dienstplan, Zeiterfassung)
// has realistic numbers to render. Everything goes through the public
// HTTP API so it passes the same validation, authorization and audit
// path that real users do — no DB layer bypassed.
//
// Per staff:
//   - PUT /api/staff/{id}/schedule (admin auth) sets Mo-Fr at 480 min Soll
//   - For each weekday in the trailing 90-day window the staff logs in
//     and walks the live clocking flow (POST check-in, POST check-out)
//     followed by PUT /api/time-tracking/{id} to backdate date + times to
//     the historical day. The audit trail therefore records the seeding
//     just like any other admin correction would.
//   - One staff member receives an approved future 3-day vacation block via
//     the normal vacation request/approval flow. About a quarter of the staff
//     get a single historical sick day via POST /api/time-tracking/absences.
type seedTimeTrackingHistoryStep struct{}

func (seedTimeTrackingHistoryStep) Name() string { return "Seeding time-tracking history" }

const (
	// Three months of trailing history so the demo tenant has enough rows
	// for the cumulative Stundenkonto / Saldo cards to look meaningful.
	// 14 days was too short — admins couldn't see a real Saldo trend.
	timeTrackingDaysBack       = 90
	timeTrackingDailyTargetMin = 480
)

func (seedTimeTrackingHistoryStep) Run(ctx context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil || len(rt.FixedSeeder.staffCredentials) == 0 {
		fmt.Println("  skipping: no staff credentials available")
		return nil
	}

	// Restore the admin auth at the end so downstream steps don't inherit
	// the last staff session.
	originalAuth := rt.TenantAuth
	defer rt.Client.BindAuth(originalAuth)

	staffOrder, staffIDByEmail := buildStaffOrder(rt.FixedSeeder)
	if len(staffOrder) == 0 {
		fmt.Println("  skipping: no staff IDs resolvable from credentials")
		return nil
	}

	scheduleCount, err := seedSchedulesViaAPI(rt, staffOrder, staffIDByEmail)
	if err != nil {
		return err
	}
	todayDate := todaySeedDate()
	if err := seedTimeTrackingCoverage(rt, staffIDByEmail[staffOrder[0].Email], todayDate.Year()); err != nil {
		return err
	}
	// One school-defined Abwesenheitsart (#2403), so the dropdown, the
	// absence list and the exports show the mixed case a real school has:
	// the five standard types plus a name of its own.
	customAbsenceTypeID, err := seedCustomAbsenceType(rt)
	if err != nil {
		return err
	}
	if len(staffOrder) > 1 {
		staffID := staffIDByEmail[staffOrder[1].Email]
		if err := seedCustomAbsenceTypeAllowance(rt, customAbsenceTypeID, staffID, todayDate.Year()); err != nil {
			return err
		}
	}
	vacationApproverAuth, err := loginVacationApprover(rt, staffOrder)
	if err != nil {
		return err
	}

	rng := rand.New(rand.NewPCG(0xC0FFEE, 0xBEEF))
	today := todaySeedDate().UTCMidnight()
	loc := seedBerlinLocation()
	statisticsSupervisorEmail := rt.FixedSeeder.staffCredentials[max(0, len(rt.FixedSeeder.staffCredentials)-2)].Email

	sessionCount := 0
	absenceCount := 0

	for idx, cred := range staffOrder {
		staffID := staffIDByEmail[cred.Email]
		if cred.Position == "Extern" {
			continue
		}
		if err := rt.Client.Login(cred.Email, cred.Password); err != nil {
			return fmt.Errorf("login as %s: %w", cred.Email, err)
		}

		if idx == 0 {
			start := nextWeekday(today.AddDate(0, 0, 1), time.Tuesday)
			if err := requestAndApproveVacation(rt, vacationApproverAuth, start, start.AddDate(0, 0, 2), "Urlaub"); err != nil {
				return fmt.Errorf("seed vacation for staff %d: %w", staffID, err)
			}
			absenceCount++
		}

		var sickDay *time.Time
		if rng.Float64() < 0.25 {
			day := mostRecentWeekday(today.AddDate(0, 0, -rng.IntN(timeTrackingDaysBack)), time.Wednesday)
			sickDay = &day
			if err := postAbsence(rt, day, day, "sick", "Krankmeldung", nil); err != nil {
				return fmt.Errorf("seed sick day for staff %d: %w", staffID, err)
			}
			absenceCount++
		}

		// The second staff member carries the school's own art, on a day the
		// sick draw cannot have taken.
		var customDay *time.Time
		if idx == 1 {
			day := mostRecentWeekday(today.AddDate(0, 0, -7), time.Monday)
			customDay = &day
			if err := postAbsence(rt, day, day, "other", "Regenerationstag", &customAbsenceTypeID); err != nil {
				return fmt.Errorf("seed custom absence for staff %d: %w", staffID, err)
			}
			absenceCount++
		}

		// Walk oldest → today so the live "today's open session" slot is
		// always free when we POST /check-in for the next iteration.
		for offset := timeTrackingDaysBack - 1; offset >= 0; offset-- {
			day := today.AddDate(0, 0, -offset)
			if !shouldSeedTimeTrackingDay(day, today, cred.Email == statisticsSupervisorEmail) {
				continue
			}
			if sickDay != nil && day.Equal(*sickDay) {
				continue
			}
			if customDay != nil && day.Equal(*customDay) {
				continue
			}

			// Every weekday outside of vacation/sick gets a session so the
			// demo data renders as a complete two-week timeline. Random
			// skips made the calendar look broken to first-time viewers.

			created, err := seedSessionViaAPI(rt, rng, day, loc, sessionCount == 0)
			if err != nil {
				return fmt.Errorf("seed session for staff %d on %s: %w", staffID, toDateKey(day), err)
			}
			if created {
				sessionCount++
			}
		}
	}

	fmt.Printf("  %d schedules, %d sessions, %d absences seeded for %d staff\n",
		scheduleCount, sessionCount, absenceCount, len(staffOrder))
	return nil
}

func shouldSeedTimeTrackingDay(day, today time.Time, isStatisticsSupervisor bool) bool {
	if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		return false
	}
	// The earlier statistics step leaves this supervisor with a closed NFC
	// block today. A synthetic 08:00–16:00 app block would overlap it whenever
	// the seed runs during working hours.
	return !isStatisticsSupervisor || toDateKey(day) != toDateKey(today)
}

func buildStaffOrder(fs *FixedSeeder) ([]StaffCredentials, map[string]int64) {
	// Join email → staff id via the personKey ("First Last") that both
	// staffCredentials.Name and staffIDs are keyed by.
	emailToStaff := make(map[string]int64, len(fs.staffCredentials))
	ordered := make([]StaffCredentials, 0, len(fs.staffCredentials))
	for _, cred := range fs.staffCredentials {
		id, ok := fs.staffIDs[cred.Name]
		if !ok {
			continue
		}
		emailToStaff[cred.Email] = id
		ordered = append(ordered, cred)
	}
	// Stable order so the "first staff gets the vacation block" rule is
	// reproducible across runs even if the underlying map iteration shuffles.
	sort.SliceStable(ordered, func(i, j int) bool {
		return emailToStaff[ordered[i].Email] < emailToStaff[ordered[j].Email]
	})
	return ordered, emailToStaff
}

func seedSchedulesViaAPI(rt *Runtime, staff []StaffCredentials, staffIDByEmail map[string]int64) (int, error) {
	rt.Client.BindAuth(rt.TenantAuth)
	entries := make([]map[string]any, 0, 4-0+1)
	for d := 0; d <= 4; d++ {
		entries = append(entries, map[string]any{
			"week_index":     0,
			"day_of_week":    d,
			"target_minutes": timeTrackingDailyTargetMin,
		})
	}

	count := 0
	for _, cred := range staff {
		if cred.Position == "Extern" {
			continue
		}
		staffID := staffIDByEmail[cred.Email]
		path := fmt.Sprintf("/api/staff/%d/schedule", staffID)
		body := map[string]any{
			"mode":            "custom",
			"rotation_length": 1,
			"entries":         entries,
		}
		if _, err := rt.Client.Put(path, body); err != nil {
			return count, fmt.Errorf("put schedule for staff %d: %w", staffID, err)
		}
		count += len(entries)
	}
	return count, nil
}

func loginVacationApprover(rt *Runtime, staff []StaffCredentials) (AuthRef, error) {
	currentAuth := rt.Client.auth
	defer rt.Client.BindAuth(currentAuth)

	for _, cred := range staff {
		if cred.Position != "OGS-Büro" {
			continue
		}
		if err := rt.Client.Login(cred.Email, cred.Password); err != nil {
			return AuthRef{}, fmt.Errorf("login vacation approver %s: %w", cred.Email, err)
		}
		return rt.Client.auth, nil
	}
	return AuthRef{}, fmt.Errorf("no OGS-Büro staff credential available for vacation approval")
}

// seedSessionViaAPI walks the live clocking flow for one historical day:
// open + close a fresh session today, then PUT to backdate it. Returns
// whether a session was created (false when the staff already had an
// open session that we couldn't safely close).
func seedSessionViaAPI(rt *Runtime, rng *rand.Rand, day time.Time, loc *time.Location, withBreak bool) (bool, error) {
	status := "present"
	if rng.Float64() < 0.1 {
		status = "home_office"
	}

	checkInResp, err := rt.Client.Post("/api/time-tracking/check-in", map[string]any{
		"status": status,
	})
	if err != nil {
		return false, fmt.Errorf("post check-in: %w", err)
	}
	sessionID, err := extractSessionID(checkInResp)
	if err != nil {
		return false, fmt.Errorf("parse check-in response: %w", err)
	}
	if withBreak {
		if err := seedOneWorkSessionBreak(rt); err != nil {
			return false, err
		}
	}

	if _, err := rt.Client.Post("/api/time-tracking/check-out", nil); err != nil {
		return false, fmt.Errorf("post check-out: %w", err)
	}

	checkInWall := time.Date(day.Year(), day.Month(), day.Day(), 8, rng.IntN(20)-10, 0, 0, loc)
	checkOutWall := time.Date(day.Year(), day.Month(), day.Day(), 16, rng.IntN(30)-15, 0, 0, loc)
	breakMinutes := 30 + rng.IntN(11) - 5 // 25..35

	// The backend decodes Date as *time.Time (RFC3339), so we send a
	// midnight timestamp in UTC. The service only looks at the date
	// portion when comparing or persisting.
	updateBody := map[string]any{
		"date":           time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"check_in_time":  checkInWall.Format(time.RFC3339),
		"check_out_time": checkOutWall.Format(time.RFC3339),
		"break_minutes":  breakMinutes,
		// A note is mandatory whenever recorded times change while the
		// deviation-reason gate is on (F8). That gate now defaults on (#1844),
		// so the backdate PUT must carry one or the seeder 400s.
		"notes": "Seed-Backdatierung",
	}
	if _, err := rt.Client.Put(fmt.Sprintf("/api/time-tracking/%d", sessionID), updateBody); err != nil {
		return false, fmt.Errorf("put backdate: %w", err)
	}
	return true, nil
}

func seedTimeTrackingCoverage(rt *Runtime, staffID int64, year int) error {
	rt.Client.BindAuth(rt.TenantAuth)
	if _, err := rt.Client.Put(fmt.Sprintf("/api/staff/%d/vacation/quota", staffID), map[string]any{
		"year": year, "entitled_days": 30, "carryover_days": 2,
	}); err != nil {
		return fmt.Errorf("seed vacation quota for staff %d: %w", staffID, err)
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/staff/%d/time-tracking/opening", staffID), map[string]any{
		"effective_date":  todaySeedDate().UTCMidnight().AddDate(0, 0, -1).Format(time.DateOnly),
		"balance_minutes": 600,
		"note":            "Übertrag für die Demo",
	}); err != nil {
		return fmt.Errorf("seed opening balance for staff %d: %w", staffID, err)
	}
	adjustmentRaw, err := rt.Client.Post(fmt.Sprintf("/api/staff/%d/time-tracking/adjustments", staffID), map[string]any{
		"type": "payout", "minutes_delta": -30,
		"effective_date": todaySeedDate().String(),
		"note":           "Korrigierter Demo-Ausgleich",
	})
	if err != nil {
		return fmt.Errorf("seed removable balance adjustment for staff %d: %w", staffID, err)
	}
	adjustmentID, err := parseEnvelopeStringID(adjustmentRaw)
	if err != nil {
		return fmt.Errorf("parse balance adjustment for staff %d: %w", staffID, err)
	}
	if _, err := rt.Client.Delete(fmt.Sprintf("/api/staff/%d/time-tracking/adjustments/%d", staffID, adjustmentID)); err != nil {
		return fmt.Errorf("delete demo balance adjustment for staff %d: %w", staffID, err)
	}
	return nil
}

func seedOneWorkSessionBreak(rt *Runtime) error {
	if _, err := rt.Client.Post("/api/time-tracking/break/start", map[string]any{
		"planned_duration_minutes": 30,
	}); err != nil {
		return fmt.Errorf("start demo work break: %w", err)
	}
	if _, err := rt.Client.Post("/api/time-tracking/break/end", nil); err != nil {
		return fmt.Errorf("end demo work break: %w", err)
	}
	return nil
}

// seedCustomAbsenceType adds the school's own Abwesenheitsart as the admin and
// returns its id. The base type is not sent: the server derives it from the
// art, which is the whole point of the split (#2403).
func seedCustomAbsenceType(rt *Runtime) (int64, error) {
	currentAuth := rt.Client.auth
	defer rt.Client.BindAuth(currentAuth)
	rt.Client.BindAuth(rt.TenantAuth)

	resp, err := rt.Client.Post("/api/absence-types", map[string]any{
		"name":              "Regenerationstag",
		"allowance_enabled": true,
		"overrun_policy":    "warn",
	})
	if err != nil {
		return 0, fmt.Errorf("post absence type: %w", err)
	}
	var payload struct {
		Data struct {
			ID json.RawMessage `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &payload); err != nil {
		return 0, fmt.Errorf("parse absence type response: %w", err)
	}
	// The endpoint sends the id as a decimal STRING — JavaScript cannot
	// represent every BIGINT as a number — so unquote before parsing, and
	// accept a bare number too rather than tying the seeder to that detail.
	raw := strings.Trim(string(payload.Data.ID), `"`)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse absence type id %q: %w", raw, err)
	}
	return id, nil
}

func seedCustomAbsenceTypeAllowance(rt *Runtime, absenceTypeID, staffID int64, year int) error {
	currentAuth := rt.Client.auth
	defer rt.Client.BindAuth(currentAuth)
	rt.Client.BindAuth(rt.TenantAuth)
	if _, err := rt.Client.Put(
		fmt.Sprintf("/api/absence-types/%d/allowances/%d", absenceTypeID, staffID),
		map[string]any{
			"year": year, "entitled_days": 3.5,
			"reason": "Demo-Anspruch",
		},
	); err != nil {
		return fmt.Errorf("seed custom absence type allowance for staff %d: %w", staffID, err)
	}
	return nil
}

func postAbsence(rt *Runtime, dateStart, dateEnd time.Time, absenceType, note string, absenceTypeID *int64) error {
	body := map[string]any{
		"absence_type": absenceType,
		"date_start":   dateStart.Format("2006-01-02"),
		"date_end":     dateEnd.Format("2006-01-02"),
		"note":         note,
	}
	if absenceTypeID != nil {
		body["absence_type_id"] = *absenceTypeID
	}
	if _, err := rt.Client.Post("/api/time-tracking/absences", body); err != nil {
		return err
	}
	return nil
}

func requestAndApproveVacation(rt *Runtime, approverAuth AuthRef, dateStart, dateEnd time.Time, note string) error {
	staffAuth := rt.Client.auth
	defer rt.Client.BindAuth(staffAuth)

	requestBody := map[string]any{
		"date_start": dateStart.Format("2006-01-02"),
		"date_end":   dateEnd.Format("2006-01-02"),
		"note":       note,
	}
	resp, err := rt.Client.Post("/api/time-tracking/vacation/request", requestBody)
	if err != nil {
		return fmt.Errorf("post vacation request: %w", err)
	}
	absenceID, err := extractAbsenceID(resp)
	if err != nil {
		return fmt.Errorf("parse vacation request response: %w", err)
	}

	rt.Client.BindAuth(approverAuth)
	approveBody := map[string]any{
		"decision_note": "Demo-Urlaub automatisch genehmigt",
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/staff/absences/%d/approve", absenceID), approveBody); err != nil {
		return fmt.Errorf("approve vacation request: %w", err)
	}
	return nil
}

func extractSessionID(resp []byte) (int64, error) {
	return extractWrappedID(resp, "session")
}

func extractAbsenceID(resp []byte) (int64, error) {
	return extractWrappedID(resp, "absence")
}

// extractWrappedID reads the id out of a wrapped success payload. The id is
// accepted as a JSON number AND as a quoted decimal string: work sessions
// serialize theirs as a string so an int64 past 2^53 survives JSON.parse in
// the browser (#2402), while the other endpoints still send a number.
// json.Number takes both without a second code path.
func extractWrappedID(resp []byte, entity string) (int64, error) {
	var payload struct {
		Data struct {
			ID json.Number `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &payload); err != nil {
		return 0, err
	}
	if payload.Data.ID == "" {
		return 0, fmt.Errorf("response did not include a %s id", entity)
	}
	id, err := payload.Data.ID.Int64()
	if err != nil {
		return 0, fmt.Errorf("response carried a non-numeric %s id %q: %w", entity, payload.Data.ID, err)
	}
	if id == 0 {
		return 0, fmt.Errorf("response did not include a %s id", entity)
	}
	return id, nil
}

func mostRecentWeekday(from time.Time, target time.Weekday) time.Time {
	delta := int(from.Weekday() - target)
	if delta < 0 {
		delta += 7
	}
	return from.AddDate(0, 0, -delta)
}

func nextWeekday(from time.Time, target time.Weekday) time.Time {
	delta := int(target - from.Weekday())
	if delta < 0 {
		delta += 7
	}
	return from.AddDate(0, 0, delta)
}

func toDateKey(t time.Time) string {
	return t.Format("2006-01-02")
}
