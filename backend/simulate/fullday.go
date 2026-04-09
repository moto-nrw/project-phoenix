package simulate

import (
	"context"
	"fmt"
	"sort"

	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
)

// FullDayOptions configures a full-day simulation run.
type FullDayOptions struct {
	StatePath string
	Close     bool // if true, do daily checkout + end sessions at the end
	Verbose   bool
}

// RunFullDay runs a one-shot full-day simulation using seed state.
func RunFullDay(ctx context.Context, opts FullDayOptions) error {
	// Phase 1: Load state and verify server
	fmt.Println("Phase 1: Loading seed state and verifying server...")

	state, err := seedapi.LoadSeedState(opts.StatePath)
	if err != nil {
		return fmt.Errorf("load seed state: %w", err)
	}

	client := NewClient(state.BaseURL, opts.Verbose)

	if err := client.CheckHealth(); err != nil {
		return fmt.Errorf("server health check: %w", err)
	}
	fmt.Printf("  Server at %s is healthy\n", state.BaseURL)

	if len(state.Accounts.Admin) == 0 {
		return fmt.Errorf("no admin accounts in seed state")
	}
	admin := state.Accounts.Admin[0]
	if err := client.Login(admin.Email, admin.Password); err != nil {
		return fmt.Errorf("admin login: %w", err)
	}
	fmt.Printf("  Logged in as %s\n", admin.Email)

	// Phase 2: Assign RFID tags to all students via device auth
	fmt.Printf("\nPhase 2: Assigning RFID tags to %d students...\n", len(state.Students))
	rfidAssigned := 0
	rfidTags := make(map[int64]string) // studentID -> rfid tag

	deviceKeysForRFID := sortedDeviceKeys(state.Devices)
	if len(deviceKeysForRFID) == 0 {
		return fmt.Errorf("no devices in seed state for RFID assignment")
	}
	rfidDevice := state.Devices[deviceKeysForRFID[0]]

	for _, student := range state.Students {
		rfidTag := fmt.Sprintf("DE%06X", student.ID)
		rfidTags[student.ID] = rfidTag

		body := map[string]string{"rfid_tag": rfidTag}
		_, err := client.DevicePost(fmt.Sprintf("/api/students/%d/rfid", student.ID), body, rfidDevice.APIKey, state.DevicePIN)
		if err != nil {
			fmt.Printf("  WARNING: failed to assign RFID to student %d (%s %s): %v\n",
				student.ID, student.FirstName, student.LastName, err)
			continue
		}
		rfidAssigned++
	}
	fmt.Printf("  %d/%d RFID tags assigned\n", rfidAssigned, len(state.Students))

	// Phase 3: Start sessions via device auth
	fmt.Println("\nPhase 3: Starting activity sessions...")

	deviceKeys := sortedDeviceKeys(state.Devices)
	betreuer := state.Accounts.Betreuer
	activityNames := sortedStringKeys(state.Activities)

	sessionsToStart := min(len(betreuer), len(deviceKeys), len(activityNames))
	if sessionsToStart > 10 {
		sessionsToStart = 10
	}

	activeRoomIDs := []int64{}
	sessionsStarted := 0

	for i := 0; i < sessionsToStart; i++ {
		deviceKey := deviceKeys[i]
		device := state.Devices[deviceKey]
		actName := activityNames[i]
		activityID := state.Activities[actName]
		supervisor := betreuer[i]

		body := map[string]any{
			"activity_id":    activityID,
			"supervisor_ids": []int64{supervisor.StaffID},
		}

		_, err := client.DevicePost("/api/iot/session/start", body, device.APIKey, state.DevicePIN)
		if err != nil {
			fmt.Printf("  WARNING: failed to start session for %s: %v\n", actName, err)
			continue
		}

		// Try to find the room for this activity from state.Rooms
		// Activity DefaultRoom maps activity->room name, but we only have state.Rooms (name->id)
		// Use the demo data mapping: look up the activity's default room
		if roomID := findRoomForActivity(actName, state.Rooms); roomID != 0 {
			activeRoomIDs = append(activeRoomIDs, roomID)
		}

		sessionsStarted++
		fmt.Printf("  Started: %s (device: %s, supervisor: %s)\n", actName, device.Name, supervisor.Name)
	}
	fmt.Printf("  %d sessions started\n", sessionsStarted)

	if len(activeRoomIDs) == 0 {
		// Fallback: use all rooms
		for _, roomID := range state.Rooms {
			activeRoomIDs = append(activeRoomIDs, roomID)
		}
	}

	// Phase 4: Attendance + Check-ins
	fmt.Println("\nPhase 4: Recording attendance and checking in students...")

	// Use first device for attendance/checkin operations
	if len(deviceKeys) == 0 {
		return fmt.Errorf("no devices available")
	}
	primaryDevice := state.Devices[deviceKeys[0]]

	attendanceCount := 0
	checkinCount := 0
	studentsToProcess := len(state.Students)
	if studentsToProcess > 90 {
		studentsToProcess = 90
	}

	for i := 0; i < studentsToProcess; i++ {
		student := state.Students[i]
		rfidTag, ok := rfidTags[student.ID]
		if !ok {
			continue
		}

		// Toggle attendance (confirm arrival)
		attendanceBody := map[string]string{
			"rfid":   rfidTag,
			"action": "confirm",
		}
		_, err := client.DevicePost("/api/iot/attendance/toggle", attendanceBody, primaryDevice.APIKey, state.DevicePIN)
		if err != nil {
			if opts.Verbose {
				fmt.Printf("  WARNING: attendance toggle failed for student %d: %v\n", student.ID, err)
			}
			continue
		}
		attendanceCount++

		// Check in to a room (distribute across active rooms)
		if i < 85 && len(activeRoomIDs) > 0 {
			roomID := activeRoomIDs[i%len(activeRoomIDs)]
			checkinBody := map[string]any{
				"student_rfid": rfidTag,
				"action":       "checkin",
				"room_id":      roomID,
			}
			_, err := client.DevicePost("/api/iot/checkin", checkinBody, primaryDevice.APIKey, state.DevicePIN)
			if err != nil {
				if opts.Verbose {
					fmt.Printf("  WARNING: checkin failed for student %d: %v\n", student.ID, err)
				}
				continue
			}
			checkinCount++
		}
	}
	fmt.Printf("  %d attendance records, %d students checked in\n", attendanceCount, checkinCount)

	// Phase 5: Mid-day activity
	fmt.Println("\nPhase 5: Mid-day activity (sick marks, individual checkouts)...")

	sickCount := 0
	checkoutCount := 0

	// Mark some students sick via admin API (students 90-99 if available)
	for i := 90; i < len(state.Students) && i < 100; i++ {
		student := state.Students[i]
		body := map[string]any{"sick": true}
		_, err := client.AdminPut(fmt.Sprintf("/api/students/%d", student.ID), body)
		if err != nil {
			if opts.Verbose {
				fmt.Printf("  WARNING: failed to mark student %d sick: %v\n", student.ID, err)
			}
			continue
		}
		sickCount++
	}

	// Check out ~10 students via device
	for i := 75; i < 85 && i < len(state.Students); i++ {
		student := state.Students[i]
		rfidTag, ok := rfidTags[student.ID]
		if !ok {
			continue
		}
		body := map[string]any{
			"student_rfid": rfidTag,
			"action":       "checkout",
		}
		_, err := client.DevicePost("/api/iot/checkin", body, primaryDevice.APIKey, state.DevicePIN)
		if err != nil {
			if opts.Verbose {
				fmt.Printf("  WARNING: checkout failed for student %d: %v\n", student.ID, err)
			}
			continue
		}
		checkoutCount++
	}
	fmt.Printf("  %d marked sick, %d checked out\n", sickCount, checkoutCount)

	// Phase 6: End of day (optional)
	if opts.Close {
		fmt.Println("\nPhase 6: End of day (daily checkout + end sessions)...")

		dailyCheckoutCount := 0
		dest := "zuhause"

		// Daily checkout for remaining students
		for i := 0; i < 75 && i < len(state.Students); i++ {
			student := state.Students[i]
			rfidTag, ok := rfidTags[student.ID]
			if !ok {
				continue
			}
			body := map[string]any{
				"rfid":        rfidTag,
				"action":      "confirm_daily_checkout",
				"destination": dest,
			}
			_, err := client.DevicePost("/api/iot/attendance/toggle", body, primaryDevice.APIKey, state.DevicePIN)
			if err != nil {
				if opts.Verbose {
					fmt.Printf("  WARNING: daily checkout failed for student %d: %v\n", student.ID, err)
				}
				continue
			}
			dailyCheckoutCount++
		}
		fmt.Printf("  %d daily checkouts\n", dailyCheckoutCount)

		// End all sessions
		sessionsEnded := 0
		for i := 0; i < sessionsStarted && i < len(deviceKeys); i++ {
			device := state.Devices[deviceKeys[i]]
			_, err := client.DevicePost("/api/iot/session/end", nil, device.APIKey, state.DevicePIN)
			if err != nil {
				if opts.Verbose {
					fmt.Printf("  WARNING: failed to end session on device %s: %v\n", deviceKeys[i], err)
				}
				continue
			}
			sessionsEnded++
		}
		fmt.Printf("  %d sessions ended\n", sessionsEnded)
	}

	// Summary
	fmt.Println("\n--- Simulation Summary ---")
	fmt.Printf("  RFID tags assigned:  %d\n", rfidAssigned)
	fmt.Printf("  Sessions started:    %d\n", sessionsStarted)
	fmt.Printf("  Attendance records:  %d\n", attendanceCount)
	fmt.Printf("  Students checked in: %d\n", checkinCount)
	fmt.Printf("  Students sick:       %d\n", sickCount)
	fmt.Printf("  Students checked out:%d\n", checkoutCount)
	if opts.Close {
		fmt.Println("  End-of-day:          completed")
	}
	fmt.Println("--------------------------")

	return nil
}

// findRoomForActivity maps activity names to their default rooms using the demo data convention.
func findRoomForActivity(activityName string, rooms map[string]int64) int64 {
	activityRoomMap := map[string]string{
		"Hausaufgaben": "OGS-Raum 1",
		"Fußball":      "Sporthalle",
		"Basteln":      "Kreativraum",
		"Kochen":       "Mensa",
		"Lesen":        "OGS-Raum 2",
		"Musik":        "OGS-Raum 1",
		"Tanzen":       "Sporthalle",
		"Schach":       "OGS-Raum 2",
		"Garten":       "Schulhof",
		"Freispiel":    "Schulhof",
	}

	roomName, ok := activityRoomMap[activityName]
	if !ok {
		return 0
	}
	return rooms[roomName]
}

func sortedDeviceKeys(devices map[string]seedapi.SeedDevice) []string {
	keys := make([]string, 0, len(devices))
	for k := range devices {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedStringKeys(m map[string]int64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
