package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
)

// RuntimeSeeder creates runtime state (active sessions, checked-in students) via IoT APIs
type RuntimeSeeder struct {
	client        *Client
	fixedSeeder   *FixedSeeder
	verbose       bool
	deviceAPIKeys []string // API keys for device authentication (multiple devices)
	staffPIN      string   // PIN for device authentication
	activeRoomIDs []int64  // Room IDs with active sessions (for check-ins)
}

// RuntimeResult contains counts of runtime state created
type RuntimeResult struct {
	ActiveSessions    int
	CheckedInStudents int
	StudentsUnterwegs int
	RFIDsAssigned     int
}

// NewRuntimeSeeder creates a new runtime state seeder
func NewRuntimeSeeder(client *Client, fixedSeeder *FixedSeeder, verbose bool, staffPIN string) *RuntimeSeeder {
	return &RuntimeSeeder{
		client:      client,
		fixedSeeder: fixedSeeder,
		verbose:     verbose,
		staffPIN:    staffPIN,
	}
}

// Seed creates runtime state based on configuration
func (s *RuntimeSeeder) Seed(ctx context.Context, config RuntimeConfig) (*RuntimeResult, error) {
	result := &RuntimeResult{}

	fmt.Println("🎬 Creating Runtime State...")

	// 1. Get device API key from fixed seeder
	if err := s.setupDeviceAuth(); err != nil {
		return nil, fmt.Errorf("failed to setup device auth: %w", err)
	}

	// 2. Assign RFID tags to students (required for check-in)
	if err := s.assignRFIDTags(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to assign RFID tags: %w", err)
	}

	// 3. Start activity sessions
	if err := s.startSessions(ctx, config, result); err != nil {
		return nil, fmt.Errorf("failed to start sessions: %w", err)
	}

	// 4. Check in students
	if err := s.checkinStudents(ctx, config, result); err != nil {
		return nil, fmt.Errorf("failed to check in students: %w", err)
	}

	fmt.Println("✅ Runtime state creation complete!")
	return result, nil
}

func (s *RuntimeSeeder) setupDeviceAuth() error {
	// Collect all device API keys from the fixed seeder
	for _, apiKey := range s.fixedSeeder.deviceKeys {
		s.deviceAPIKeys = append(s.deviceAPIKeys, apiKey)
	}
	if len(s.deviceAPIKeys) == 0 {
		return fmt.Errorf("no device API keys available")
	}
	if s.verbose {
		fmt.Printf("  ✓ Using %d device API keys for IoT operations\n", len(s.deviceAPIKeys))
	}
	return nil
}

func (s *RuntimeSeeder) assignRFIDTags(ctx context.Context, result *RuntimeResult) error {
	// Assign RFID tags to all students via device-authenticated API
	for studentName, studentID := range s.fixedSeeder.studentIDs {
		// Generate a proper hexadecimal RFID tag (8 characters minimum)
		// Using format: DE + 6 hex digits derived from student ID
		rfidTag := fmt.Sprintf("DE%06X", studentID)

		// Assign via device API
		path := fmt.Sprintf("/api/students/%d/rfid", studentID)
		body := map[string]string{
			"rfid_tag": rfidTag,
		}

		if err := s.devicePost(path, body); err != nil {
			return fmt.Errorf("failed to assign RFID to %s: %w", studentName, err)
		}

		// Store for check-in use
		s.fixedSeeder.studentRFID[studentID] = rfidTag
		result.RFIDsAssigned++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d RFID tags assigned\n", result.RFIDsAssigned)
	}
	return nil
}

