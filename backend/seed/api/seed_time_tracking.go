package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
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
	vacationApproverAuth, err := loginVacationApprover(rt, staffOrder)
	if err != nil {
		return err
	}

	rng := rand.New(rand.NewPCG(0xC0FFEE, 0xBEEF))
	today := timezone.TodayDate().UTCMidnight()
	loc := timezone.Berlin

	sessionCount := 0
	absenceCount := 0

	for idx, cred := range staffOrder {
		staffID := staffIDByEmail[cred.Email]
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
			if err := postAbsence(rt, day, day, activeModels.AbsenceTypeSick, "Krankmeldung"); err != nil {
				return fmt.Errorf("seed sick day for staff %d: %w", staffID, err)
			}
			absenceCount++
		}

		// Walk oldest → today so the live "today's open session" slot is
		// always free when we POST /check-in for the next iteration.
		for offset := timeTrackingDaysBack - 1; offset >= 0; offset-- {
			day := today.AddDate(0, 0, -offset)
			if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
				continue
			}
			if sickDay != nil && day.Equal(*sickDay) {
				continue
			}

			// Every weekday outside of vacation/sick gets a session so the
			// demo data renders as a complete two-week timeline. Random
			// skips made the calendar look broken to first-time viewers.

			created, err := seedSessionViaAPI(rt, rng, day, loc)
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
	entries := make([]map[string]any, 0, configModels.DayFriday-configModels.DayMonday+1)
	for d := configModels.DayMonday; d <= configModels.DayFriday; d++ {
		entries = append(entries, map[string]any{
			"week_index":     0,
			"day_of_week":    d,
			"target_minutes": timeTrackingDailyTargetMin,
		})
	}

	count := 0
	for _, cred := range staff {
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

func loginVacationApprover(rt *Runtime, staff []StaffCredentials) (phoenixapi.AuthRef, error) {
	currentAuth := rt.Client.auth
	defer rt.Client.BindAuth(currentAuth)

	for _, cred := range staff {
		if cred.Position != "OGS-Büro" {
			continue
		}
		if err := rt.Client.Login(cred.Email, cred.Password); err != nil {
			return phoenixapi.AuthRef{}, fmt.Errorf("login vacation approver %s: %w", cred.Email, err)
		}
		return rt.Client.auth, nil
	}
	return phoenixapi.AuthRef{}, fmt.Errorf("no OGS-Büro staff credential available for vacation approval")
}

// seedSessionViaAPI walks the live clocking flow for one historical day:
// open + close a fresh session today, then PUT to backdate it. Returns
// whether a session was created (false when the staff already had an
// open session that we couldn't safely close).
func seedSessionViaAPI(rt *Runtime, rng *rand.Rand, day time.Time, loc *time.Location) (bool, error) {
	status := activeModels.WorkSessionStatusPresent
	if rng.Float64() < 0.1 {
		status = activeModels.WorkSessionStatusHomeOffice
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

func postAbsence(rt *Runtime, dateStart, dateEnd time.Time, absenceType, note string) error {
	body := map[string]any{
		"absence_type": absenceType,
		"date_start":   dateStart.Format("2006-01-02"),
		"date_end":     dateEnd.Format("2006-01-02"),
		"note":         note,
	}
	if _, err := rt.Client.Post("/api/time-tracking/absences", body); err != nil {
		return err
	}
	return nil
}

func requestAndApproveVacation(rt *Runtime, approverAuth phoenixapi.AuthRef, dateStart, dateEnd time.Time, note string) error {
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

func extractWrappedID(resp []byte, entity string) (int64, error) {
	var payload struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &payload); err != nil {
		return 0, err
	}
	if payload.Data.ID == 0 {
		return 0, fmt.Errorf("response did not include a %s id", entity)
	}
	return payload.Data.ID, nil
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
