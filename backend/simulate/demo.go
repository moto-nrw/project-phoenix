package simulate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"
)

const demoWebGracePeriod = 15 * time.Minute

// DemoVisit is the latest visit of a child, read from the owning school's
// presence capability. ChangedAt includes checkout, not just arrival.
type DemoVisit struct {
	StudentID int64
	Active    bool
	Web       bool
	ChangedAt time.Time
}

type DemoTickOptions struct {
	State  *SeedState
	Client Client
	Now    func() time.Time
	Visits func(context.Context) ([]DemoVisit, error)
	// Parents are the other parents of the school (#3468); ParentClient
	// builds the client they act through. Without either, no parent acts.
	Parents      []DemoParent
	ParentClient func() DemoParentClient
	// PlanWeekdays runs a weekday along today's timetable, moved to the
	// current hour when it lies outside the school's day (#3921). Without
	// it, every day runs the open simulation with kiosk sessions.
	PlanWeekdays bool
}

// DemoTicker reuses live actions while reconciling against server state before
// each tick. The caller owns scheduling, authentication and tenant isolation.
type DemoTicker struct {
	options  DemoTickOptions
	live     *liveState
	counts   liveCounts
	prepared map[int64]bool
	parents  demoParentState
	day      demoDay
}

func NewDemoTicker(options DemoTickOptions) (*DemoTicker, error) {
	if options.State == nil || options.Client == nil || options.Now == nil || options.Visits == nil {
		return nil, fmt.Errorf("demo tick requires state, client, clock and visit query")
	}
	if len(options.State.Devices) == 0 || len(options.State.Rooms) == 0 || len(options.State.Activities) == 0 || len(options.State.Accounts.Betreuer) == 0 {
		return nil, fmt.Errorf("demo tick requires devices, rooms, activities and supervisors")
	}
	return &DemoTicker{options: options, prepared: make(map[int64]bool), live: &liveState{
		sick: make(map[int64]bool), rotation: make(map[int64]*rotationState),
		lastAttendance: make(map[int64]time.Time), staffIDs: collectStaffIDs(options.State),
		interval: 5 * time.Second,
	}}, nil
}

func (d *DemoTicker) Tick(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	visits, err := d.options.Visits(ctx)
	if err != nil {
		return fmt.Errorf("read demo visits: %w", err)
	}
	now := d.options.Now()
	latest := make(map[int64]DemoVisit, len(visits))
	for _, visit := range visits {
		latest[visit.StudentID] = visit
	}
	planDay := d.options.PlanWeekdays && demoWeekday(now)
	minute := now.In(demoBerlin).Hour()*60 + now.In(demoBerlin).Minute()
	if planDay {
		changed, err := d.syncDay(ctx, now, latest)
		if err != nil || changed {
			// The visits read above are stale once blocks or sessions
			// ended; the children move on the next tick.
			return err
		}
	}
	state := *d.options.State
	state.Students = nil
	d.live.checkedIn = make(map[int64]bool)
	d.live.unterwegs = make(map[int64]bool)
	d.live.rfidTags = make(map[int64]string)
	d.live.clock = func() time.Time { return now }
	active := 0
	for _, student := range d.options.State.Students {
		visit, exists := latest[student.ID]
		if visit.Active {
			active++
		}
		if visit.Web && now.Before(visit.ChangedAt.Add(demoWebGracePeriod)) {
			continue
		}
		if planDay && !d.day.present(student.ID, minute) {
			continue // At home: not arrived yet, or picked up.
		}
		state.Students = append(state.Students, student)
		d.live.rfidTags[student.ID] = fmt.Sprintf("DE%06X", student.ID)
		if visit.Active {
			d.live.checkedIn[student.ID] = true
		} else if exists || planDay {
			// On a planned day a child due at the OGS but in no room is on
			// its way there; returning brings it to its block.
			d.live.unterwegs[student.ID] = true
		}
	}
	// Every student action, including sick/attendance toggles and rebuilding,
	// sees only this eligible slice. Web actions therefore win over simulation.
	var rooms []int64
	if planDay {
		rooms, d.live.plannedRoom = d.day.rooms()
		d.live.withoutDeviceSession = true
	} else {
		var err error
		if rooms, err = d.ensureSessions(ctx); err != nil {
			return err
		}
		d.live.plannedRoom, d.live.withoutDeviceSession = nil, false
	}
	// The parents act apart from the children: a failing parent is logged
	// and never stops the children or the opening of a school.
	d.parentTick(now)
	return d.childrenTick(ctx, &state, rooms, active)
}

