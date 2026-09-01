package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// seedAnnouncementsStep creates demo announcements via the operator API.
type seedAnnouncementsStep struct{}

func (seedAnnouncementsStep) Name() string { return "Seeding announcements" }

func (seedAnnouncementsStep) Run(ctx context.Context, rt *Runtime) error {
	// Switch to operator auth for announcements
	rt.Client.BindAuth(rt.OperatorAuth)
	defer rt.Client.BindAuth(rt.TenantAuth) // restore tenant auth

	announcements := []map[string]any{
		{
			"title":    "Willkommen im neuen Schuljahr 2026/27",
			"content":  "Liebe Kolleginnen und Kollegen, wir freuen uns auf ein neues Schuljahr! Bitte überprüft eure Dienstpläne und meldet euch bei Fragen im OGS-Büro.",
			"type":     "announcement",
			"severity": "info",
		},
		{
			"title":    "Wartungsarbeiten am Freitag, 11.04.",
			"content":  "Am Freitag zwischen 18:00 und 20:00 Uhr wird das System für Wartungsarbeiten kurzzeitig nicht verfügbar sein. Bitte plant entsprechend.",
			"type":     "maintenance",
			"severity": "warning",
		},
		{
			"title":    "Neue Funktion: Abholzeiten jetzt im System",
			"content":  "Ab sofort können Abholzeiten pro Kind und Wochentag hinterlegt werden. Die Zeiten werden auf dem RFID-Terminal angezeigt, wenn ein Kind abgeholt wird.",
			"type":     "release",
			"severity": "info",
		},
	}

	for _, a := range announcements {
		if _, err := rt.Client.Post("/operator/announcements", a); err != nil {
			fmt.Printf("  WARNING: failed to create announcement %q: %v\n", a["title"], err)
			continue
		}
	}

	fmt.Printf("  %d announcements created\n", len(announcements))
	return nil
}

// seedPrivacyConsentsStep creates privacy consent records for all students.
type seedPrivacyConsentsStep struct{}

func (seedPrivacyConsentsStep) Name() string { return "Seeding privacy consents" }

func (seedPrivacyConsentsStep) Run(ctx context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("fixed seeder not available")
	}

	count := 0
	for i, student := range DemoStudents {
		studentID, ok := rt.FixedSeeder.studentIDByIndex[i]
		if !ok {
			continue
		}
		_ = student // used only for index

		_, err := rt.Client.Put(fmt.Sprintf("/api/students/%d/privacy-consent", studentID), map[string]any{
			"policy_version":      "1.0",
			"accepted":            true,
			"duration_days":       365,
			"data_retention_days": 30,
			"details": map[string]any{
				"accepted_by": "guardian",
				"accepted_at": time.Now().AddDate(0, -1, 0).Format(time.RFC3339),
			},
		})
		if err != nil {
			if rt.Verbose {
				fmt.Printf("  WARNING: failed to create consent for student %d: %v\n", studentID, err)
			}
			continue
		}
		count++
	}

	fmt.Printf("  %d privacy consents created\n", count)
	return nil
}

// seedStatisticsDemoStep records a small, deterministic attendance and room
// scenario so a fresh demo tenant can show the Statistik page immediately.
type seedStatisticsDemoStep struct{}

func (seedStatisticsDemoStep) Name() string { return "Seeding statistics demo data" }

func (seedStatisticsDemoStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil || len(rt.FixedSeeder.staffCredentials) == 0 {
		return fmt.Errorf("fixed seeder data not available")
	}
	deviceKey := rt.FixedSeeder.deviceKeys[DemoDevices[0].DeviceID]
	activityID := rt.FixedSeeder.activityIDs[DemoActivities[0].Name]
	roomID := rt.FixedSeeder.activityRoomIDs[activityID]
	staffID := rt.FixedSeeder.staffIDs[rt.FixedSeeder.staffCredentials[0].Name]
	if deviceKey == "" || activityID == 0 || roomID == 0 || staffID == 0 {
		return fmt.Errorf("statistics demo prerequisites not available")
	}
	if _, err := rt.Client.DevicePost("/api/iot/session/start", map[string]any{
		"activity_id": activityID, "room_id": roomID, "supervisor_ids": []int64{staffID},
	}, deviceKey, rt.StaffPIN); err != nil {
		return fmt.Errorf("start statistics demo session: %w", err)
	}
	seeded := 0
	for i := 0; i < 3 && i < len(DemoStudents); i++ {
		studentID, ok := rt.FixedSeeder.studentIDByIndex[i]
		if !ok {
			continue
		}
		// Der Tag muss hexadezimal sein (models/users.RFIDCard.Validate).
		rfid := fmt.Sprintf("57A7%08X", studentID)
		if _, err := rt.Client.DevicePost(fmt.Sprintf("/api/students/%d/rfid", studentID), map[string]string{"rfid_tag": rfid}, deviceKey, rt.StaffPIN); err != nil {
			return fmt.Errorf("assign statistics demo RFID: %w", err)
		}
		if _, err := rt.Client.DevicePost("/api/iot/attendance/toggle", map[string]string{"rfid": rfid, "action": "confirm"}, deviceKey, rt.StaffPIN); err != nil {
			return fmt.Errorf("record statistics demo attendance: %w", err)
		}
		if _, err := rt.Client.DevicePost("/api/iot/checkin", map[string]any{"student_rfid": rfid, "action": "checkin", "room_id": roomID}, deviceKey, rt.StaffPIN); err != nil {
			return fmt.Errorf("record statistics demo room visit: %w", err)
		}
		seeded++
	}
	if _, err := rt.Client.DevicePost("/api/iot/session/end", nil, deviceKey, rt.StaffPIN); err != nil {
		return fmt.Errorf("end statistics demo session: %w", err)
	}
	if err := checkOutStatisticsSupervisor(rt); err != nil {
		return err
	}
	fmt.Printf("  %d attendance records and room visits seeded for Statistik\n", seeded)
	return nil
}

