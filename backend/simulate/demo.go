package simulate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
}

// DemoTicker reuses live actions while reconciling against server state before
// each tick. The caller owns scheduling, authentication and tenant isolation.
type DemoTicker struct {
	options  DemoTickOptions
	live     *liveState
	counts   liveCounts
	prepared map[int64]bool
	parents  demoParentState
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
		state.Students = append(state.Students, student)
		d.live.rfidTags[student.ID] = fmt.Sprintf("DE%06X", student.ID)
		if visit.Active {
			d.live.checkedIn[student.ID] = true
		} else if exists {
			d.live.unterwegs[student.ID] = true
		}
	}
	// Every student action, including sick/attendance toggles and rebuilding,
	// sees only this eligible slice. Web actions therefore win over simulation.
	rooms, err := d.ensureSessions(ctx)
	if err != nil {
		return err
	}
	// The parents act apart from the children: a failing parent is logged
	// and never stops the children or the opening of a school.
	d.parentTick(now)
	return d.childrenTick(ctx, &state, rooms, active)
}

// childrenTick moves the eligible children, or rebuilds the day when none is present.
func (d *DemoTicker) childrenTick(ctx context.Context, state *SeedState, rooms []int64, active int) error {
	if len(state.Students) == 0 {
		return nil
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
	if err := runLiveTick(d.options.Client, d.live, state, device, &d.counts); err != nil {
		var coded interface{ HTTPErrorCode() string }
		if errors.As(err, &coded) && coded.HTTPErrorCode() == "ROOM_CAPACITY_EXCEEDED" {
			return nil // A visitor or another simulated child already fills the room.
		}
		return fmt.Errorf("demo live action failed: %w", err)
	}
	return nil
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
		if _, err := client.DevicePost("/api/iot/attendance/toggle", map[string]string{"rfid": tag, "action": "confirm"}, device.APIKey, state.DevicePIN); err != nil {
			return fmt.Errorf("restore demo attendance: %w", err)
		}
		if _, err := client.DevicePost("/api/iot/checkin", map[string]any{"student_rfid": tag, "action": "checkin", "room_id": rooms[i%len(rooms)]}, device.APIKey, state.DevicePIN); err != nil {
			return fmt.Errorf("restore demo visit: %w", err)
		}
	}
	return nil
}

func (d *DemoTicker) prepareStudents(ctx context.Context, state *SeedState) error {
	device := state.Devices[sortedDeviceKeys(state.Devices)[0]]
	for _, student := range state.Students {
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.prepared[student.ID] {
			continue
		}
		_, err := d.options.Client.DevicePost(fmt.Sprintf("/api/students/%d/rfid", student.ID), map[string]string{"rfid_tag": d.live.rfidTags[student.ID]}, device.APIKey, state.DevicePIN)
		if err != nil {
			return fmt.Errorf("assign demo RFID: %w", err)
		}
		d.prepared[student.ID] = true
	}
	return nil
}
