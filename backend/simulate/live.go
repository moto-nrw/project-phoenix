package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// LiveOptions configures the continuous live simulation.
type LiveOptions struct {
	StatePath string
	Profile   string
	Interval  time.Duration
	Verbose   bool
	Client    ClientFactory
}

// visitCooldown is the minimum pause between rotation events for the same
// student (mirrors the retired simulator/iot engine).
const visitCooldown = 3 * time.Second

// rotationPhase values for the Schulhof rotation cycle.
const (
	phaseHeimatraum = "heimatraum"
	phaseAG         = "ag"
	phaseSchulhof   = "schulhof"
)

// liveCounts tracks actions performed during the live simulation.
type liveCounts struct {
	roomMoves       int
	unterwegs       int
	returns         int
	sickToggle      int
	schulhofRotates int
	attendance      int
	supervisorSwaps int
	errors          int
}

// rotationState tracks a student's position in the Heimatraum → AG → Schulhof cycle.
type rotationState struct {
	phase         string
	agHops        int
	agHopTarget   int
	cooldownUntil time.Time
}

// liveState tracks the mutable simulation state.
type liveState struct {
	checkedIn map[int64]bool   // student IDs currently in a room
	unterwegs map[int64]bool   // student IDs checked out but still in building
	sick      map[int64]bool   // student IDs currently marked sick
	rfidTags  map[int64]string // student ID → RFID tag
	roomIDs   []int64

	rotation       map[int64]*rotationState // Schulhof rotation per student
	lastAttendance map[int64]time.Time      // attendance-toggle cooldown per student
	schulhofRoomID int64                    // 0 = system room not found → fall back to random room
	sessionID      int64                    // active session for supervisor swaps (0 = unknown)
	staffIDs       []int64                  // supervisor pool from seed-state accounts
	interval       time.Duration
}