func (s *RuntimeSeeder) startSessions(ctx context.Context, config RuntimeConfig, result *RuntimeResult) error {
	// Select random activities to start
	activityIDs := make([]int64, 0, len(s.fixedSeeder.activityIDs))
	for _, id := range s.fixedSeeder.activityIDs {
		activityIDs = append(activityIDs, id)
	}

	// Shuffle and take first N
	rand.Shuffle(len(activityIDs), func(i, j int) {
		activityIDs[i], activityIDs[j] = activityIDs[j], activityIDs[i]
	})

	// Limit sessions to available devices (each device can only run 1 session)
	sessionsToStart := config.ActiveSessions
	if sessionsToStart > len(activityIDs) {
		sessionsToStart = len(activityIDs)
	}
	if sessionsToStart > len(s.deviceAPIKeys) {
		sessionsToStart = len(s.deviceAPIKeys)
	}

	// Collect staff IDs for rotation (different supervisor per session)
	staffIDs := make([]int64, 0, len(s.fixedSeeder.staffIDs))
	for _, id := range s.fixedSeeder.staffIDs {
		staffIDs = append(staffIDs, id)
	}

	// Start sessions (each session uses a different device and supervisor)
	for i := 0; i < sessionsToStart; i++ {
		activityID := activityIDs[i]
		deviceKey := s.deviceAPIKeys[i]
		supervisorID := staffIDs[i%len(staffIDs)] // Rotate through supervisors

		body := map[string]any{
			"activity_id":    activityID,
			"supervisor_ids": []int64{supervisorID},
		}

		_, err := s.devicePostWithKey("/api/iot/session/start", body, deviceKey)
		if err != nil {
			return fmt.Errorf("failed to start session for activity %d: %w", activityID, err)
		}

		// Track room ID for this session (used for check-ins)
		if roomID, ok := s.fixedSeeder.activityRoomIDs[activityID]; ok {
			s.activeRoomIDs = append(s.activeRoomIDs, roomID)
		}

		result.ActiveSessions++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d activity sessions started\n", result.ActiveSessions)
	}
	return nil
}

func (s *RuntimeSeeder) checkinStudents(ctx context.Context, config RuntimeConfig, result *RuntimeResult) error {
	// Get list of all students with RFID tags
	studentIDs := make([]int64, 0, len(s.fixedSeeder.studentRFID))
	for id := range s.fixedSeeder.studentRFID {
		studentIDs = append(studentIDs, id)
	}

	// Shuffle students
	rand.Shuffle(len(studentIDs), func(i, j int) {
		studentIDs[i], studentIDs[j] = studentIDs[j], studentIDs[i]
	})

	// Only use rooms with active sessions for check-ins
	if len(s.activeRoomIDs) == 0 {
		return fmt.Errorf("no active rooms available for check-ins")
	}

	// Calculate how many to check in (leave some unterwegs)
	toCheckin := config.CheckedInStudents
	if toCheckin > len(studentIDs)-config.StudentsUnterwegs {
		toCheckin = len(studentIDs) - config.StudentsUnterwegs
	}

	// Check in students, distributing across rooms with active sessions only
	for i := 0; i < toCheckin; i++ {
		studentID := studentIDs[i]
		rfidTag := s.fixedSeeder.studentRFID[studentID]
		// Distribute students across active rooms only (round-robin)
		roomID := s.activeRoomIDs[i%len(s.activeRoomIDs)]

		body := map[string]interface{}{
			"student_rfid": rfidTag,
			"action":       "checkin",
			"room_id":      roomID,
		}

		_, err := s.devicePostWithResponse("/api/iot/checkin", body)
		if err != nil {
			// Log but don't fail - some students might not be eligible
			if s.verbose {
				fmt.Printf("    Warning: failed to check in student %d: %v\n", studentID, err)
			}
			continue
		}

		result.CheckedInStudents++
	}

	// Set unterwegs count from config (students intentionally not checked in)
	result.StudentsUnterwegs = config.StudentsUnterwegs

	if s.verbose {
		fmt.Printf("  ✓ %d students checked in\n", result.CheckedInStudents)
	}
	return nil
}

// devicePost makes a POST request with device authentication using first device key
func (s *RuntimeSeeder) devicePost(path string, body any) error {
	_, err := s.devicePostWithKey(path, body, s.deviceAPIKeys[0])
	return err
}

// devicePostWithResponse makes a POST request with device authentication using first device key
func (s *RuntimeSeeder) devicePostWithResponse(path string, body any) ([]byte, error) {
	return s.devicePostWithKey(path, body, s.deviceAPIKeys[0])
}

// devicePostWithKey makes a POST request with a specific device API key
func (s *RuntimeSeeder) devicePostWithKey(path string, body any, deviceAPIKey string) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", s.client.baseURL+path, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add device authentication headers
	req.Header.Set("Authorization", "Bearer "+deviceAPIKey)
	req.Header.Set("X-Staff-PIN", s.staffPIN)
	req.Header.Set("Content-Type", "application/json")

	// Log request with device auth context
	if s.verbose {
		logAPIRequest("POST", path, body, "device auth")
	}

	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Log response
	if s.verbose {
		logAPIResponse(resp.StatusCode, respBody)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
