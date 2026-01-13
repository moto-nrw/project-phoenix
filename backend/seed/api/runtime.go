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
	client       *Client
	fixedSeeder  *FixedSeeder
	verbose      bool
	deviceAPIKey string // API key for device authentication
	staffPIN     string // PIN for device authentication
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
	// Get the first device's API key from the fixed seeder
	for _, apiKey := range s.fixedSeeder.deviceKeys {
		s.deviceAPIKey = apiKey
		if s.verbose {
			fmt.Printf("  ✓ Using device API key for IoT operations\n")
		}
		return nil
	}
	return fmt.Errorf("no device API keys available")
}

func (s *RuntimeSeeder) assignRFIDTags(ctx context.Context, result *RuntimeResult) error {
	// Assign RFID tags to all students via device-authenticated API
	for studentName, studentID := range s.fixedSeeder.studentIDs {
		// Generate a simple hex RFID tag (8 characters minimum)
		rfidTag := fmt.Sprintf("DEMO%04d", studentID)

		// Assign via device API
		path := fmt.Sprintf("/students/%d/rfid", studentID)
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

	sessionsToStart := config.ActiveSessions
	if sessionsToStart > len(activityIDs) {
		sessionsToStart = len(activityIDs)
	}

	// Get first staff ID for supervisor
	var firstStaffID int64
	for _, id := range s.fixedSeeder.staffIDs {
		firstStaffID = id
		break
	}

	// Start sessions
	for i := 0; i < sessionsToStart; i++ {
		activityID := activityIDs[i]

		body := map[string]interface{}{
			"activity_id":    activityID,
			"supervisor_ids": []int64{firstStaffID},
		}

		_, err := s.devicePostWithResponse("/iot/session/start", body)
		if err != nil {
			return fmt.Errorf("failed to start session for activity %d: %w", activityID, err)
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

	// Get a room ID for check-in
	var roomID int64
	for _, id := range s.fixedSeeder.roomIDs {
		roomID = id
		break
	}

	// Calculate how many to check in
	toCheckin := config.CheckedInStudents
	if toCheckin > len(studentIDs) {
		toCheckin = len(studentIDs)
	}

	// Check in students
	for i := 0; i < toCheckin; i++ {
		studentID := studentIDs[i]
		rfidTag := s.fixedSeeder.studentRFID[studentID]

		body := map[string]interface{}{
			"student_rfid": rfidTag,
			"action":       "checkin",
			"room_id":      roomID,
		}

		_, err := s.devicePostWithResponse("/iot/checkin", body)
		if err != nil {
			// Log but don't fail - some students might not be eligible
			if s.verbose {
				fmt.Printf("    Warning: failed to check in student %d: %v\n", studentID, err)
			}
			continue
		}

		result.CheckedInStudents++
	}

	// Calculate unterwegs count
	result.StudentsUnterwegs = len(studentIDs) - result.CheckedInStudents

	if s.verbose {
		fmt.Printf("  ✓ %d students checked in\n", result.CheckedInStudents)
	}
	return nil
}

// devicePost makes a POST request with device authentication (no response body parsing)
func (s *RuntimeSeeder) devicePost(path string, body any) error {
	_, err := s.devicePostWithResponse(path, body)
	return err
}

// devicePostWithResponse makes a POST request with device authentication and returns parsed response
func (s *RuntimeSeeder) devicePostWithResponse(path string, body any) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", s.client.baseURL+path, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add device authentication headers
	req.Header.Set("Authorization", "Bearer "+s.deviceAPIKey)
	req.Header.Set("X-Staff-PIN", s.staffPIN)
	req.Header.Set("Content-Type", "application/json")

	if s.verbose {
		fmt.Printf("  → POST %s (device auth)\n", path)
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

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
