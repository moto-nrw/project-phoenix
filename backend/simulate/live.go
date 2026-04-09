package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
)

// LiveOptions configures the continuous live simulation.
type LiveOptions struct {
	StatePath string
	Interval  time.Duration
	Verbose   bool
}

// liveCounts tracks actions performed during the live simulation.
type liveCounts struct {
	roomMoves  int
	unterwegs  int
	returns    int
	sickToggle int
	errors     int
}

// liveState tracks the mutable simulation state.
type liveState struct {
	checkedIn map[int64]bool   // student IDs currently in a room
	unterwegs map[int64]bool   // student IDs checked out but still in building
	sick      map[int64]bool   // student IDs currently marked sick
	rfidTags  map[int64]string // student ID → RFID tag
	roomIDs   []int64
}

// RunLive runs the continuous live simulation loop.
func RunLive(ctx context.Context, opts LiveOptions) error {
	state, err := seedapi.LoadSeedState(opts.StatePath)
	if err != nil {
		return fmt.Errorf("load seed state: %w", err)
	}

	client := NewClient(state.BaseURL, opts.Verbose)

	// Login as admin
	if len(state.Accounts.Admin) == 0 {
		return fmt.Errorf("no admin accounts in seed state")
	}
	admin := state.Accounts.Admin[0]
	if err := client.Login(admin.Email, admin.Password); err != nil {
		return fmt.Errorf("admin login: %w", err)
	}

	deviceKeys := sortedDeviceKeys(state.Devices)
	if len(deviceKeys) == 0 {
		return fmt.Errorf("no devices in seed state")
	}
	primaryDevice := state.Devices[deviceKeys[0]]

	// Build RFID tag map
	rfidTags := make(map[int64]string, len(state.Students))
	for _, s := range state.Students {
		rfidTags[s.ID] = fmt.Sprintf("DE%06X", s.ID)
	}

	// Collect room IDs
	var roomIDs []int64
	for _, id := range state.Rooms {
		roomIDs = append(roomIDs, id)
	}
	if len(roomIDs) == 0 {
		return fmt.Errorf("no rooms in seed state")
	}

	// Bootstrap state from server
	ls := &liveState{
		checkedIn: make(map[int64]bool),
		unterwegs: make(map[int64]bool),
		sick:      make(map[int64]bool),
		rfidTags:  rfidTags,
		roomIDs:   roomIDs,
	}
	if err := bootstrapLiveState(client, ls, state.Students); err != nil {
		fmt.Printf("  WARNING: could not bootstrap live state: %v\n", err)
		fmt.Println("  Assuming all students with RFID tags are checked in")
		for _, s := range state.Students {
			if _, ok := rfidTags[s.ID]; ok {
				ls.checkedIn[s.ID] = true
			}
		}
	} else if len(ls.checkedIn) == 0 {
		return fmt.Errorf("no active visits found; run simulate full-day first and do not close sessions before starting live mode")
	}

	if opts.Interval <= 0 {
		return fmt.Errorf("interval must be greater than 0")
	}

	fmt.Printf("Live simulation started (interval: %s, Ctrl+C to stop)\n", opts.Interval)
	fmt.Printf("  Students checked in: %d, unterwegs: %d\n\n", len(ls.checkedIn), len(ls.unterwegs))

	counts := &liveCounts{}
	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	// Re-login before JWT expires (default 1h, refresh at 50min)
	reloginTicker := time.NewTicker(50 * time.Minute)
	defer reloginTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			printLiveSummary(counts)
			return nil
		case <-reloginTicker.C:
			if err := client.Login(admin.Email, admin.Password); err != nil {
				fmt.Printf("[%s] WARNING: JWT refresh login failed: %v\n", time.Now().Format("15:04:05"), err)
			}
		case <-ticker.C:
			runLiveTick(client, ls, state, primaryDevice, counts)
		}
	}
}

func runLiveTick(client *Client, ls *liveState, state *seedapi.SeedState, device seedapi.SeedDevice, counts *liveCounts) {
	now := time.Now().Format("15:04:05")

	// Keep session alive (mirrors PyrePortal's periodic ping)
	_, _ = client.DevicePost("/api/iot/ping", nil, device.APIKey, state.DevicePIN)

	// Weighted random action selection
	roll := rand.Intn(100)
	switch {
	case roll < 50:
		// Room move (50%)
		if err := liveRoomMove(client, ls, state, device, now); err != nil {
			counts.errors++
			return
		}
		counts.roomMoves++

	case roll < 65:
		// Go unterwegs (15%)
		if err := liveGoUnterwegs(client, ls, state, device, now); err != nil {
			counts.errors++
			return
		}
		counts.unterwegs++

	case roll < 90:
		// Return from unterwegs (25%)
		if err := liveReturnFromUnterwegs(client, ls, state, device, now); err != nil {
			counts.errors++
			return
		}
		counts.returns++

	default:
		// Toggle sick (10%)
		if err := liveToggleSick(client, ls, state, now); err != nil {
			counts.errors++
			return
		}
		counts.sickToggle++
	}
}