// childrenTick moves the eligible children, or rebuilds the day when none is present.
func (d *DemoTicker) childrenTick(ctx context.Context, state *SeedState, rooms []int64, active int) error {
	if len(state.Students) == 0 || len(rooms) == 0 {
		return nil // Nobody due, or no block running: the children are elsewhere.
	}
	if err := d.prepareStudents(ctx, state); err != nil {
		return err
	}
	d.live.roomIDs = rooms
	d.live.sessionID = 0
	if active == 0 {
		return d.rebuild(ctx, state, rooms)
	}
	device := state.Devices[sortedDeviceKeys(state.Devices)[0]]
	if d.live.withoutDeviceSession {
		d.arrive(state, device, rooms)
	}
	if err := runLiveTick(d.options.Client, d.live, state, device, &d.counts); err != nil {
		if isRoomCapacityExceeded(err) {
			return nil // A visitor or another simulated child already fills the room.
		}
		return fmt.Errorf("demo live action failed: %w", err)
	}
	return nil
}

// arrive checks a few children due at the OGS into the room of their block,
// so a block fills within a minute or two of its start instead of one child
// per random return.
func (d *DemoTicker) arrive(state *SeedState, device SeedDevice, rooms []int64) {
	waiting := slices.Sorted(maps.Keys(d.live.unterwegs))
	for _, studentID := range waiting[:min(len(waiting), demoDayArrivalsPerTick)] {
		room := d.live.plannedRoom[studentID]
		if room == 0 {
			room = rooms[int(studentID)%len(rooms)]
		}
		_, err := d.options.Client.DevicePost("/api/iot/checkin", map[string]any{
			"student_rfid": d.live.rfidTags[studentID], "action": "checkin", "room_id": room,
		}, device.APIKey, state.DevicePIN)
		if err != nil {
			if !isRoomCapacityExceeded(err) {
				slog.Info("demo day: child not checked in",
					"student_id", studentID,
					"error", err,
				)
			}
			continue
		}
		delete(d.live.unterwegs, studentID)
		d.live.checkedIn[studentID] = true
	}
}

