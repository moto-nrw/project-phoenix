package iot

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/data"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
)

var (
	// ErrPartialAuthentication indicates that at least one device failed authentication.
	ErrPartialAuthentication = errors.New("one or more devices failed to authenticate")
	rng                      = rand.New(rand.NewSource(time.Now().UnixNano()))
)

const sessionStartRetryInterval = 30 * time.Second

// Run executes the simulator discovery phase: authenticate devices and keep their state in sync.
func Run(ctx context.Context, cfg *Config) error {
	globalPIN := getGlobalPIN()
	if globalPIN == "" {
		return fmt.Errorf("OGS_DEVICE_PIN environment variable is required")
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	client := NewClient(cfg.BaseURL, globalPIN, httpClient)
	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"device_count": len(cfg.Devices),
			"base_url":     strings.TrimSuffix(cfg.BaseURL, "/"),
		}).Info("Starting state sync")
	}

	states := make(map[string]*DeviceState, len(cfg.Devices))
	stateMu := &sync.RWMutex{}

	// Phase 1: Authenticate all devices
	failed := authenticateDevices(ctx, client, cfg, states, stateMu)
	if len(failed) > 0 {
		return fmt.Errorf("%w: %s", ErrPartialAuthentication, strings.Join(failed, ", "))
	}

	// Phase 2: Start event engine if configured
	eventTicker := startEventEngine(ctx, cfg, client, stateMu, states)
	if eventTicker != nil {
		defer eventTicker.Stop()
	}

	// Phase 3: Run refresh loop
	return runRefreshLoop(ctx, cfg, client, states, stateMu)
}

// authenticateDevices authenticates all configured devices and performs initial state sync.
func authenticateDevices(ctx context.Context, client *Client, cfg *Config, states map[string]*DeviceState, stateMu *sync.RWMutex) []string {
	var failed []string

	for _, device := range cfg.Devices {
		if ctx.Err() != nil {
			break
		}

		if err := client.Authenticate(ctx, device); err != nil {
			if logger.Logger != nil {
				logger.Logger.WithFields(map[string]interface{}{
					"device_id": device.DeviceID,
					"error":     err.Error(),
				}).Error("Device authentication failed")
			}
			failed = append(failed, device.DeviceID)
			continue
		}

		if logger.Logger != nil {
			logger.Logger.WithField("device_id", device.DeviceID).Info("Device authentication OK")
		}
		syncDeviceState(ctx, client, cfg, device, states, stateMu)
	}

	return failed
}

// syncDeviceState refreshes a single device's state and updates the shared state map.
func syncDeviceState(ctx context.Context, client *Client, cfg *Config, device DeviceConfig, states map[string]*DeviceState, stateMu *sync.RWMutex) {
	stateMu.RLock()
	prevState := states[device.DeviceID]
	stateMu.RUnlock()

	state, err := refreshDeviceState(ctx, client, cfg, device, prevState)
	if err != nil {
		if logger.Logger != nil {
			logger.Logger.WithFields(map[string]interface{}{
				"device_id": device.DeviceID,
				"error":     err.Error(),
			}).Error("Device sync failed")
		}
		return
	}

	preserveSessionState(state, prevState)

	stateMu.Lock()
	states[device.DeviceID] = state
	stateMu.Unlock()

	maybeStartDefaultSession(ctx, client, device, stateMu, state)

	stateMu.RLock()
	logDeviceState(device.DeviceID, state)
	stateMu.RUnlock()
}

// preserveSessionState carries over session management state from the previous state.
func preserveSessionState(state, prev *DeviceState) {
	if prev == nil {
		return
	}
	state.SessionManaged = prev.SessionManaged
	state.ManagedSessionID = prev.ManagedSessionID
	state.LastSessionStartAttempt = prev.LastSessionStartAttempt

	if !state.sessionActive() && prev.SessionManaged {
		state.SessionManaged = false
		state.ManagedSessionID = nil
	}
}

// startEventEngine initializes the event engine if configured.
func startEventEngine(ctx context.Context, cfg *Config, client *Client, stateMu *sync.RWMutex, states map[string]*DeviceState) *time.Ticker {
	if cfg.Event.Interval <= 0 || cfg.Event.MaxEventsPerTick <= 0 {
		return nil
	}

	engine := NewEngine(cfg, client, stateMu, states)
	ticker := time.NewTicker(cfg.Event.Interval)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				engine.Tick(ctx)
			}
		}
	}()

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"interval":   cfg.Event.Interval.String(),
			"max_events": cfg.Event.MaxEventsPerTick,
		}).Info("Event loop running")
	}
	return ticker
}