// checkOutStatisticsSupervisor bucht die Aufsicht wieder aus. Der
// Sitzungsstart stempelt sie per NFC ein (ensureNFCAutoCheckIn), das
// Sitzungsende bucht sie nicht aus; sonst bliebe eine offene Arbeitszeit im
// Mandanten stehen. Hat die Zeiterfassungs-Historie den heutigen Block schon
// geschrieben, unterbleibt der NFC-Stempel und es gibt nichts auszubuchen.
func checkOutStatisticsSupervisor(rt *Runtime) error {
	cred := rt.FixedSeeder.staffCredentials[0]
	previous := rt.Client.auth
	defer rt.Client.BindAuth(previous)

	if err := rt.Client.Login(cred.Email, cred.Password); err != nil {
		return fmt.Errorf("login statistics demo supervisor %s: %w", cred.Email, err)
	}
	open, err := hasOpenWorkSession(rt)
	if err != nil {
		return err
	}
	if !open {
		return nil
	}
	if _, err := rt.Client.Post("/api/time-tracking/check-out", nil); err != nil {
		return fmt.Errorf("check out statistics demo supervisor: %w", err)
	}
	return nil
}

// hasOpenWorkSession reports whether the logged-in staff account has a
// running work session. GET /current answers with a null payload when there
// is none.
func hasOpenWorkSession(rt *Runtime) (bool, error) {
	resp, err := rt.Client.Get("/api/time-tracking/current")
	if err != nil {
		return false, fmt.Errorf("read current work session: %w", err)
	}
	var payload struct {
		Data *struct {
			ID json.Number `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &payload); err != nil {
		return false, fmt.Errorf("parse current work session: %w", err)
	}
	return payload.Data != nil && payload.Data.ID != "", nil
}

// seedCareExitsStep records two planned ends of care (#2487) so the child
// management shows the "Betreuung endet am …" state and the reason table is
// not empty on a fresh machine.
//
// Both exits are dated today or later on purpose: the children stay fully
// usable for every demo and for `simulate full-day`, and the API refuses a
// retroactive exit anyway. The archive view ("Beendete Betreuungen") fills up
// by itself from the day after the first of them.
type seedCareExitsStep struct{}

func (seedCareExitsStep) Name() string { return "Seeding planned care exits" }

func (seedCareExitsStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("fixed seeder not available")
	}

	today := timezone.TodayDate()
	// Two children from the tail of the demo cohort, so the exits never
	// collide with the children the other steps mark sick or check in.
	plans := []struct {
		StudentIndex int
		LastCareDay  timezone.Date
		Reason       string
		Note         string
	}{
		{len(DemoStudents) - 1, today, "moved_away", ""},
		{len(DemoStudents) - 2, today.AddDays(7), "other", "Wechsel in die Nachmittagsbetreuung des Vereins"},
	}

	created := 0
	for _, plan := range plans {
		studentID, ok := rt.FixedSeeder.studentIDByIndex[plan.StudentIndex]
		if !ok {
			continue
		}
		body := map[string]any{
			"student_ids":   []string{strconv.FormatInt(studentID, 10)},
			"last_care_day": plan.LastCareDay.String(),
			"reason":        plan.Reason,
			"reason_note":   plan.Note,
		}

		// Preview first, exactly like the UI: the confirmation only accepts the
		// token the preview handed out.
		raw, err := rt.Client.Post("/api/students/care-end/preview", body)
		if err != nil {
			if rt.Verbose {
				fmt.Printf("  WARNING: care-exit preview failed for student %d: %v\n", studentID, err)
			}
			continue
		}
		var preview struct {
			Data struct {
				Token   string `json:"token"`
				Blocked bool   `json:"blocked"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &preview); err != nil || preview.Data.Blocked || preview.Data.Token == "" {
			if rt.Verbose {
				fmt.Printf("  WARNING: care-exit preview unusable for student %d\n", studentID)
			}
			continue
		}

		body["token"] = preview.Data.Token
		if _, err := rt.Client.Post("/api/students/care-end", body); err != nil {
			if rt.Verbose {
				fmt.Printf("  WARNING: care-exit failed for student %d: %v\n", studentID, err)
			}
			continue
		}
		created++
	}

	fmt.Printf("  %d planned care exits created\n", created)
	return nil
}