// ensureSessions retains running sessions and recreates those ended by the
// daily close. It deliberately does not depend on a weekday timetable.
func (d *DemoTicker) ensureSessions(ctx context.Context) ([]int64, error) {
	state, client := d.options.State, d.options.Client
	keys, activities := sortedDeviceKeys(state.Devices), sortedStringKeys(state.Activities)
	count := min(len(keys), len(activities), len(state.Accounts.Betreuer), 10)
	rooms := make([]int64, 0, count)
	for i := 0; i < count; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		device := state.Devices[keys[i]]
		raw, err := client.DeviceGet("/api/iot/session/current", device.APIKey, state.DevicePIN)
		if err != nil {
			return nil, fmt.Errorf("read demo session: %w", err)
		}
		var response struct {
			Data struct {
				Active bool  `json:"is_active"`
				RoomID int64 `json:"room_id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, fmt.Errorf("decode demo session: %w", err)
		}
		room := response.Data.RoomID
		if !response.Data.Active {
			room = findRoomForActivity(activities[i], state.Rooms)
			if room == 0 {
				return nil, fmt.Errorf("demo activity %q has no room", activities[i])
			}
			_, err = client.DevicePost("/api/iot/session/start", map[string]any{
				"activity_id": state.Activities[activities[i]], "room_id": room,
				"supervisor_ids": []int64{state.Accounts.Betreuer[i].StaffID},
			}, device.APIKey, state.DevicePIN)
			if err != nil {
				return nil, fmt.Errorf("start demo session: %w", err)
			}
		}
		if room == 0 {
			return nil, fmt.Errorf("demo session has no room")
		}
		if _, err := client.DevicePost("/api/iot/ping", nil, device.APIKey, state.DevicePIN); err != nil {
			return nil, fmt.Errorf("keep demo session alive: %w", err)
		}
		rooms = append(rooms, room)
	}
	return rooms, nil
}

func (d *DemoTicker) rebuild(ctx context.Context, state *SeedState, rooms []int64) error {
	device := state.Devices[sortedDeviceKeys(state.Devices)[0]]
	client := d.options.Client
	for i, student := range state.Students[:min(len(state.Students), fullDayCheckinLimit)] {
		if err := ctx.Err(); err != nil {
			return err
		}
		tag := d.live.rfidTags[student.ID]
		// The kiosk confirms attendance only inside a session of its own; on
		// a planned day the room check-in records the arrival.
		if !d.live.withoutDeviceSession {
			if _, err := client.DevicePost("/api/iot/attendance/toggle", map[string]string{"rfid": tag, "action": "confirm"}, device.APIKey, state.DevicePIN); err != nil {
				return fmt.Errorf("restore demo attendance: %w", err)
			}
		}
		room := rooms[i%len(rooms)]
		if planned := d.live.plannedRoom[student.ID]; planned != 0 {
			room = planned
		}
		if _, err := client.DevicePost("/api/iot/checkin", map[string]any{"student_rfid": tag, "action": "checkin", "room_id": room}, device.APIKey, state.DevicePIN); err != nil {
			if isRoomCapacityExceeded(err) {
				continue // The room is full; the child waits for the next tick.
			}
			return fmt.Errorf("restore demo visit: %w", err)
		}
	}
	return nil
}

func isRoomCapacityExceeded(err error) bool {
	var coded interface{ HTTPErrorCode() string }
	return errors.As(err, &coded) && coded.HTTPErrorCode() == codeRoomCapacityExceeded
}

func (d *DemoTicker) prepareStudents(ctx context.Context, state *SeedState) error {
	for _, student := range state.Students {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := d.prepareStudent(state, student.ID); err != nil {
			return err
		}
	}
	return nil
}

// prepareStudent gives the child its demo RFID tag once per process.
func (d *DemoTicker) prepareStudent(state *SeedState, studentID int64) error {
	if d.prepared[studentID] {
		return nil
	}
	device := state.Devices[sortedDeviceKeys(state.Devices)[0]]
	tag := fmt.Sprintf("DE%06X", studentID)
	if _, err := d.options.Client.DevicePost(fmt.Sprintf("/api/students/%d/rfid", studentID), map[string]string{"rfid_tag": tag}, device.APIKey, state.DevicePIN); err != nil {
		return fmt.Errorf("assign demo RFID: %w", err)
	}
	d.prepared[studentID] = true
	return nil
}

// syncDay runs a weekday along its timetable (#3921): it ends the kiosk
// sessions an open-simulation day left running, brings the plan up to date,
// and sends the children home at their pickup. It reports whether it changed
// the children's rooms.
func (d *DemoTicker) syncDay(ctx context.Context, now time.Time, latest map[int64]DemoVisit) (bool, error) {
	state := d.options.State
	if d.day.date != now.In(demoBerlin).Format(isoDate) {
		d.day.devices = false
	}
	ended := false
	if !d.day.devices {
		var err error
		if ended, err = d.endDeviceSessions(ctx); err != nil {
			return false, err
		}
	}
	studentIDs := make([]int64, 0, len(state.Students))
	for _, student := range state.Students {
		studentIDs = append(studentIDs, student.ID)
	}
	changed, err := d.day.sync(d.options.Client, now, studentIDs, demoActivities(state))
	if err != nil {
		return false, err
	}
	d.day.devices = true
	if ended || changed {
		return true, nil
	}
	return false, d.sendHome(ctx, now, latest)
}

// endDeviceSessions ends the running kiosk sessions. On a planned day the
// children check into the rooms of the running blocks instead, so the
// sessions would only show as open spontaneous blocks. It reports whether it
// ended one.
func (d *DemoTicker) endDeviceSessions(ctx context.Context) (bool, error) {
	state := d.options.State
	ended := false
	for _, key := range sortedDeviceKeys(state.Devices) {
		if err := ctx.Err(); err != nil {
			return ended, err
		}
		device := state.Devices[key]
		raw, err := d.options.Client.DeviceGet("/api/iot/session/current", device.APIKey, state.DevicePIN)
		if err != nil {
			slog.Info("demo day: device session not read",
				"device", key,
				"error", err,
			)
			continue
		}
		var response struct {
			Data struct {
				Active bool `json:"is_active"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &response) != nil || !response.Data.Active {
			continue
		}
		if _, err := d.options.Client.DevicePost("/api/iot/session/end", nil, device.APIKey, state.DevicePIN); err != nil {
			slog.Info("demo day: device session not ended",
				"device", key,
				"error", err,
			)
			continue
		}
		ended = true
	}
	return ended, nil
}

// sendHome lets children whose pickup is due leave: out of the room and
// out of the OGS. A child that did not come today only counts as gone.
func (d *DemoTicker) sendHome(ctx context.Context, now time.Time, latest map[int64]DemoVisit) error {
	state := d.options.State
	local := now.In(demoBerlin)
	minute, today := local.Hour()*60+local.Minute(), local.Format(isoDate)
	sent := 0
	for _, student := range state.Students {
		if err := ctx.Err(); err != nil {
			return err
		}
		if sent >= demoDayHomeGoingPerTick {
			return nil
		}
		if !d.day.pickedUp(student.ID, minute) {
			continue
		}
		visit, seen := latest[student.ID]
		if visit.Web && now.Before(visit.ChangedAt.Add(demoWebGracePeriod)) {
			continue // A web action wins over the simulation.
		}
		d.day.home[student.ID] = true
		if !seen || visit.ChangedAt.In(demoBerlin).Format(isoDate) != today {
			continue
		}
		sent++
		// The staff checkout ends the room visit and the day's attendance;
		// the kiosk could only do that inside a session of its own.
		if _, err := d.options.Client.Post(fmt.Sprintf("/api/active/visits/student/%d/checkout", student.ID), nil); err != nil {
			// Also a child that already left on its own; nothing to undo.
			slog.Debug("demo day: child not sent home",
				"student_id", student.ID,
				"error", err,
			)
		}
	}
	return nil
}