func liveRoomMove(client *Client, ls *liveState, state *seedapi.SeedState, device seedapi.SeedDevice, now string) error {
	studentID := randomFromSet(ls.checkedIn)
	if studentID == 0 {
		return fmt.Errorf("no checked-in students")
	}
	rfid := ls.rfidTags[studentID]
	student := findStudent(state.Students, studentID)

	// Checkout
	_, err := client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": rfid,
		"action":       "checkout",
	}, device.APIKey, state.DevicePIN)
	if err != nil {
		fmt.Printf("[%s] ERROR room move checkout %s %s: %v\n", now, student.FirstName, student.LastName, err)
		return err
	}

	// Checkin to a different room
	roomID := ls.roomIDs[rand.Intn(len(ls.roomIDs))]
	_, err = client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": rfid,
		"action":       "checkin",
		"room_id":      roomID,
	}, device.APIKey, state.DevicePIN)
	if err != nil {
		fmt.Printf("[%s] ERROR room move checkin %s %s: %v\n", now, student.FirstName, student.LastName, err)
		// Student is now unterwegs since checkout succeeded
		delete(ls.checkedIn, studentID)
		ls.unterwegs[studentID] = true
		return err
	}

	roomName := roomNameByID(state.Rooms, roomID)
	fmt.Printf("[%s] moved %s %s → %s\n", now, student.FirstName, student.LastName, roomName)
	return nil
}

func liveGoUnterwegs(client *Client, ls *liveState, state *seedapi.SeedState, device seedapi.SeedDevice, now string) error {
	studentID := randomFromSet(ls.checkedIn)
	if studentID == 0 {
		return fmt.Errorf("no checked-in students")
	}
	rfid := ls.rfidTags[studentID]
	student := findStudent(state.Students, studentID)

	_, err := client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": rfid,
		"action":       "checkout",
	}, device.APIKey, state.DevicePIN)
	if err != nil {
		fmt.Printf("[%s] ERROR unterwegs %s %s: %v\n", now, student.FirstName, student.LastName, err)
		return err
	}

	delete(ls.checkedIn, studentID)
	ls.unterwegs[studentID] = true
	fmt.Printf("[%s] %s %s → unterwegs\n", now, student.FirstName, student.LastName)
	return nil
}

func liveReturnFromUnterwegs(client *Client, ls *liveState, state *seedapi.SeedState, device seedapi.SeedDevice, now string) error {
	studentID := randomFromSet(ls.unterwegs)
	if studentID == 0 {
		// Nobody unterwegs — check in a random checked-in student to a new room instead
		return liveRoomMove(client, ls, state, device, now)
	}
	rfid := ls.rfidTags[studentID]
	student := findStudent(state.Students, studentID)

	roomID := ls.roomIDs[rand.Intn(len(ls.roomIDs))]
	_, err := client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": rfid,
		"action":       "checkin",
		"room_id":      roomID,
	}, device.APIKey, state.DevicePIN)
	if err != nil {
		fmt.Printf("[%s] ERROR return %s %s: %v\n", now, student.FirstName, student.LastName, err)
		return err
	}

	delete(ls.unterwegs, studentID)
	ls.checkedIn[studentID] = true
	roomName := roomNameByID(state.Rooms, roomID)
	fmt.Printf("[%s] %s %s ← returned to %s\n", now, student.FirstName, student.LastName, roomName)
	return nil
}

func liveToggleSick(client *Client, ls *liveState, state *seedapi.SeedState, now string) error {
	if len(state.Students) == 0 {
		return fmt.Errorf("no students")
	}
	student := state.Students[rand.Intn(len(state.Students))]
	newSick := !ls.sick[student.ID]

	_, err := client.AdminPut(fmt.Sprintf("/api/students/%d", student.ID), map[string]any{
		"sick": newSick,
	})
	if err != nil {
		fmt.Printf("[%s] ERROR toggle sick %s %s: %v\n", now, student.FirstName, student.LastName, err)
		return err
	}

	ls.sick[student.ID] = newSick
	status := "gesund"
	if newSick {
		status = "krank"
	}
	fmt.Printf("[%s] %s %s → %s\n", now, student.FirstName, student.LastName, status)
	return nil
}

func bootstrapLiveState(client *Client, ls *liveState, students []seedapi.SeedStudent) error {
	resp, err := client.AdminGet("/api/active/visits?active=true")
	if err != nil {
		return err
	}

	var envelope struct {
		Data []struct {
			StudentID int64 `json:"student_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &envelope); err != nil {
		return fmt.Errorf("parse visits: %w", err)
	}

	visitSet := make(map[int64]bool, len(envelope.Data))
	for _, v := range envelope.Data {
		visitSet[v.StudentID] = true
	}

	for _, s := range students {
		if _, hasRFID := ls.rfidTags[s.ID]; !hasRFID {
			continue
		}
		if visitSet[s.ID] {
			ls.checkedIn[s.ID] = true
		}
	}

	return nil
}

func randomFromSet(set map[int64]bool) int64 {
	if len(set) == 0 {
		return 0
	}
	n := rand.Intn(len(set))
	i := 0
	for id := range set {
		if i == n {
			return id
		}
		i++
	}
	return 0
}

func findStudent(students []seedapi.SeedStudent, id int64) seedapi.SeedStudent {
	for _, s := range students {
		if s.ID == id {
			return s
		}
	}
	return seedapi.SeedStudent{ID: id, FirstName: "Unknown", LastName: "Student"}
}

func roomNameByID(rooms map[string]int64, targetID int64) string {
	for name, id := range rooms {
		if id == targetID {
			return name
		}
	}
	return fmt.Sprintf("room-%d", targetID)
}

func printLiveSummary(counts *liveCounts) {
	fmt.Println("\n--- Live Simulation Summary ---")
	fmt.Printf("  Room moves:    %d\n", counts.roomMoves)
	fmt.Printf("  Unterwegs:     %d\n", counts.unterwegs)
	fmt.Printf("  Returns:       %d\n", counts.returns)
	fmt.Printf("  Sick toggles:  %d\n", counts.sickToggle)
	fmt.Printf("  Errors:        %d\n", counts.errors)
	total := counts.roomMoves + counts.unterwegs + counts.returns + counts.sickToggle
	fmt.Printf("  Total actions: %d\n", total)
	fmt.Println("-------------------------------")
}