// runRefreshLoop periodically refreshes device states.
func runRefreshLoop(ctx context.Context, cfg *Config, client *Client, states map[string]*DeviceState, stateMu *sync.RWMutex) error {
	if cfg.RefreshInterval <= 0 {
		if logger.Logger != nil {
			logger.Logger.Info("Initial authentication complete; no refresh interval configured, exiting")
		}
		return nil
	}

	if logger.Logger != nil {
		logger.Logger.WithField("interval", cfg.RefreshInterval.String()).Info("State sync running. Press Ctrl+C to stop")
	}

	ticker := time.NewTicker(cfg.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if logger.Logger != nil {
				logger.Logger.Info("Context cancelled, shutting down state sync")
			}
			return nil
		case <-ticker.C:
			refreshAllDevices(ctx, client, cfg, states, stateMu)
		}
	}
}

// refreshAllDevices refreshes state for all configured devices.
func refreshAllDevices(ctx context.Context, client *Client, cfg *Config, states map[string]*DeviceState, stateMu *sync.RWMutex) {
	for _, device := range cfg.Devices {
		if ctx.Err() != nil {
			if logger.Logger != nil {
				logger.Logger.Info("Context cancelled, shutting down state sync")
			}
			return
		}
		syncDeviceState(ctx, client, cfg, device, states, stateMu)
	}
}

func refreshDeviceState(ctx context.Context, client *Client, cfg *Config, device DeviceConfig, prev *DeviceState) (*DeviceState, error) {
	// Fetch all required data from API
	state, err := fetchDeviceData(ctx, client, device)
	if err != nil {
		return nil, err
	}

	// Preserve supervisor assignments from previous state
	if prev != nil {
		for id, assignment := range prev.ActiveSupervisors {
			state.ActiveSupervisors[id] = assignment
		}
	}

	// Build lookup indexes
	buildStateIndexes(state)

	// Build student states with previous state preservation
	prevStudents := extractPrevStudents(prev)
	buildStudentStates(state, cfg, prevStudents)

	// Build staff roster
	teachers := fetchTeachersIfNeeded(ctx, client, device)
	prevStaff := extractPrevStaff(prev)
	leadHints := buildLeadHints(device)
	buildStaffRoster(state, teachers, prevStaff, leadHints)

	// Ensure at least one lead exists
	ensureStaffLead(state)

	// Initialize default supervisors if needed
	initializeDefaultSupervisors(state, device)

	// Prune supervisors for staff no longer available
	pruneSupervisors(state)

	return state, nil
}

// fetchDeviceData retrieves session, rooms, activities, and students from the API.
func fetchDeviceData(ctx context.Context, client *Client, device DeviceConfig) (*DeviceState, error) {
	session, err := client.FetchSession(ctx, device)
	if err != nil {
		return nil, fmt.Errorf("fetch session: %w", err)
	}

	rooms, err := client.FetchRooms(ctx, device)
	if err != nil {
		return nil, fmt.Errorf("fetch rooms: %w", err)
	}

	activities, err := client.FetchActivities(ctx, device)
	if err != nil {
		return nil, fmt.Errorf("fetch activities: %w", err)
	}

	students, err := client.FetchStudents(ctx, device)
	if err != nil {
		return nil, fmt.Errorf("fetch students: %w", err)
	}

	state := &DeviceState{
		Session:       session,
		Rooms:         rooms,
		Activities:    activities,
		Students:      students,
		LastRefreshed: time.Now(),
	}
	state.ensureIndexes()

	return state, nil
}

// buildStateIndexes populates the room and activity lookup maps.
func buildStateIndexes(state *DeviceState) {
	for _, room := range state.Rooms {
		state.RoomsByID[room.ID] = room
	}
	for _, activity := range state.Activities {
		state.ActivitiesByID[activity.ID] = activity
	}
}

// extractPrevStudents returns the student states from the previous state if available.
func extractPrevStudents(prev *DeviceState) map[int64]*StudentState {
	if prev == nil || len(prev.StudentStates) == 0 {
		return nil
	}
	return prev.StudentStates
}

// extractPrevStaff returns the staff roster from the previous state if available.
func extractPrevStaff(prev *DeviceState) map[int64]*StaffState {
	if prev == nil || len(prev.StaffRoster) == 0 {
		return nil
	}
	return prev.StaffRoster
}