// RunLive runs the continuous live simulation loop.
func RunLive(ctx context.Context, opts LiveOptions) error {
	if opts.Client == nil {
		return fmt.Errorf("simulation client factory is required")
	}
	state, err := LoadSeedStateProfile(opts.StatePath, opts.Profile)
	if err != nil {
		return fmt.Errorf("load seed state: %w", err)
	}

	client, err := buildClient(opts.Client, state.BaseURL, opts.Verbose)
	if err != nil {
		return err
	}

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
		checkedIn:      make(map[int64]bool),
		unterwegs:      make(map[int64]bool),
		sick:           make(map[int64]bool),
		rfidTags:       rfidTags,
		roomIDs:        roomIDs,
		rotation:       make(map[int64]*rotationState),
		lastAttendance: make(map[int64]time.Time),
		staffIDs:       collectStaffIDs(state),
		interval:       opts.Interval,
	}
	ls.schulhofRoomID = lookupSchulhofRoom(client)
	if ls.schulhofRoomID == 0 {
		fmt.Println("  NOTE: Schulhof room not found; rotation uses random rooms for the Schulhof leg")
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

func runLiveTick(client Client, ls *liveState, state *SeedState, device SeedDevice, counts *liveCounts) {
	now := time.Now().Format("15:04:05")

	// Keep session alive (mirrors PyrePortal's periodic ping)
	_, _ = client.DevicePost("/api/iot/ping", nil, device.APIKey, state.DevicePIN)

	// Weighted random action selection
	roll := rand.Intn(100)
	switch {
	case roll < 35:
		// Room move (35%)
		if err := liveRoomMove(client, ls, state, device, now); err != nil {
			counts.errors++
			return
		}
		counts.roomMoves++

	case roll < 45:
		// Go unterwegs (10%)
		if err := liveGoUnterwegs(client, ls, state, device, now); err != nil {
			counts.errors++
			return
		}
		counts.unterwegs++

	case roll < 65:
		// Return from unterwegs (20%)
		if err := liveReturnFromUnterwegs(client, ls, state, device, now); err != nil {
			counts.errors++
			return
		}
		counts.returns++

	case roll < 70:
		// Toggle sick (5%)
		if err := liveToggleSick(client, ls, state, now); err != nil {
			counts.errors++
			return
		}
		counts.sickToggle++

	case roll < 85:
		// Schulhof rotation (15%)
		if err := liveSchulhofRotate(client, ls, state, device, now); err != nil {
			counts.errors++
			return
		}
		counts.schulhofRotates++

	case roll < 95:
		// Attendance toggle (10%)
		if err := liveAttendanceToggle(client, ls, state, device, now); err != nil {
			counts.errors++
			return
		}
		counts.attendance++

	default:
		// Supervisor swap (5%)
		if err := liveSupervisorSwap(client, ls, state, device, now); err != nil {
			counts.errors++
			return
		}
		counts.supervisorSwaps++
	}
}

func liveRoomMove(client Client, ls *liveState, state *SeedState, device SeedDevice, now string) error {
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

func liveGoUnterwegs(client Client, ls *liveState, state *SeedState, device SeedDevice, now string) error {
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

func liveReturnFromUnterwegs(client Client, ls *liveState, state *SeedState, device SeedDevice, now string) error {
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

func liveToggleSick(client Client, ls *liveState, state *SeedState, now string) error {
	if len(state.Students) == 0 {
		return fmt.Errorf("no students")
	}
	student := state.Students[rand.Intn(len(state.Students))]
	newSick := !ls.sick[student.ID]

	_, err := client.Put(fmt.Sprintf("/api/students/%d", student.ID), map[string]any{
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

// liveSchulhofRotate advances a random student through the
// Heimatraum → AG (1-2 hops) → Schulhof → Heimatraum cycle.
func liveSchulhofRotate(client Client, ls *liveState, state *SeedState, device SeedDevice, now string) error {
	studentID := ls.pickRotationCandidate()
	if studentID == 0 {
		return fmt.Errorf("no rotation-eligible students")
	}
	rfid := ls.rfidTags[studentID]
	student := findStudent(state.Students, studentID)
	rot := ls.rotationFor(studentID)

	roomID, nextPhase := ls.nextRotationStep(rot)

	// Checkout current room, then checkin to the next phase's room
	_, err := client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": rfid,
		"action":       "checkout",
	}, device.APIKey, state.DevicePIN)
	if err != nil {
		fmt.Printf("[%s] ERROR rotation checkout %s %s: %v\n", now, student.FirstName, student.LastName, err)
		rot.cooldownUntil = time.Now().Add(visitCooldown)
		return err
	}

	_, err = client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": rfid,
		"action":       "checkin",
		"room_id":      roomID,
	}, device.APIKey, state.DevicePIN)
	if err != nil {
		fmt.Printf("[%s] ERROR rotation checkin %s %s: %v\n", now, student.FirstName, student.LastName, err)
		// Checkout succeeded, so the student is now unterwegs
		delete(ls.checkedIn, studentID)
		ls.unterwegs[studentID] = true
		rot.cooldownUntil = time.Now().Add(visitCooldown)
		return err
	}

	ls.applyRotationStep(rot, nextPhase)
	fmt.Printf("[%s] %s %s → %s (%s)\n", now, student.FirstName, student.LastName, roomNameByID(state.Rooms, roomID), nextPhase)
	return nil
}

// pickRotationCandidate returns a random checked-in student whose rotation
// cooldown has elapsed (0 = none available).
func (ls *liveState) pickRotationCandidate() int64 {
	nowTime := time.Now()
	eligible := make(map[int64]bool, len(ls.checkedIn))
	for id := range ls.checkedIn {
		if rot, ok := ls.rotation[id]; ok && rot.cooldownUntil.After(nowTime) {
			continue
		}
		eligible[id] = true
	}
	return randomFromSet(eligible)
}

// rotationFor returns the student's rotation state, initializing it lazily.
func (ls *liveState) rotationFor(studentID int64) *rotationState {
	if ls.rotation == nil {
		ls.rotation = make(map[int64]*rotationState)
	}
	rot, ok := ls.rotation[studentID]
	if !ok {
		rot = &rotationState{phase: phaseHeimatraum, agHopTarget: randAGHopTarget()}
		ls.rotation[studentID] = rot
	}
	return rot
}

// nextRotationStep decides the target room and next phase for a student.
func (ls *liveState) nextRotationStep(rot *rotationState) (roomID int64, nextPhase string) {
	switch {
	case rot.phase == phaseAG && rot.agHops >= rot.agHopTarget:
		roomID = ls.schulhofRoomID
		if roomID == 0 {
			roomID = ls.roomIDs[rand.Intn(len(ls.roomIDs))]
		}
		return roomID, phaseSchulhof
	case rot.phase == phaseSchulhof:
		return ls.roomIDs[rand.Intn(len(ls.roomIDs))], phaseHeimatraum
	default: // heimatraum, or ag with hops remaining → next AG hop
		return ls.roomIDs[rand.Intn(len(ls.roomIDs))], phaseAG
	}
}

// applyRotationStep records a successful rotation move.
func (ls *liveState) applyRotationStep(rot *rotationState, nextPhase string) {
	switch nextPhase {
	case phaseAG:
		rot.agHops++
	case phaseHeimatraum:
		rot.agHops = 0
		rot.agHopTarget = randAGHopTarget()
	}
	rot.phase = nextPhase
	rot.cooldownUntil = time.Now().Add(visitCooldown)
}

// randAGHopTarget mirrors the seed-generated engine config (min 1, max 2 AG hops).
func randAGHopTarget() int {
	return 1 + rand.Intn(2)
}

// liveAttendanceToggle confirms attendance for a random student, rate-limited
// per student to one toggle per tick interval.
func liveAttendanceToggle(client Client, ls *liveState, state *SeedState, device SeedDevice, now string) error {
	cutoff := time.Now().Add(-ls.interval)
	eligible := make(map[int64]bool, len(ls.rfidTags))
	for id := range ls.rfidTags {
		if last, ok := ls.lastAttendance[id]; ok && last.After(cutoff) {
			continue
		}
		eligible[id] = true
	}
	studentID := randomFromSet(eligible)
	if studentID == 0 {
		return fmt.Errorf("no attendance-eligible students")
	}
	student := findStudent(state.Students, studentID)

	if ls.lastAttendance == nil {
		ls.lastAttendance = make(map[int64]time.Time)
	}
	_, err := client.DevicePost("/api/iot/attendance/toggle", map[string]string{
		"rfid":   ls.rfidTags[studentID],
		"action": "confirm",
	}, device.APIKey, state.DevicePIN)
	ls.lastAttendance[studentID] = time.Now()
	if err != nil {
		fmt.Printf("[%s] ERROR attendance toggle %s %s: %v\n", now, student.FirstName, student.LastName, err)
		return err
	}

	fmt.Printf("[%s] %s %s → Anwesenheit bestätigt\n", now, student.FirstName, student.LastName)
	return nil
}

// liveSupervisorSwap reassigns a random supervisor to the primary device's session.
func liveSupervisorSwap(client Client, ls *liveState, state *SeedState, device SeedDevice, now string) error {
	if len(ls.staffIDs) == 0 {
		return fmt.Errorf("no staff IDs in seed state")
	}

	if ls.sessionID == 0 {
		sessionID, err := fetchCurrentSessionID(client, state, device)
		if err != nil {
			fmt.Printf("[%s] ERROR supervisor swap (session lookup): %v\n", now, err)
			return err
		}
		if sessionID == 0 {
			return fmt.Errorf("no active session on primary device")
		}
		ls.sessionID = sessionID
	}

	staffID := ls.staffIDs[rand.Intn(len(ls.staffIDs))]
	_, err := client.DevicePut(fmt.Sprintf("/api/iot/session/%d/supervisors", ls.sessionID), map[string]any{
		"supervisor_ids": []int64{staffID},
	}, device.APIKey, state.DevicePIN)
	if err != nil {
		fmt.Printf("[%s] ERROR supervisor swap session %d: %v\n", now, ls.sessionID, err)
		// Session may have ended — re-discover it on the next attempt
		ls.sessionID = 0
		return err
	}

	fmt.Printf("[%s] session %d → supervisor %d\n", now, ls.sessionID, staffID)
	return nil
}

// fetchCurrentSessionID queries the device's current session (0 = none active).
func fetchCurrentSessionID(client Client, state *SeedState, device SeedDevice) (int64, error) {
	resp, err := client.DeviceGet("/api/iot/session/current", device.APIKey, state.DevicePIN)
	if err != nil {
		return 0, err
	}
	var envelope struct {
		Data struct {
			ActiveGroupID *int64 `json:"active_group_id"`
			IsActive      bool   `json:"is_active"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &envelope); err != nil {
		return 0, fmt.Errorf("parse current session: %w", err)
	}
	if !envelope.Data.IsActive || envelope.Data.ActiveGroupID == nil {
		return 0, nil
	}
	return *envelope.Data.ActiveGroupID, nil
}

// lookupSchulhofRoom finds the auto-provisioned Schulhof system room (0 = not found).
func lookupSchulhofRoom(client Client) int64 {
	resp, err := client.Get("/api/rooms?include_system=true&page_size=100")
	if err != nil {
		return 0
	}
	var envelope struct {
		Data []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &envelope); err != nil {
		return 0
	}
	for _, room := range envelope.Data {
		if room.Name == "Schulhof" {
			return room.ID
		}
	}
	return 0
}

// collectStaffIDs gathers the supervisor pool from seed-state accounts.
func collectStaffIDs(state *SeedState) []int64 {
	ids := make([]int64, 0, len(state.Accounts.Betreuer)+len(state.Accounts.Admin))
	for _, acc := range state.Accounts.Betreuer {
		if acc.StaffID > 0 {
			ids = append(ids, acc.StaffID)
		}
	}
	for _, acc := range state.Accounts.Admin {
		if acc.StaffID > 0 {
			ids = append(ids, acc.StaffID)
		}
	}
	return ids
}

func bootstrapLiveState(client Client, ls *liveState, students []SeedStudent) error {
	resp, err := client.Get("/api/active/visits?active=true")
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

func findStudent(students []SeedStudent, id int64) SeedStudent {
	for _, s := range students {
		if s.ID == id {
			return s
		}
	}
	return SeedStudent{ID: id, FirstName: "Unknown", LastName: "Student"}
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
	fmt.Printf("  Room moves:       %d\n", counts.roomMoves)
	fmt.Printf("  Unterwegs:        %d\n", counts.unterwegs)
	fmt.Printf("  Returns:          %d\n", counts.returns)
	fmt.Printf("  Sick toggles:     %d\n", counts.sickToggle)
	fmt.Printf("  Schulhof rotates: %d\n", counts.schulhofRotates)
	fmt.Printf("  Attendance:       %d\n", counts.attendance)
	fmt.Printf("  Supervisor swaps: %d\n", counts.supervisorSwaps)
	fmt.Printf("  Errors:           %d\n", counts.errors)
	total := counts.roomMoves + counts.unterwegs + counts.returns + counts.sickToggle +
		counts.schulhofRotates + counts.attendance + counts.supervisorSwaps
	fmt.Printf("  Total actions:    %d\n", total)
	fmt.Println("-------------------------------")
}
