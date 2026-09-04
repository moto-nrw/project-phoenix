package simulate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/rand"
	"slices"
	"time"
)

var feedbackValues = []string{"positive", "neutral", "negative"}

// Eight seeded sessions receive students round-robin. The 85th student would
// become the 11th child in Leseecke, whose seeded capacity is 10.
const fullDayCheckinLimit = 84

// FullDayOptions configures a full-day simulation run.
type FullDayOptions struct {
	StatePath string
	Close     bool // if true, do daily checkout + end sessions at the end
	Verbose   bool
	Client    ClientFactory
}

// RunFullDay runs a one-shot full-day simulation using seed state.
func RunFullDay(ctx context.Context, opts FullDayOptions) error {
	if opts.Client == nil {
		return fmt.Errorf("simulation client factory is required")
	}
	state, err := LoadSeedState(opts.StatePath)
	if err != nil {
		return fmt.Errorf("load seed state: %w", err)
	}

	client, err := buildClient(opts.Client, state.BaseURL, opts.Verbose)
	if err != nil {
		return err
	}
	runtime := newRuntime(state, client, opts)
	scenario := fullDayScenario(opts.Close)
	if err := scenario.Run(ctx, runtime); err != nil {
		profile := state.Bootstrap.TenantSlug
		if profile == "" {
			profile = "legacy seed state"
		}
		var actionErr *ActionError
		if errors.As(err, &actionErr) {
			return fmt.Errorf("demo school profile %q, workflow step %s failed: %w", profile, actionErr.Action, actionErr.Err)
		}
		return fmt.Errorf("demo school profile %q, workflow step %s failed: %w", profile, scenario.Name, err)
	}
	return nil
}

type verifyServerAction struct{}

func (verifyServerAction) Name() string { return "server health check" }

func (verifyServerAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("Phase 1: Loading seed state and verifying server...")
	if err := rt.Client.CheckHealth(); err != nil {
		return fmt.Errorf("server health check: %w", err)
	}
	fmt.Printf("  Server at %s is healthy\n", rt.State.BaseURL)
	return nil
}

type loginAdminAction struct{}

func (loginAdminAction) Name() string { return "admin login" }

func (loginAdminAction) Run(_ context.Context, rt *Runtime) error {
	if len(rt.State.Accounts.Admin) == 0 {
		return fmt.Errorf("no admin accounts in seed state")
	}
	admin := rt.State.Accounts.Admin[0]
	if err := rt.Client.Login(admin.Email, admin.Password); err != nil {
		return fmt.Errorf("admin login: %w", err)
	}
	fmt.Printf("  Logged in as %s\n", admin.Email)
	return nil
}

type assignRFIDsAction struct{}

func (assignRFIDsAction) Name() string { return "assign RFID tags" }

func (assignRFIDsAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Printf("\nPhase 2: Assigning RFID tags to %d students...\n", len(rt.State.Students))
	deviceKeysForRFID := sortedDeviceKeys(rt.State.Devices)
	if len(deviceKeysForRFID) == 0 {
		return fmt.Errorf("no devices in seed state for RFID assignment")
	}
	if len(rt.State.Students) == 0 {
		return fmt.Errorf("no students in seed state for RFID assignment")
	}
	rfidDevice := rt.State.Devices[deviceKeysForRFID[0]]

	for _, student := range rt.State.Students {
		rfidTag := fmt.Sprintf("DE%06X", student.ID)
		rt.RFIDTags[student.ID] = rfidTag

		body := map[string]string{"rfid_tag": rfidTag}
		_, err := rt.Client.DevicePost(fmt.Sprintf("/api/students/%d/rfid", student.ID), body, rfidDevice.APIKey, rt.State.DevicePIN)
		if err != nil {
			return fmt.Errorf("assign RFID to student %d: %w", student.ID, err)
		}
		rt.Counts.RFIDAssigned++
	}
	fmt.Printf("  %d/%d RFID tags assigned\n", rt.Counts.RFIDAssigned, len(rt.State.Students))
	return nil
}