// buildStudentStates creates student state entries, preserving previous state where applicable.
func buildStudentStates(state *DeviceState, cfg *Config, prevStudents map[int64]*StudentState) {
	for _, student := range state.Students {
		st := createStudentState(student, cfg)
		preserveStudentState(st, prevStudents[student.StudentID])
		ensureAGHopTarget(st, cfg)
		state.StudentStates[student.StudentID] = st
	}
}

// createStudentState creates a new student state from API data.
func createStudentState(student data.TeacherStudentResponse, cfg *Config) *StudentState {
	return &StudentState{
		StudentID:    student.StudentID,
		PersonID:     student.PersonID,
		FirstName:    student.FirstName,
		LastName:     student.LastName,
		RFIDTag:      student.RFIDTag,
		CurrentPhase: RotationPhaseHeimatraum,
		NextPhase:    RotationPhaseAG,
		VisitedAGs:   make(map[int64]time.Time),
		AGHopTarget:  cfg.Event.Rotation.MinAGHops,
	}
}

// preserveStudentState copies relevant fields from a previous student state.
func preserveStudentState(st, existing *StudentState) {
	if existing == nil {
		return
	}
	st.CurrentPhase = existing.CurrentPhase
	if existing.NextPhase != "" {
		st.NextPhase = existing.NextPhase
	}
	st.CurrentRoomID = existing.CurrentRoomID
	st.RotationIndex = existing.RotationIndex
	st.AGHopCount = existing.AGHopCount
	if existing.AGHopTarget > 0 {
		st.AGHopTarget = existing.AGHopTarget
	}
	st.VisitedAGs = cloneVisitedAGs(existing.VisitedAGs)
	st.AttendanceStatus = existing.AttendanceStatus
	st.LastAttendance = existing.LastAttendance
	st.LastEventAt = existing.LastEventAt
	st.HomeRoomID = existing.HomeRoomID
	st.HomeDeviceID = existing.HomeDeviceID
	st.HasActiveVisit = existing.HasActiveVisit
}

// ensureAGHopTarget ensures the student has a valid AG hop target.
func ensureAGHopTarget(st *StudentState, cfg *Config) {
	if st.AGHopTarget <= 0 {
		st.AGHopTarget = generateAGHopTarget(cfg.Event)
	}
}

// fetchTeachersIfNeeded fetches teachers if the device has teacher IDs or a default session.
func fetchTeachersIfNeeded(ctx context.Context, client *Client, device DeviceConfig) []data.DeviceTeacherResponse {
	if len(device.TeacherIDs) == 0 && device.DefaultSession == nil {
		return nil
	}
	teachers, err := client.FetchTeachers(ctx, device)
	if err != nil {
		if logger.Logger != nil {
			logger.Logger.WithFields(map[string]interface{}{
				"device_id": device.DeviceID,
				"error":     err.Error(),
			}).Warn("Device teacher refresh failed")
		}
		return nil
	}
	return teachers
}

// buildLeadHints creates a set of staff IDs that should be marked as leads.
func buildLeadHints(device DeviceConfig) map[int64]struct{} {
	hints := make(map[int64]struct{})
	if device.DefaultSession == nil || len(device.DefaultSession.SupervisorIDs) == 0 {
		return hints
	}
	firstLead := device.DefaultSession.SupervisorIDs[0]
	if firstLead > 0 {
		hints[firstLead] = struct{}{}
	}
	return hints
}

// buildStaffRoster creates staff state entries from teacher data.
func buildStaffRoster(state *DeviceState, teachers []data.DeviceTeacherResponse, prevStaff map[int64]*StaffState, leadHints map[int64]struct{}) {
	for _, teacher := range teachers {
		staff := &StaffState{
			StaffID:     teacher.StaffID,
			PersonID:    teacher.PersonID,
			FirstName:   teacher.FirstName,
			LastName:    teacher.LastName,
			DisplayName: teacher.DisplayName,
		}

		if _, ok := leadHints[teacher.StaffID]; ok {
			staff.IsLead = true
		}

		if existing := prevStaff[teacher.StaffID]; existing != nil {
			staff.IsLead = staff.IsLead || existing.IsLead
			staff.LastActive = existing.LastActive
		}

		state.StaffRoster[teacher.StaffID] = staff
	}
}

// ensureStaffLead ensures at least one staff member is marked as lead.
func ensureStaffLead(state *DeviceState) {
	if len(state.StaffRoster) == 0 {
		return
	}

	for _, staff := range state.StaffRoster {
		if staff.IsLead {
			return // Already have a lead
		}
	}

	// Select the staff with the lowest ID as lead
	var selectedID int64
	first := true
	for id := range state.StaffRoster {
		if first || id < selectedID {
			selectedID = id
			first = false
		}
	}

	if !first {
		if selected := state.StaffRoster[selectedID]; selected != nil {
			selected.IsLead = true
		}
	}
}