// seedCourseParticipationStep records past occurrences of three demo courses
// with a decided attendance row per child, so the Statistik section "Kurse"
// (#2891) is not empty on a fresh demo tenant.
//
// The occurrences are created through the ordinary instance endpoint with an
// activity_group_id: that is a real course date, only outside the
// materialization cycle. One date is cancelled and one child is left
// undecided on purpose — the screen has to show that cancelled dates count
// nowhere and that an unfinished block does not drag the quota down.
type seedCourseParticipationStep struct{}

func (seedCourseParticipationStep) Name() string { return "Seeding course participation" }

// courseDemoWeeks is how many past occurrences each demo course gets.
const courseDemoWeeks = 4

// courseDemoChildren is how many children take part per course; the fixed
// seeder enrolls the first five demo children in every activity.
const courseDemoChildren = 5

func (seedCourseParticipationStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("fixed seeder not available")
	}
	dates := pastWeekdays(courseDemoWeeks)
	instances, cancelled := 0, 0

	for courseIndex, activity := range DemoActivities {
		if courseIndex >= 3 {
			break
		}
		activityID := rt.FixedSeeder.activityIDs[activity.Name]
		roomID := rt.FixedSeeder.activityRoomIDs[activityID]
		if activityID == 0 || roomID == 0 {
			continue
		}
		studentIDs := make([]int64, 0, courseDemoChildren)
		for i := range courseDemoChildren {
			if id, ok := rt.FixedSeeder.studentIDByIndex[i]; ok {
				studentIDs = append(studentIDs, id)
			}
		}
		if len(studentIDs) == 0 {
			continue
		}

		for dateIndex, date := range dates {
			instanceID, err := createCourseOccurrence(rt, activity.Name, date, roomID, activityID, studentIDs)
			if err != nil {
				return err
			}
			instances++

			// The oldest date of the first course was cancelled.
			if courseIndex == 0 && dateIndex == 0 {
				if _, err := rt.Client.Post(fmt.Sprintf("/api/timetable/instances/%d/cancel", instanceID), map[string]any{
					"reason": "Raum war belegt",
				}); err != nil {
					return fmt.Errorf("cancel course occurrence: %w", err)
				}
				cancelled++
				continue
			}

			for studentIndex, studentID := range studentIDs {
				status := courseAttendanceStatus(courseIndex, dateIndex, studentIndex, len(dates))
				if status == "" {
					continue // left undecided: the block was never finished
				}
				path := fmt.Sprintf("/api/timetable/instances/%d/students/%d", instanceID, studentID)
				if _, err := rt.Client.Patch(path, map[string]any{"status": status}); err != nil {
					return fmt.Errorf("record course attendance: %w", err)
				}
			}
		}
	}

	fmt.Printf("  %d course occurrences seeded (%d cancelled)\n", instances, cancelled)
	return nil
}

// createCourseOccurrence books one past date of a course with its roster and
// returns the new instance ID.
func createCourseOccurrence(rt *Runtime, title string, date timezone.Date, roomID, activityID int64, studentIDs []int64) (int64, error) {
	body := map[string]any{
		"date":              date.String(),
		"start_time":        "14:00",
		"end_time":          "15:00",
		"title":             title,
		"room_id":           roomID,
		"activity_group_id": activityID,
		"student_ids":       studentIDs,
	}
	respBody, err := rt.Client.Post("/api/timetable/instances", body)
	if err != nil {
		return 0, fmt.Errorf("create course occurrence %s on %s: %w", title, date, err)
	}
	var resp struct {
		Data struct {
			ID json.Number `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return 0, fmt.Errorf("parse course occurrence response: %w", err)
	}
	id, err := resp.Data.ID.Int64()
	if err != nil || id == 0 {
		return 0, fmt.Errorf("course occurrence response carries no id")
	}
	return id, nil
}

// courseAttendanceStatus spreads a plausible pattern over the demo courses:
// mostly present, a recurring absence for one child, and the newest date of
// the last course left undecided so the "Offen" column is not always zero.
// An empty result means "write nothing".
func courseAttendanceStatus(courseIndex, dateIndex, studentIndex, dateCount int) string {
	if courseIndex == 2 && dateIndex == dateCount-1 {
		return ""
	}
	if studentIndex == courseIndex+1 && dateIndex%2 == 0 {
		return "absent"
	}
	return "present"
}

// pastWeekdays returns the last n weekdays before today, oldest first, one
// per week so the occurrences read as a weekly course.
func pastWeekdays(n int) []timezone.Date {
	dates := make([]timezone.Date, 0, n)
	day := timezone.TodayDate().AddDays(-1)
	for len(dates) < n {
		switch day.Weekday() {
		case time.Saturday, time.Sunday:
			day = day.AddDays(-1)
			continue
		default:
		}
		dates = append(dates, day)
		day = day.AddDays(-7)
	}
	// Collected newest first; the demo reads better oldest first.
	for i, j := 0, len(dates)-1; i < j; i, j = i+1, j-1 {
		dates[i], dates[j] = dates[j], dates[i]
	}
	return dates
}