type startSessionsAction struct{}

func (startSessionsAction) Name() string { return "start sessions" }

func (startSessionsAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 3: Starting activity sessions...")

	rt.DeviceKeys = sortedDeviceKeys(rt.State.Devices)
	rt.ActivityNames = sortedStringKeys(rt.State.Activities)
	betreuer := rt.State.Accounts.Betreuer

	sessionsToStart := min(len(betreuer), len(rt.DeviceKeys), len(rt.ActivityNames))
	if sessionsToStart == 0 {
		return fmt.Errorf("no sessions can start: %d staff accounts, %d devices, %d activities", len(betreuer), len(rt.DeviceKeys), len(rt.ActivityNames))
	}
	if sessionsToStart > 10 {
		sessionsToStart = 10
	}

	for i := 0; i < sessionsToStart; i++ {
		deviceKey := rt.DeviceKeys[i]
		device := rt.State.Devices[deviceKey]
		actName := rt.ActivityNames[i]
		activityID := rt.State.Activities[actName]
		supervisor := betreuer[i]

		roomID := findRoomForActivity(actName, rt.State.Rooms)

		body := map[string]any{
			"activity_id":    activityID,
			"supervisor_ids": []int64{supervisor.StaffID},
		}
		if roomID != 0 {
			body["room_id"] = roomID
		}

		_, err := rt.Client.DevicePost("/api/iot/session/start", body, device.APIKey, rt.State.DevicePIN)
		if err != nil {
			startErr := fmt.Errorf("start session for %s: %w", actName, err)
			if cleanupErr := endSessions(rt, rt.DeviceKeys[:i]); cleanupErr != nil {
				return errors.Join(startErr, fmt.Errorf("end previously started sessions: %w", cleanupErr))
			}
			return startErr
		}

		if roomID != 0 {
			rt.ActiveRoomIDs = append(rt.ActiveRoomIDs, roomID)
		}

		rt.Counts.SessionsStarted++
		fmt.Printf("  Started: %s (device: %s, supervisor: %s)\n", actName, device.Name, supervisor.Name)
	}

	if len(rt.ActiveRoomIDs) == 0 {
		for _, roomID := range rt.State.Rooms {
			rt.ActiveRoomIDs = append(rt.ActiveRoomIDs, roomID)
		}
	}

	fmt.Printf("  %d sessions started\n", rt.Counts.SessionsStarted)
	return nil
}

type recordAttendanceAction struct{}

type seedStaffFeedTombstoneAction struct{}

func (seedStaffFeedTombstoneAction) Name() string { return "seed staff feed tombstone" }

