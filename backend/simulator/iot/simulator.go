package iot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
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

	log.Printf("[simulator] Starting state sync for %d device(s) against %s", len(cfg.Devices), strings.TrimSuffix(cfg.BaseURL, "/"))

	var failed []string
	states := make(map[string]*DeviceState, len(cfg.Devices))
	stateMu := &sync.RWMutex{}

	for _, device := range cfg.Devices {
		device := device // capture range variable

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := client.Authenticate(ctx, device); err != nil {
			log.Printf("[simulator] Device %s authentication FAILED: %v", device.DeviceID, err)
			failed = append(failed, device.DeviceID)
			continue
		}

		log.Printf("[simulator] Device %s authentication OK", device.DeviceID)

		stateMu.RLock()
		prevState := states[device.DeviceID]
		stateMu.RUnlock()

		if state, err := refreshDeviceState(ctx, client, cfg, device, prevState); err != nil {
			log.Printf("[simulator] Device %s initial sync failed: %v", device.DeviceID, err)
		} else {
			if prev := prevState; prev != nil {
				state.SessionManaged = prev.SessionManaged
				state.ManagedSessionID = prev.ManagedSessionID
				state.LastSessionStartAttempt = prev.LastSessionStartAttempt
				if !state.sessionActive() && prev.SessionManaged {
					state.SessionManaged = false
					state.ManagedSessionID = nil
				}
			}
			stateMu.Lock()
			states[device.DeviceID] = state
			stateMu.Unlock()
			maybeStartDefaultSession(ctx, client, device, stateMu, state)
			stateMu.RLock()
			logDeviceState(device.DeviceID, state)
			stateMu.RUnlock()
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("%w: %s", ErrPartialAuthentication, strings.Join(failed, ", "))
	}

	var (
		engine      *Engine
		eventTicker *time.Ticker
	)

	if cfg.Event.Interval > 0 && cfg.Event.MaxEventsPerTick > 0 {
		engine = NewEngine(cfg, client, stateMu, states)
		eventTicker = time.NewTicker(cfg.Event.Interval)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-eventTicker.C:
					engine.Tick(ctx)
				}
			}
		}()
		log.Printf("[engine] Event loop running (interval=%s, max_events=%d)", cfg.Event.Interval, cfg.Event.MaxEventsPerTick)
	}

	if eventTicker != nil {
		defer eventTicker.Stop()
	}

	if cfg.RefreshInterval <= 0 {
		log.Printf("[simulator] Initial authentication complete; no refresh interval configured, exiting.")
		return nil
	}

	log.Printf("[simulator] State sync running (interval %s). Press Ctrl+C to stop.", cfg.RefreshInterval)

	ticker := time.NewTicker(cfg.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[simulator] Context cancelled, shutting down state sync.")
			return nil
		case <-ticker.C:
			for _, device := range cfg.Devices {
				select {
				case <-ctx.Done():
					log.Printf("[simulator] Context cancelled, shutting down state sync.")
					return nil
				default:
				}

				stateMu.RLock()
				prevState := states[device.DeviceID]
				stateMu.RUnlock()

				state, err := refreshDeviceState(ctx, client, cfg, device, prevState)
				if err != nil {
					log.Printf("[simulator] Device %s refresh failed: %v", device.DeviceID, err)
					continue
				}

				if prev := prevState; prev != nil {
					state.SessionManaged = prev.SessionManaged
					state.ManagedSessionID = prev.ManagedSessionID
					state.LastSessionStartAttempt = prev.LastSessionStartAttempt
					if !state.sessionActive() && prev.SessionManaged {
						state.SessionManaged = false
						state.ManagedSessionID = nil
					}
				}

				stateMu.Lock()
				states[device.DeviceID] = state
				stateMu.Unlock()
				maybeStartDefaultSession(ctx, client, device, stateMu, state)
				stateMu.RLock()
				logDeviceState(device.DeviceID, state)
				stateMu.RUnlock()
			}
		}
	}
}

