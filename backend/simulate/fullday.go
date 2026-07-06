package simulate

import (
	"context"
	"fmt"
	"maps"
	"math/rand"
	"slices"

	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
)

var feedbackValues = []string{"positive", "neutral", "negative"}

// FullDayOptions configures a full-day simulation run.
type FullDayOptions struct {
	StatePath string
	Close     bool // if true, do daily checkout + end sessions at the end
	Verbose   bool
}

// RunFullDay runs a one-shot full-day simulation using seed state.
func RunFullDay(ctx context.Context, opts FullDayOptions) error {
	state, err := seedapi.LoadSeedState(opts.StatePath)
	if err != nil {
		return fmt.Errorf("load seed state: %w", err)
	}

	client := newClient(state.BaseURL, opts.Verbose)
	runtime := newRuntime(state, client, opts)
	scenario := fullDayScenario(opts.Close)
	if err := scenario.Run(ctx, runtime); err != nil {
		return err
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
	rfidDevice := rt.State.Devices[deviceKeysForRFID[0]]

	for _, student := range rt.State.Students {
		rfidTag := fmt.Sprintf("DE%06X", student.ID)
		rt.RFIDTags[student.ID] = rfidTag

		body := map[string]string{"rfid_tag": rfidTag}
		_, err := rt.Client.DevicePost(fmt.Sprintf("/api/students/%d/rfid", student.ID), body, rfidDevice.APIKey, rt.State.DevicePIN)
		if err != nil {
			fmt.Printf("  WARNING: failed to assign RFID to student %d (%s %s): %v\n",
				student.ID, student.FirstName, student.LastName, err)
			continue
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
			fmt.Printf("  WARNING: failed to start session for %s: %v\n", actName, err)
			continue
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

func (recordAttendanceAction) Name() string { return "record attendance and checkins" }

func (recordAttendanceAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 4: Recording attendance and checking in students...")

	primaryDevice, err := rt.primaryDevice()
	if err != nil {
		return err
	}

	studentsToProcess := len(rt.State.Students)
	if studentsToProcess > 90 {
		studentsToProcess = 90
	}

	for i := 0; i < studentsToProcess; i++ {
		student := rt.State.Students[i]
		rfidTag, ok := rt.RFIDTags[student.ID]
		if !ok {
			continue
		}

		attendanceBody := map[string]string{
			"rfid":   rfidTag,
			"action": "confirm",
		}
		_, err := rt.Client.DevicePost("/api/iot/attendance/toggle", attendanceBody, primaryDevice.APIKey, rt.State.DevicePIN)
		if err != nil {
			if rt.Options.Verbose {
				fmt.Printf("  WARNING: attendance toggle failed for student %d: %v\n", student.ID, err)
			}
			continue
		}
		rt.Counts.AttendanceRecords++

		if i < 85 && len(rt.ActiveRoomIDs) > 0 {
			roomID := rt.ActiveRoomIDs[i%len(rt.ActiveRoomIDs)]
			checkinBody := map[string]any{
				"student_rfid": rfidTag,
				"action":       "checkin",
				"room_id":      roomID,
			}
			_, err := rt.Client.DevicePost("/api/iot/checkin", checkinBody, primaryDevice.APIKey, rt.State.DevicePIN)
			if err != nil {
				if rt.Options.Verbose {
					fmt.Printf("  WARNING: checkin failed for student %d: %v\n", student.ID, err)
				}
				continue
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
			if rt.Options.Verbose {
				fmt.Printf("  WARNING: failed to mark student %d sick: %v\n", student.ID, err)
			}
			continue
		}
		rt.Counts.StudentsSick++
	}

	for i := 75; i < 85 && i < len(rt.State.Students); i++ {
		student := rt.State.Students[i]
		rfidTag, ok := rt.RFIDTags[student.ID]
		if !ok {
			continue
		}
		body := map[string]any{
			"student_rfid": rfidTag,
			"action":       "checkout",
		}
		_, err := rt.Client.DevicePost("/api/iot/checkin", body, primaryDevice.APIKey, rt.State.DevicePIN)
		if err != nil {
			if rt.Options.Verbose {
				fmt.Printf("  WARNING: checkout failed for student %d: %v\n", student.ID, err)
			}
			continue
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
			continue
		}
		// Query pickup info first (mirrors PyrePortal flow)
		_, _ = rt.Client.DevicePost("/api/iot/pickup-query", map[string]any{
			"student_rfid": rfidTag,
		}, primaryDevice.APIKey, rt.State.DevicePIN)

		body := map[string]any{
			"rfid":        rfidTag,
			"action":      "confirm_daily_checkout",
			"destination": "zuhause",
		}
		_, err := rt.Client.DevicePost("/api/iot/attendance/toggle", body, primaryDevice.APIKey, rt.State.DevicePIN)
		if err != nil {
			if rt.Options.Verbose {
				fmt.Printf("  WARNING: daily checkout failed for student %d: %v\n", student.ID, err)
			}
			continue
		}
		rt.Counts.DailyCheckouts++

		// Submit feedback (like the PyrePortal smiley picker after checkout)
		feedbackBody := map[string]any{
			"student_id": student.ID,
			"value":      feedbackValues[rand.Intn(len(feedbackValues))],
		}
		_, err = rt.Client.DevicePost("/api/iot/feedback", feedbackBody, primaryDevice.APIKey, rt.State.DevicePIN)
		if err != nil {
			if rt.Options.Verbose {
				fmt.Printf("  WARNING: feedback failed for student %d: %v\n", student.ID, err)
			}
			continue
		}
		rt.Counts.FeedbackSubmitted++
	}
	fmt.Printf("  %d daily checkouts, %d feedback submitted\n", rt.Counts.DailyCheckouts, rt.Counts.FeedbackSubmitted)

	for i := 0; i < rt.Counts.SessionsStarted && i < len(rt.DeviceKeys); i++ {
		device := rt.State.Devices[rt.DeviceKeys[i]]
		_, err := rt.Client.DevicePost("/api/iot/session/end", nil, device.APIKey, rt.State.DevicePIN)
		if err != nil {
			if rt.Options.Verbose {
				fmt.Printf("  WARNING: failed to end session on device %s: %v\n", rt.DeviceKeys[i], err)
			}
			continue
		}
		rt.Counts.SessionsEnded++
	}
	fmt.Printf("  %d sessions ended\n", rt.Counts.SessionsEnded)
	return nil
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

func sortedDeviceKeys(devices map[string]seedapi.SeedDevice) []string {
	return slices.Sorted(maps.Keys(devices))
}

func sortedStringKeys(m map[string]int64) []string {
	return slices.Sorted(maps.Keys(m))
}