func (seedStaffFeedTombstoneAction) Run(_ context.Context, rt *Runtime) error {
	if len(rt.State.Accounts.Betreuer) == 0 {
		return fmt.Errorf("no staff accounts in seed state")
	}
	roomNames := sortedStringKeys(rt.State.Rooms)
	if len(roomNames) == 0 {
		return fmt.Errorf("no rooms in seed state")
	}

	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return fmt.Errorf("load Berlin timezone: %w", err)
	}
	periodResponse, err := rt.Client.Post("/api/timetable/periods/bootstrap", nil)
	if err != nil {
		return fmt.Errorf("bootstrap calendar periods for demo cancellation: %w", err)
	}
	var periodEnvelope struct {
		Data struct {
			Periods []struct {
				StartDate string `json:"start_date"`
				EndDate   string `json:"end_date"`
				IsActive  bool   `json:"is_active"`
			} `json:"periods"`
		} `json:"data"`
	}
	if err := json.Unmarshal(periodResponse, &periodEnvelope); err != nil {
		return fmt.Errorf("decode POST /api/timetable/periods/bootstrap response: %w", err)
	}
	today, err := time.ParseInLocation("2006-01-02", time.Now().In(berlin).Format("2006-01-02"), berlin)
	if err != nil {
		return fmt.Errorf("normalize current date: %w", err)
	}
	var date time.Time
	for _, period := range periodEnvelope.Data.Periods {
		if !period.IsActive {
			continue
		}
		start, startErr := time.ParseInLocation("2006-01-02", period.StartDate, berlin)
		if startErr != nil {
			return fmt.Errorf("decode calendar period start date for demo cancellation: %w", startErr)
		}
		end, endErr := time.ParseInLocation("2006-01-02", period.EndDate, berlin)
		if endErr != nil {
			return fmt.Errorf("decode calendar period end date for demo cancellation: %w", endErr)
		}
		candidate := today.AddDate(0, 0, 1)
		if start.After(candidate) {
			candidate = start
		}
		for candidate.Weekday() == time.Saturday || candidate.Weekday() == time.Sunday {
			candidate = candidate.AddDate(0, 0, 1)
		}
		if candidate.After(end) || (!date.IsZero() && !candidate.Before(date)) {
			continue
		}
		date = candidate
	}
	if date.IsZero() {
		return fmt.Errorf("no future weekday in an active calendar period for demo cancellation")
	}
	body := map[string]any{
		"date":       date.Format("2006-01-02"),
		"start_time": "07:00",
		"end_time":   "07:30",
		"title":      "Abgesagter Demo-Termin",
		"room_id":    rt.State.Rooms[roomNames[0]],
		"staff_ids":  []int64{rt.State.Accounts.Betreuer[0].StaffID},
	}
	created, err := rt.Client.Post("/api/timetable/instances", body)
	if err != nil {
		return fmt.Errorf("create demo cancellation: %w", err)
	}
	var envelope struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created, &envelope); err != nil {
		return fmt.Errorf("decode POST /api/timetable/instances response: %w", err)
	}
	if envelope.Data.ID <= 0 {
		return fmt.Errorf("decode demo cancellation: response has no instance id")
	}
	if _, err := rt.Client.Delete(fmt.Sprintf("/api/timetable/instances/%d", envelope.Data.ID)); err != nil {
		return fmt.Errorf("delete demo cancellation: %w", err)
	}
	return nil
}

func (recordAttendanceAction) Name() string { return "record attendance and checkins" }

func (recordAttendanceAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 4: Recording attendance and checking in students...")

	primaryDevice, err := rt.primaryDevice()
	if err != nil {
		return err
	}
	// Exercise the real kiosk failure path too. The endpoint intentionally
	// returns 404, while recording the unknown tag for operator follow-up.
	_, err = rt.Client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": "DEMO-UNREGISTERED-TAG", "action": "checkin",
	}, primaryDevice.APIKey, rt.State.DevicePIN)
	var expectedErr interface {
		HTTPStatusCode() int
		HTTPErrorCode() string
	}
	if !errors.As(err, &expectedErr) || expectedErr.HTTPStatusCode() != 404 || expectedErr.HTTPErrorCode() != "rfid_tag_not_found" {
		return fmt.Errorf("record unregistered tag scan: expected 404 (rfid_tag_not_found), got %w", err)
	}

	studentsToProcess := len(rt.State.Students)
	if studentsToProcess > 90 {
		studentsToProcess = 90
	}

	for i := 0; i < studentsToProcess; i++ {
		student := rt.State.Students[i]
		rfidTag, ok := rt.RFIDTags[student.ID]
		if !ok {
			return fmt.Errorf("RFID assignment missing for student %d", student.ID)
		}

		attendanceBody := map[string]string{
			"rfid":   rfidTag,
			"action": "confirm",
		}
		_, err := rt.Client.DevicePost("/api/iot/attendance/toggle", attendanceBody, primaryDevice.APIKey, rt.State.DevicePIN)
		if err != nil {
			return fmt.Errorf("record attendance for student %d: %w", student.ID, err)
		}
		rt.Counts.AttendanceRecords++

		if i < fullDayCheckinLimit && len(rt.ActiveRoomIDs) > 0 {
			roomID := rt.ActiveRoomIDs[i%len(rt.ActiveRoomIDs)]
			checkinBody := map[string]any{
				"student_rfid": rfidTag,
				"action":       "checkin",
				"room_id":      roomID,
			}
			_, err := rt.Client.DevicePost("/api/iot/checkin", checkinBody, primaryDevice.APIKey, rt.State.DevicePIN)
			if err != nil {
				return fmt.Errorf("check in student %d: %w", student.ID, err)
			}
			rt.Counts.StudentsCheckedIn++
		}
	}

	fmt.Printf("  %d attendance records, %d students checked in\n", rt.Counts.AttendanceRecords, rt.Counts.StudentsCheckedIn)
	return nil
}

