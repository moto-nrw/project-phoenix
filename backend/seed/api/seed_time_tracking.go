package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"
	"sync"
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
//     (several staff members in parallel) and walks the live clocking flow (POST check-in, POST check-out)
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
	// Caps how many staff histories are written at the same time.
	timeTrackingWorkers = 8
)

func (seedTimeTrackingHistoryStep) Run(ctx context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil || len(rt.FixedSeeder.staffCredentials) == 0 {
		return fmt.Errorf("staff credentials not available")
	}

	// Restore the admin auth at the end so downstream steps don't inherit
	// the last staff session.
	originalAuth := rt.TenantAuth
	defer rt.Client.BindAuth(originalAuth)

	staffOrder, staffIDByEmail := buildStaffOrder(rt.FixedSeeder)
	if len(staffOrder) != len(rt.FixedSeeder.staffCredentials) {
		return fmt.Errorf("resolved %d of %d staff IDs from credentials", len(staffOrder), len(rt.FixedSeeder.staffCredentials))
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
	customAbsenceTypeID, err := seedCustomAbsenceType(rt, "Regenerationstag")
	if err != nil {
		return err
	}
	// The other Kontingente of a school under the TVöD SuE (#3256): each staff
	// member has an own claim per art, booked by the Leitung directly.
	sickLeaveTypeID, err := seedCustomAbsenceType(rt, "Krank-Urlaubstag")
	if err != nil {
		return err
	}
	// Krank-Urlaubstage stay usable until 31.03. of the following year
	// (#3257); the other arts keep the default and expire on 31.12.
	if err := seedAbsenceTypeCarryover(rt, sickLeaveTypeID, "03-31"); err != nil {
		return err
	}
	conversionTypeID, err := seedCustomAbsenceType(rt, "Umwandlungstag")
	if err != nil {
		return err
	}
	vacationApproverAuth, err := loginVacationApprover(rt, staffOrder)
	if err != nil {
		return err
	}

	plan := timeTrackingHistoryPlan{
		today:                     todaySeedDate().UTCMidnight(),
		loc:                       seedBerlinLocation(),
		statisticsSupervisorEmail: rt.FixedSeeder.staffCredentials[max(0, len(rt.FixedSeeder.staffCredentials)-2)].Email,
		vacationApproverAuth:      vacationApproverAuth,
		customAbsenceTypeID:       customAbsenceTypeID,
		breakStaffIdx:             -1,
		compTimeDays:              map[string]bool{},
	}
	// Wissingen (#3258): a colleague took Fridays off to use up
	// Krank-Urlaubstage before that allowance existed, so the Leitung entered
	// them as Freizeitausgleich. The second staff member carries the last
	// three past Fridays like that.
	// The Stundenkonto starts on 1 January, and Freizeitausgleich before it
	// is rejected, so an early-January run keeps only this year's Fridays.
	lastFriday := mostRecentWeekday(plan.today.AddDate(0, 0, -1), time.Friday)
	var compTimeFridays []time.Time
	for weeksBack := 2; weeksBack >= 0; weeksBack-- {
		friday := lastFriday.AddDate(0, 0, -7*weeksBack)
		if friday.Year() == plan.today.Year() {
			compTimeFridays = append(compTimeFridays, friday)
			plan.compTimeDays[toDateKey(friday)] = true
		}
	}
	for idx, cred := range staffOrder {
		if cred.Position != "Extern" {
			plan.breakStaffIdx = idx
			break
		}
	}

	// Each staff member clocks in on an own client with an own token, so the
	// histories run in parallel. No history depends on another one.
	sessionCounts := make([]int, len(staffOrder))
	absenceCounts := make([]int, len(staffOrder))
	errs := make([]error, len(staffOrder))
	slots := make(chan struct{}, timeTrackingWorkers)
	var wg sync.WaitGroup
	for idx, cred := range staffOrder {
		if cred.Position == "Extern" {
			continue
		}
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			client := NewClientWithAdapter(rt.Adapter, rt.Verbose)
			sessionCounts[idx], absenceCounts[idx], errs[idx] = plan.seedStaff(client, idx, cred, staffIDByEmail[cred.Email])
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return err
	}
	sessionCount, absenceCount := 0, 0
	for idx := range staffOrder {
		sessionCount += sessionCounts[idx]
		absenceCount += absenceCounts[idx]
	}
	if len(staffOrder) > 1 {
		staffID := staffIDByEmail[staffOrder[1].Email]
		claims := []struct {
			typeID int64
			days   float64
			reason string
		}{
			{customAbsenceTypeID, 3.5, "Demo-Anspruch"},
			{sickLeaveTypeID, 10, "Krank in den Sommerferien"},
			{conversionTypeID, 2, "Bei der Gemeinde beantragt"},
		}
		for _, claim := range claims {
			if err := seedCustomAbsenceTypeAllowance(rt, claim.typeID, staffID, todayDate.Year(), claim.days, claim.reason); err != nil {
				return err
			}
		}
		// A rest from the previous year that nobody used: from April on the
		// Vorjahr card shows it as expired instead of dropping it (#3257).
		if err := seedCustomAbsenceTypeAllowance(rt, sickLeaveTypeID, staffID, todayDate.Year()-1, 4, "Krank in den Herbstferien"); err != nil {
			return err
		}
		// Booked by the Leitung without a request: one Krank-Urlaubstag and
		// one vacation day, both still ahead.
		sickLeaveDay := nextWeekday(plan.today.AddDate(0, 0, 1), time.Thursday)
		if err := postStaffAbsence(rt, staffID, sickLeaveDay, "other", "Mündlich abgesprochen", &sickLeaveTypeID); err != nil {
			return fmt.Errorf("seed direct Krank-Urlaubstag for staff %d: %w", staffID, err)
		}
		vacationDay := nextWeekday(plan.today.AddDate(0, 0, 1), time.Friday)
		if err := postStaffAbsence(rt, staffID, vacationDay, "vacation", "Mündlich abgesprochen", nil); err != nil {
			return fmt.Errorf("seed direct vacation for staff %d: %w", staffID, err)
		}
		absenceCount += 2
		rebooked, err := seedCompTimeFridays(rt, staffID, compTimeFridays, sickLeaveTypeID)
		if err != nil {
			return err
		}
		absenceCount += rebooked
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

type timeTrackingHistoryPlan struct {
	today                     time.Time
	loc                       *time.Location
	statisticsSupervisorEmail string
	vacationApproverAuth      AuthRef
	customAbsenceTypeID       int64
	// breakStaffIdx is the staff member whose first session carries a break.
	breakStaffIdx int
	// compTimeDays are the days the second staff member stays at home on
	// Freizeitausgleich (#3258), so no session is written for them.
	compTimeDays map[string]bool
}

// seedStaff logs in as one staff member and writes that person's absences and
// session history. It returns the number of sessions and absences created.
func (p timeTrackingHistoryPlan) seedStaff(client *Client, idx int, cred StaffCredentials, staffID int64) (int, int, error) {
	if err := client.Login(cred.Email, cred.Password); err != nil {
		return 0, 0, fmt.Errorf("login as %s: %w", cred.Email, err)
	}
	// One seed per staff member keeps the data reproducible whatever order
	// the workers run in.
	rng := rand.New(rand.NewPCG(0xC0FFEE, 0xBEEF+uint64(idx)))
	sessions, absences := 0, 0

	if idx == 0 {
		start := nextWeekday(p.today.AddDate(0, 0, 1), time.Tuesday)
		if err := requestAndApproveVacation(client, p.vacationApproverAuth, start, start.AddDate(0, 0, 2), "Urlaub"); err != nil {
			return sessions, absences, fmt.Errorf("seed vacation for staff %d: %w", staffID, err)
		}
		absences++
	}

	var sickDay *time.Time
	if rng.Float64() < 0.25 {
		day := mostRecentWeekday(p.today.AddDate(0, 0, -rng.IntN(timeTrackingDaysBack)), time.Wednesday)
		sickDay = &day
		if err := postAbsence(client, day, day, "sick", "Krankmeldung", nil); err != nil {
			return sessions, absences, fmt.Errorf("seed sick day for staff %d: %w", staffID, err)
		}
		absences++
	}

	// The second staff member carries the school's own art, on a day the
	// sick draw cannot have taken.
	var customDay *time.Time
	if idx == 1 {
		day := mostRecentWeekday(p.today.AddDate(0, 0, -7), time.Monday)
		customDay = &day
		if err := postAbsence(client, day, day, "other", "Regenerationstag", &p.customAbsenceTypeID); err != nil {
			return sessions, absences, fmt.Errorf("seed custom absence for staff %d: %w", staffID, err)
		}
		absences++
	}

	// Walk oldest → today so the live "today's open session" slot is
	// always free when we POST /check-in for the next iteration.
	for offset := timeTrackingDaysBack - 1; offset >= 0; offset-- {
		day := p.today.AddDate(0, 0, -offset)
		if !shouldSeedTimeTrackingDay(day, p.today, cred.Email == p.statisticsSupervisorEmail) {
			continue
		}
		if sickDay != nil && day.Equal(*sickDay) {
			continue
		}
		if customDay != nil && day.Equal(*customDay) {
			continue
		}
		if idx == 1 && p.compTimeDays[toDateKey(day)] {
			continue
		}

		// Every weekday outside of vacation/sick gets a session so the
		// demo data renders as a complete two-week timeline. Random
		// skips made the calendar look broken to first-time viewers.

		withBreak := idx == p.breakStaffIdx && sessions == 0
		created, err := seedSessionViaAPI(client, rng, day, p.loc, withBreak)
		if err != nil {
			return sessions, absences, fmt.Errorf("seed session for staff %d on %s: %w", staffID, toDateKey(day), err)
		}
		if created {
			sessions++
		}
	}
	return sessions, absences, nil
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
func seedSessionViaAPI(client *Client, rng *rand.Rand, day time.Time, loc *time.Location, withBreak bool) (bool, error) {
	status := "present"
	if rng.Float64() < 0.1 {
		status = "home_office"
	}

	checkInResp, err := client.Post("/api/time-tracking/check-in", map[string]any{
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
		if err := seedOneWorkSessionBreak(client); err != nil {
			return false, err
		}
	}

	if _, err := client.Post("/api/time-tracking/check-out", nil); err != nil {
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
	if _, err := client.Put(fmt.Sprintf("/api/time-tracking/%d", sessionID), updateBody); err != nil {
		return false, fmt.Errorf("put backdate: %w", err)
	}
	return true, nil
}

func seedTimeTrackingCoverage(rt *Runtime, staffID int64, year int) error {
	rt.Client.BindAuth(rt.TenantAuth)
	if _, err := rt.Client.Put(fmt.Sprintf("/api/staff/%d/vacation/quota", staffID), map[string]any{
		"year": year, "entitled_days": 30, "carryover_days": 2,
		"reason": "Tariflicher Jahresurlaub",
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
	// Sonderarbeitszeit (#3259): a holiday-care week in the future with 8.5h
	// per day, so the Arbeitszeitmodell tab and the daily table show one.
	careStart := nextWeekday(todaySeedDate().UTCMidnight().AddDate(0, 0, 14), time.Monday)
	if _, err := rt.Client.Post(fmt.Sprintf("/api/staff/%d/target-overrides", staffID), map[string]any{
		"start_date":    toDateKey(careStart),
		"end_date":      toDateKey(careStart.AddDate(0, 0, 4)),
		"daily_minutes": 510,
	}); err != nil {
		return fmt.Errorf("seed target override for staff %d: %w", staffID, err)
	}
	return nil
}

func seedOneWorkSessionBreak(client *Client) error {
	if _, err := client.Post("/api/time-tracking/break/start", map[string]any{
		"planned_duration_minutes": 30,
	}); err != nil {
		return fmt.Errorf("start demo work break: %w", err)
	}
	if _, err := client.Post("/api/time-tracking/break/end", nil); err != nil {
		return fmt.Errorf("end demo work break: %w", err)
	}
	return nil
}

// seedCustomAbsenceType adds the school's own Abwesenheitsart as the admin and
// returns its id. The base type is not sent: the server derives it from the
// art, which is the whole point of the split (#2403).
func seedCustomAbsenceType(rt *Runtime, name string) (int64, error) {
	currentAuth := rt.Client.auth
	defer rt.Client.BindAuth(currentAuth)
	rt.Client.BindAuth(rt.TenantAuth)

	resp, err := rt.Client.Post("/api/absence-types", map[string]any{
		"name": name,
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

func seedAbsenceTypeCarryover(rt *Runtime, absenceTypeID int64, until string) error {
	currentAuth := rt.Client.auth
	defer rt.Client.BindAuth(currentAuth)
	rt.Client.BindAuth(rt.TenantAuth)
	if _, err := rt.Client.Put(
		fmt.Sprintf("/api/absence-types/%d", absenceTypeID),
		map[string]any{"allowance_enabled": true, "carryover_until": until},
	); err != nil {
		return fmt.Errorf("seed absence type carryover: %w", err)
	}
	return nil
}

func seedCustomAbsenceTypeAllowance(rt *Runtime, absenceTypeID, staffID int64, year int, days float64, reason string) error {
	currentAuth := rt.Client.auth
	defer rt.Client.BindAuth(currentAuth)
	rt.Client.BindAuth(rt.TenantAuth)
	if _, err := rt.Client.Put(
		fmt.Sprintf("/api/absence-types/%d", absenceTypeID),
		map[string]any{"allowance_enabled": true},
	); err != nil {
		return fmt.Errorf("enable custom absence type allowance: %w", err)
	}
	if _, err := rt.Client.Put(
		fmt.Sprintf("/api/absence-types/%d/allowances/%d", absenceTypeID, staffID),
		map[string]any{
			"year": year, "entitled_days": days,
			"reason": reason,
		},
	); err != nil {
		return fmt.Errorf("seed custom absence type allowance for staff %d: %w", staffID, err)
	}
	return nil
}

func postAbsence(client *Client, dateStart, dateEnd time.Time, absenceType, note string, absenceTypeID *int64) error {
	body := map[string]any{
		"absence_type": absenceType,
		"date_start":   dateStart.Format("2006-01-02"),
		"date_end":     dateEnd.Format("2006-01-02"),
		"note":         note,
	}
	if absenceTypeID != nil {
		body["absence_type_id"] = *absenceTypeID
	}
	if _, err := client.Post("/api/time-tracking/absences", body); err != nil {
		return err
	}
	return nil
}

// postStaffAbsence books one day for staffID as the Leitung
// (POST /api/staff/{id}/absences), without a request (#3256).
func postStaffAbsence(rt *Runtime, staffID int64, day time.Time, absenceType, note string, absenceTypeID *int64) error {
	currentAuth := rt.Client.auth
	defer rt.Client.BindAuth(currentAuth)
	rt.Client.BindAuth(rt.TenantAuth)
	body := map[string]any{
		"absence_type": absenceType,
		"date_start":   day.Format("2006-01-02"),
		"date_end":     day.Format("2006-01-02"),
		"note":         note,
	}
	if absenceTypeID != nil {
		body["absence_type_id"] = *absenceTypeID
	}
	_, err := rt.Client.Post(fmt.Sprintf("/api/staff/%d/absences", staffID), body)
	return err
}

// seedCompTimeFridays books the Fridays as Freizeitausgleich and rebooks the
// oldest one into the Krank-Urlaubstag allowance (#3258). The other Fridays
// stay for the Leitung to rebook, and the audit log shows one rebooking with
// its reason. It returns the number of absences it created.
func seedCompTimeFridays(rt *Runtime, staffID int64, fridays []time.Time, sickLeaveTypeID int64) (int, error) {
	currentAuth := rt.Client.auth
	defer rt.Client.BindAuth(currentAuth)
	rt.Client.BindAuth(rt.TenantAuth)
	if len(fridays) == 0 {
		return 0, nil
	}
	ids := make([]string, 0, len(fridays))
	for _, friday := range fridays {
		resp, err := rt.Client.Post(fmt.Sprintf("/api/staff/%d/absences", staffID), map[string]any{
			"absence_type": "comp_time",
			"date_start":   toDateKey(friday),
			"date_end":     toDateKey(friday),
			"note":         "Freitag frei",
		})
		if err != nil {
			return 0, fmt.Errorf("seed comp_time Friday for staff %d: %w", staffID, err)
		}
		id, err := extractAbsenceID(resp)
		if err != nil {
			return 0, err
		}
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	if _, err := rt.Client.Post(fmt.Sprintf("/api/staff/%d/absences/rebook", staffID), map[string]any{
		"absence_ids":     ids[:1],
		"absence_type":    "other",
		"absence_type_id": strconv.FormatInt(sickLeaveTypeID, 10),
		"reason":          "Kontingent angelegt, der Freitag war ein Krank-Urlaubstag",
	}); err != nil {
		return 0, fmt.Errorf("seed absence rebooking for staff %d: %w", staffID, err)
	}
	return len(ids), nil
}

func requestAndApproveVacation(client *Client, approverAuth AuthRef, dateStart, dateEnd time.Time, note string) error {
	requestBody := map[string]any{
		"date_start": dateStart.Format("2006-01-02"),
		"date_end":   dateEnd.Format("2006-01-02"),
		"note":       note,
	}
	resp, err := client.Post("/api/time-tracking/vacation/request", requestBody)
	if err != nil {
		return fmt.Errorf("post vacation request: %w", err)
	}
	absenceID, err := extractAbsenceID(resp)
	if err != nil {
		return fmt.Errorf("parse vacation request response: %w", err)
	}

	approveBody := map[string]any{
		"decision_note": "Demo-Urlaub automatisch genehmigt",
	}
	if _, err := client.PostWithAuth(approverAuth, fmt.Sprintf("/api/staff/absences/%d/approve", absenceID), approveBody); err != nil {
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