// initializeDefaultSupervisors sets up initial supervisor assignments from device config.
func initializeDefaultSupervisors(state *DeviceState, device DeviceConfig) {
	if len(state.ActiveSupervisors) > 0 || device.DefaultSession == nil {
		return
	}

	now := time.Now()
	for idx, supID := range device.DefaultSession.SupervisorIDs {
		if supID <= 0 {
			continue
		}
		staff := state.StaffRoster[supID]
		isLead := (staff != nil && staff.IsLead) || idx == 0

		state.ActiveSupervisors[supID] = SupervisorAssignment{
			StaffID:     supID,
			IsLead:      isLead,
			LastUpdated: now,
		}
	}
}

// pruneSupervisors removes supervisor assignments for staff no longer in the roster.
func pruneSupervisors(state *DeviceState) {
	for staffID := range state.ActiveSupervisors {
		if _, ok := state.StaffRoster[staffID]; !ok {
			delete(state.ActiveSupervisors, staffID)
		}
	}
}

func generateAGHopTarget(eventCfg EventConfig) int {
	min := eventCfg.Rotation.MinAGHops
	max := eventCfg.Rotation.MaxAGHops
	if min <= 0 && max <= 0 {
		return defaultMinAGHops
	}
	if min <= 0 {
		min = defaultMinAGHops
	}
	if max < min {
		max = min
	}
	if max == min {
		return min
	}

	span := max - min + 1
	if span <= 1 {
		return min
	}
	return min + rng.Intn(span)
}

func logDeviceState(deviceID string, state *DeviceState) {
	if logger.Logger == nil {
		return
	}

	if state == nil {
		logger.Logger.WithField("device_id", deviceID).Warn("Device state unavailable")
		return
	}

	roomName := "<none>"
	sessionStatus := "inactive"

	if state.Session != nil {
		if state.Session.RoomName != nil && *state.Session.RoomName != "" {
			roomName = *state.Session.RoomName
		}
		if state.Session.IsActive {
			sessionStatus = "active"
		}
	}

	logger.Logger.WithFields(map[string]interface{}{
		"device_id":      deviceID,
		"session_status": sessionStatus,
		"room_name":      roomName,
		"student_count":  len(state.Students),
		"room_count":     len(state.Rooms),
		"activity_count": len(state.Activities),
		"refreshed":      state.LastRefreshed.Format(time.RFC3339),
	}).Debug("Device state synced")
}

func getGlobalPIN() string {
	return strings.TrimSpace(os.Getenv("OGS_DEVICE_PIN"))
}

func maybeStartDefaultSession(ctx context.Context, client *Client, device DeviceConfig, stateMu *sync.RWMutex, state *DeviceState) {
	if device.DefaultSession == nil || state == nil {
		return
	}

	stateMu.Lock()

	if state.sessionActive() {
		stateMu.Unlock()
		return
	}

	if time.Since(state.LastSessionStartAttempt) < sessionStartRetryInterval {
		stateMu.Unlock()
		return
	}

	state.LastSessionStartAttempt = time.Now()
	stateMu.Unlock()

	resp, err := client.StartSession(ctx, device, device.DefaultSession)
	if err != nil {
		if logger.Logger != nil {
			logger.Logger.WithFields(map[string]interface{}{
				"device_id": device.DeviceID,
				"error":     err.Error(),
			}).Error("Device session start failed")
		}
		return
	}

	session, err := client.FetchSession(ctx, device)
	if err != nil {
		if logger.Logger != nil {
			logger.Logger.WithFields(map[string]interface{}{
				"device_id": device.DeviceID,
				"error":     err.Error(),
			}).Warn("Device failed to refresh session after start")
		}
	}

	stateMu.Lock()
	defer stateMu.Unlock()

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"device_id":      device.DeviceID,
			"room_id":        device.DefaultSession.RoomID,
			"activity_id":    device.DefaultSession.ActivityID,
			"supervisor_ids": device.DefaultSession.SupervisorIDs,
		}).Info("Device session started")
	}

	id := resp.ActiveGroupID
	state.SessionManaged = true
	state.ManagedSessionID = &id
	state.LastSessionStartAttempt = time.Now()
	if session != nil {
		state.Session = session
	}
}