type middayActivityAction struct{}

func (middayActivityAction) Name() string { return "run mid-day activity" }

func (middayActivityAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 5: Mid-day activity (sick marks, individual checkouts)...")

	primaryDevice, err := rt.primaryDevice()
	if err != nil {
		return err
	}

	for i := 90; i < len(rt.State.Students) && i < 100; i++ {
		student := rt.State.Students[i]
		body := map[string]any{"sick": true}
		_, err := rt.Client.Put(fmt.Sprintf("/api/students/%d", student.ID), body)
		if err != nil {
			return fmt.Errorf("mark student %d sick: %w", student.ID, err)
		}
		rt.Counts.StudentsSick++
	}

	for i := 75; i < fullDayCheckinLimit && i < len(rt.State.Students); i++ {
		student := rt.State.Students[i]
		rfidTag, ok := rt.RFIDTags[student.ID]
		if !ok {
			return fmt.Errorf("RFID assignment missing for student %d", student.ID)
		}
		body := map[string]any{
			"student_rfid": rfidTag,
			"action":       "checkout",
		}
		_, err := rt.Client.DevicePost("/api/iot/checkin", body, primaryDevice.APIKey, rt.State.DevicePIN)
		if err != nil {
			return fmt.Errorf("check out student %d: %w", student.ID, err)
		}
		rt.Counts.StudentsCheckedOut++
	}

	fmt.Printf("  %d marked sick, %d checked out\n", rt.Counts.StudentsSick, rt.Counts.StudentsCheckedOut)
	return nil
}

type endOfDayAction struct{}

func (endOfDayAction) Name() string { return "run end-of-day flow" }

func (endOfDayAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 6: End of day (daily checkout + end sessions)...")

	primaryDevice, err := rt.primaryDevice()
	if err != nil {
		return err
	}

	for i := 0; i < 75 && i < len(rt.State.Students); i++ {
		student := rt.State.Students[i]
		rfidTag, ok := rt.RFIDTags[student.ID]
		if !ok {
			return fmt.Errorf("RFID assignment missing for student %d", student.ID)
		}
		// Query pickup info first (mirrors PyrePortal flow)
		_, err := rt.Client.DevicePost("/api/iot/pickup-query", map[string]any{
			"student_rfid": rfidTag,
		}, primaryDevice.APIKey, rt.State.DevicePIN)
		if err != nil {
			return fmt.Errorf("query pickup for student %d: %w", student.ID, err)
		}

		body := map[string]any{
			"rfid":        rfidTag,
			"action":      "confirm_daily_checkout",
			"destination": "zuhause",
		}
		_, err = rt.Client.DevicePost("/api/iot/attendance/toggle", body, primaryDevice.APIKey, rt.State.DevicePIN)
		if err != nil {
			return fmt.Errorf("record daily checkout for student %d: %w", student.ID, err)
		}
		rt.Counts.DailyCheckouts++

		// Submit feedback (like the PyrePortal smiley picker after checkout)
		feedbackBody := map[string]any{
			"student_id": student.ID,
			"value":      feedbackValues[rand.Intn(len(feedbackValues))],
		}
		_, err = rt.Client.DevicePost("/api/iot/feedback", feedbackBody, primaryDevice.APIKey, rt.State.DevicePIN)
		if err != nil {
			return fmt.Errorf("submit feedback for student %d: %w", student.ID, err)
		}
		rt.Counts.FeedbackSubmitted++
	}
	fmt.Printf("  %d daily checkouts, %d feedback submitted\n", rt.Counts.DailyCheckouts, rt.Counts.FeedbackSubmitted)

	sessionsStarted := min(rt.Counts.SessionsStarted, len(rt.DeviceKeys))
	if err := endSessions(rt, rt.DeviceKeys[:sessionsStarted]); err != nil {
		return err
	}
	fmt.Printf("  %d sessions ended\n", rt.Counts.SessionsEnded)
	return nil
}

