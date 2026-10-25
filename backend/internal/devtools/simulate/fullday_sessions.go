package simulate

import (
	"context"
	"fmt"
	"slices"
)

// maxMiddaySessions caps the kiosk sessions that open at midday.
const maxMiddaySessions = 10

// kioskSessionMinutes is how long the timetable shows a kiosk session: the
// kiosk mirror plans it for one hour.
const kioskSessionMinutes = 60

// earlyArrivalActivity is what a caregiver opens for a class that arrives
// before the midday rooms open, e.g. when its lessons are cancelled.
const earlyArrivalActivity = "Hausaufgaben"

type startMiddayAction struct{}

func (startMiddayAction) Name() string { return "start sessions" }

// Run opens the afternoon at sessionsStart: the planned blocks of that time
// start with their planned staff, and caregivers who are on shift and have
// no block then open a kiosk session each, all in the same minute. A room
// opened earlier for an early class closes; its children move on.
func (startMiddayAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 3c: Starting the afternoon...")
	early := rt.Rooms.inRooms(rt.Rooms.sessionRooms)
	if err := endActiveSessions(rt); err != nil {
		return err
	}
	rt.SupervisorStaffIDs = nil
	rt.Rooms.closeSessions()
	// A failure leaves the started sessions to RunFullDay, which ends them.
	if _, err := rt.startPlannedBlocks(sessionsStart); err != nil {
		return err
	}
	if err := rt.startKioskSessions(sessionsStart, maxMiddaySessions, ""); err != nil {
		return err
	}
	if !rt.Rooms.open() {
		return fmt.Errorf("no room opens at %s: no planned block, %d caregivers, %d devices, %d activities",
			sessionsStart, len(rt.State.Accounts.Betreuer), len(rt.DeviceKeys), len(rt.State.Activities))
	}
	device, err := rt.primaryDevice()
	if err != nil {
		return err
	}
	for _, id := range early {
		child, _ := rt.child(id)
		rt.Rooms.leave(id)
		rt.Counts.StudentsCheckedIn-- // place counts the child again
		if err := rt.place(child.SeedStudent, device); err != nil {
			return err
		}
	}
	fmt.Printf("  %d planned blocks, %d sessions started\n", rt.Counts.BlocksStarted, rt.Counts.SessionsStarted)
	return nil
}

// ensureRoom opens a room for a child arriving before the midday rooms: a
// free caregiver starts earlyArrivalActivity at the kiosk.
func (rt *Runtime) ensureRoom(_ context.Context, c wallClock) error {
	if rt.Rooms.open() {
		return nil
	}
	if err := rt.startKioskSessions(c, 1, earlyArrivalActivity); err != nil {
		return err
	}
	if !rt.Rooms.open() {
		fmt.Printf("  No caregiver free at %s; early children wait without a room\n", c)
	}
	return nil
}

// startKioskSessions starts up to limit kiosk sessions at the time of day c,
// the preferred activity first, each on its own device with a free
// caregiver. Activities whose room a planned block already uses are left
// out, and nobody planned on a block during the session supervises it.
func (rt *Runtime) startKioskSessions(c wallClock, limit int, preferred string) error {
	caregivers, err := rt.freeCaregivers(c, c+kioskSessionMinutes)
	if err != nil {
		return err
	}
	devices := slices.DeleteFunc(slices.Clone(rt.DeviceKeys), func(key string) bool {
		return slices.Contains(rt.ActiveSessionDeviceKeys, key)
	})
	started := 0
	for _, activity := range kioskActivities(rt.State.Activities, preferred) {
		if started == limit || started == len(caregivers) || started == len(devices) {
			break
		}
		roomID := findRoomForActivity(activity, rt.State.Rooms)
		if roomID != 0 && rt.Rooms.isOpen(roomID) {
			continue
		}
		if err := rt.startSession(devices[started], activity, caregivers[started]); err != nil {
			return err
		}
		started++
	}
	return nil
}

// kioskActivities lists the activities in name order, the preferred one
// first.
func kioskActivities(activities map[string]int64, preferred string) []string {
	names := sortedStringKeys(activities)
	if i := slices.Index(names, preferred); i > 0 {
		names = append([]string{preferred}, slices.Delete(names, i, i+1)...)
	}
	return names
}

// startSession starts the activity on the device with one supervisor, in the
// activity's room when the demo convention names one.
func (rt *Runtime) startSession(deviceKey, actName string, supervisor AccountCredentials) error {
	device := rt.State.Devices[deviceKey]
	roomID := findRoomForActivity(actName, rt.State.Rooms)
	body := map[string]any{
		"activity_id":    rt.State.Activities[actName],
		"supervisor_ids": []int64{supervisor.StaffID},
	}
	if roomID != 0 {
		body["room_id"] = roomID
	}
	if _, err := rt.Client.DevicePost("/api/iot/session/start", body, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("start session for %s: %w", actName, err)
	}
	rt.ActiveSessionDeviceKeys = append(rt.ActiveSessionDeviceKeys, deviceKey)
	rt.SupervisorStaffIDs = append(rt.SupervisorStaffIDs, supervisor.StaffID)
	if roomID != 0 {
		rt.Rooms.openSession(roomID)
	}
	rt.Counts.SessionsStarted++
	fmt.Printf("  Started: %s (device: %s, supervisor: %s)\n", actName, device.Name, supervisor.Name)
	return nil
}