func refreshDeviceState(ctx context.Context, client *Client, cfg *Config, device DeviceConfig, prev *DeviceState) (*DeviceState, error) {
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

	var teachers []iotapi.DeviceTeacherResponse
	if len(device.TeacherIDs) > 0 || device.DefaultSession != nil {
		var fetchErr error
		teachers, fetchErr = client.FetchTeachers(ctx, device)
		if fetchErr != nil {
			log.Printf("[simulator] Device %s teacher refresh failed: %v", device.DeviceID, fetchErr)
			teachers = nil
		}
	}

	state := &DeviceState{
		Session:       session,
		Rooms:         rooms,
		Activities:    activities,
		Students:      students,
		LastRefreshed: time.Now(),
	}

	state.ensureIndexes()

	if prev != nil {
		for id, assignment := range prev.ActiveSupervisors {
			state.ActiveSupervisors[id] = assignment
		}
	}

	for _, room := range rooms {
		state.RoomsByID[room.ID] = room
	}

	for _, activity := range activities {
		state.ActivitiesByID[activity.ID] = activity
	}

	var prevStudents map[int64]*StudentState
	var prevStaff map[int64]*StaffState
	if prev != nil {
		if len(prev.StudentStates) > 0 {
			prevStudents = prev.StudentStates
		}
		if len(prev.StaffRoster) > 0 {
			prevStaff = prev.StaffRoster
		}
	}

	for _, student := range students {
		st := &StudentState{
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

		if existing := prevStudents[student.StudentID]; existing != nil {
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
		} else {
			st.AGHopTarget = generateAGHopTarget(cfg.Event)
		}

		if st.AGHopTarget <= 0 {
			st.AGHopTarget = generateAGHopTarget(cfg.Event)
		}

		state.StudentStates[student.StudentID] = st
	}

	// Build staff roster snapshot
	leadHints := map[int64]struct{}{}
	if device.DefaultSession != nil && len(device.DefaultSession.SupervisorIDs) > 0 {
		firstLead := device.DefaultSession.SupervisorIDs[0]
		if firstLead > 0 {
			leadHints[firstLead] = struct{}{}
		}
	}

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

		if prevStaff != nil {
			if existing := prevStaff[teacher.StaffID]; existing != nil {
				staff.IsLead = staff.IsLead || existing.IsLead
				staff.LastActive = existing.LastActive
			}
		}

		state.StaffRoster[teacher.StaffID] = staff
	}

	if len(state.StaffRoster) > 0 {
		hasLead := false
		for _, staff := range state.StaffRoster {
			if staff.IsLead {
				hasLead = true
				break
			}
		}
		if !hasLead {
			var selectedID int64
			first := true
			for id := range state.StaffRoster {
				if first || id < selectedID {
					selectedID = id
					first = false
				}
			}
			if !first {
				if selected, ok := state.StaffRoster[selectedID]; ok {
					selected.IsLead = true
				}
			}
		}
	}

	if len(state.ActiveSupervisors) == 0 && device.DefaultSession != nil {
		now := time.Now()
		for idx, supID := range device.DefaultSession.SupervisorIDs {
			if supID <= 0 {
				continue
			}
			staff := state.StaffRoster[supID]
			isLead := false
			if staff != nil && staff.IsLead {
				isLead = true
			} else if idx == 0 {
				isLead = true
			}
			state.ActiveSupervisors[supID] = SupervisorAssignment{
				StaffID:     supID,
				IsLead:      isLead,
				LastUpdated: now,
			}
		}
	}

	// Prune supervisor assignments for staff no longer available.
	for staffID := range state.ActiveSupervisors {
		if _, ok := state.StaffRoster[staffID]; !ok {
			delete(state.ActiveSupervisors, staffID)
		}
	}

	return state, nil
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
	if state == nil {
		log.Printf("[simulator] Device %s state unavailable", deviceID)
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

	log.Printf(
		"[simulator] Device %s -> session=%s room=%s students=%d rooms=%d activities=%d refreshed=%s",
		deviceID,
		sessionStatus,
		roomName,
		len(state.Students),
		len(state.Rooms),
		len(state.Activities),
		state.LastRefreshed.Format(time.RFC3339),
	)
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
		log.Printf("[simulator] Device %s session start failed: %v", device.DeviceID, err)
		return
	}

	session, err := client.FetchSession(ctx, device)
	if err != nil {
		log.Printf("[simulator] Device %s failed to refresh session after start: %v", device.DeviceID, err)
	}

	stateMu.Lock()
	defer stateMu.Unlock()

	log.Printf("[simulator] Device %s session started (room=%d activity=%d supervisors=%v)", device.DeviceID, device.DefaultSession.RoomID, device.DefaultSession.ActivityID, device.DefaultSession.SupervisorIDs)

	id := resp.ActiveGroupID
	state.SessionManaged = true
	state.ManagedSessionID = &id
	state.LastSessionStartAttempt = time.Now()
	if session != nil {
		state.Session = session
	}
}