func endSessions(rt *Runtime, deviceKeys []string) error {
	var errs []error
	for _, deviceKey := range deviceKeys {
		device := rt.State.Devices[deviceKey]
		_, err := rt.Client.DevicePost("/api/iot/session/end", nil, device.APIKey, rt.State.DevicePIN)
		if err != nil {
			errs = append(errs, fmt.Errorf("end session on device %s: %w", deviceKey, err))
			continue
		}
		rt.Counts.SessionsEnded++
	}
	return errors.Join(errs...)
}

type printSummaryAction struct{}

func (printSummaryAction) Name() string { return "print summary" }

func (printSummaryAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\n--- Simulation Summary ---")
	fmt.Printf("  RFID tags assigned:  %d\n", rt.Counts.RFIDAssigned)
	fmt.Printf("  Sessions started:    %d\n", rt.Counts.SessionsStarted)
	fmt.Printf("  Attendance records:  %d\n", rt.Counts.AttendanceRecords)
	fmt.Printf("  Students checked in: %d\n", rt.Counts.StudentsCheckedIn)
	fmt.Printf("  Students sick:       %d\n", rt.Counts.StudentsSick)
	fmt.Printf("  Students checked out:%d\n", rt.Counts.StudentsCheckedOut)
	if rt.Options.Close {
		fmt.Printf("  Feedback submitted:  %d\n", rt.Counts.FeedbackSubmitted)
		fmt.Println("  End-of-day:          completed")
	}
	fmt.Println("--------------------------")
	return nil
}

func fullDayScenario(close bool) Scenario {
	actions := []Action{
		verifyServerAction{},
		loginAdminAction{},
		assignRFIDsAction{},
		startSessionsAction{},
		seedStaffFeedTombstoneAction{},
		recordAttendanceAction{},
		middayActivityAction{},
	}
	if close {
		actions = append(actions, endOfDayAction{})
	}
	actions = append(actions, printSummaryAction{})
	return Scenario{
		Name:    "full-day",
		Actions: actions,
	}
}

// findRoomForActivity maps activity names to their default rooms using the demo data convention.
func findRoomForActivity(activityName string, rooms map[string]int64) int64 {
	activityRoomMap := map[string]string{
		"Hausaufgaben": "OGS-Raum 1",
		"Fußball":      "Sporthalle",
		"Basteln":      "Kreativraum",
		"Kochen":       "Mensa",
		"Lesen":        "Leseecke",
		"Musik":        "Musikraum",
		"Tanzen":       "Bewegungsraum",
		"Schach":       "OGS-Raum 2",
		"Garten":       "Schulhof",
		"Freispiel":    "OGS-Raum 3",
	}

	roomName, ok := activityRoomMap[activityName]
	if !ok {
		return 0
	}
	return rooms[roomName]
}

func sortedDeviceKeys(devices map[string]SeedDevice) []string {
	return slices.Sorted(maps.Keys(devices))
}

func sortedStringKeys(m map[string]int64) []string {
	return slices.Sorted(maps.Keys(m))
}
