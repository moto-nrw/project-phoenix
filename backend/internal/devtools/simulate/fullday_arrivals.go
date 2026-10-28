package simulate

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand"
	"slices"
)

// The children of a full day who come today (todayChild.Absent false):
//
//   - most arrive after their lessons and stay in a room all afternoon;
//   - unterwegsCount of the last wave leave their room again without going
//     home (unterwegs);
//   - earlyPickupCount of the first wave go home at earlyPickupsAt.
//
// Children reported sick, excused or not coming today never arrive.
const (
	unterwegsCount   = 2
	earlyPickupCount = 6
	fullDaySickFrom  = 96
	fullDaySickTo    = 100 // four fall ill: with the seed's, under one child in ten
)

// Registered error codes of a full room or activity (error-registry/).
const codeActivityCapacityExceeded = "iot.activity_capacity_exceeded"

// roomPlan tracks the open rooms and where each child of the day is.
type roomPlan struct {
	// blockRooms are the rooms of the running planned blocks; blockRoom is
	// the room of the running block each child is on.
	blockRooms []int64
	blockRoom  map[int64]int64
	// sessionRooms are the rooms of the running kiosk sessions, in turn;
	// sessionDevice is the kiosk that runs each of them.
	sessionRooms  []int64
	sessionDevice map[int64]string
	next          int
	// childRoom is the room each child was checked into; order lists the
	// children in check-in order.
	childRoom map[int64]int64
	order     []int64
}

func newRoomPlan() roomPlan {
	return roomPlan{blockRoom: map[int64]int64{}, sessionDevice: map[int64]string{}, childRoom: map[int64]int64{}}
}

func (p *roomPlan) openBlock(block dayBlock) {
	p.blockRooms = append(p.blockRooms, block.RoomID)
	for _, id := range block.StudentIDs {
		if _, taken := p.blockRoom[id]; !taken {
			p.blockRoom[id] = block.RoomID
		}
	}
}

func (p *roomPlan) closeBlock(block dayBlock) {
	if i := slices.Index(p.blockRooms, block.RoomID); i >= 0 {
		p.blockRooms = slices.Delete(p.blockRooms, i, i+1)
	}
	for _, id := range block.StudentIDs {
		if p.blockRoom[id] == block.RoomID {
			delete(p.blockRoom, id)
		}
	}
}

func (p *roomPlan) openSession(roomID int64, deviceKey string) {
	p.sessionRooms = append(p.sessionRooms, roomID)
	p.sessionDevice[roomID] = deviceKey
}

func (p *roomPlan) closeSessions() {
	p.sessionRooms, p.sessionDevice, p.next = nil, map[int64]string{}, 0
}

// open reports whether any room takes children.
func (p *roomPlan) open() bool { return len(p.blockRoom) > 0 || len(p.sessionRooms) > 0 }

// isOpen reports whether a running block or session already uses the room.
func (p *roomPlan) isOpen(roomID int64) bool {
	return slices.Contains(p.sessionRooms, roomID) || slices.Contains(p.blockRooms, roomID)
}

// candidates lists the rooms to try for a child: the room of its block
// first, then the session rooms in turn.
func (p *roomPlan) candidates(childID int64) []int64 {
	var rooms []int64
	if room, ok := p.blockRoom[childID]; ok {
		rooms = append(rooms, room)
	}
	for i := range p.sessionRooms {
		room := p.sessionRooms[(p.next+i)%len(p.sessionRooms)]
		if !slices.Contains(rooms, room) {
			rooms = append(rooms, room)
		}
	}
	if len(p.sessionRooms) > 0 {
		p.next = (p.next + 1) % len(p.sessionRooms)
	}
	return rooms
}

func (p *roomPlan) record(childID, roomID int64) {
	p.leave(childID)
	p.childRoom[childID] = roomID
	p.order = append(p.order, childID)
}

func (p *roomPlan) leave(childID int64) {
	if _, ok := p.childRoom[childID]; !ok {
		return
	}
	delete(p.childRoom, childID)
	p.order = slices.DeleteFunc(p.order, func(id int64) bool { return id == childID })
}

// inRooms lists the checked-in children of the rooms, in check-in order.
func (p *roomPlan) inRooms(rooms []int64) []int64 {
	var ids []int64
	for _, id := range p.order {
		if slices.Contains(rooms, p.childRoom[id]) {
			ids = append(ids, id)
		}
	}
	return ids
}

// scheduledArrival is one child arriving at a time of the day.
type scheduledArrival struct {
	at      wallClock
	childID int64
}

// arrivalPlan is when the children come and what they do at midday.
type arrivalPlan struct {
	planned   bool
	pending   []scheduledArrival
	unterwegs []int64
	pickups   []int64
	home      map[int64]bool
}

// planArrivals schedules every child who comes today: at the time of an
// arrival exception (a class whose lessons are cancelled), otherwise with the
// wave of its class, arrivalsPerMinute per minute.
func planArrivals(children []todayChild) arrivalPlan {
	waves := make([][]int64, len(arrivalWaves))
	byTime := map[wallClock][]int64{}
	for _, child := range children {
		switch {
		case child.Absent:
		case child.ArrivalException != 0:
			byTime[child.ArrivalException] = append(byTime[child.ArrivalException], child.ID)
		default:
			w := arrivalWaveIndex(child.Class)
			waves[w] = append(waves[w], child.ID)
		}
	}
	for w, ids := range waves {
		byTime[arrivalWaves[w].start] = append(byTime[arrivalWaves[w].start], ids...)
	}
	plan := arrivalPlan{planned: true, home: map[int64]bool{}}
	for _, start := range slices.Sorted(maps.Keys(byTime)) {
		for n, id := range byTime[start] {
			plan.pending = append(plan.pending, scheduledArrival{at: start + wallClock(n/arrivalsPerMinute), childID: id})
		}
	}
	first, last := waves[0], waves[len(waves)-1]
	plan.pickups = slices.Clone(first[max(0, len(first)-earlyPickupCount):])
	if len(waves) > 1 {
		plan.unterwegs = slices.Clone(last[max(0, len(last)-unterwegsCount):])
	}
	return plan
}

// wait moves the child's arrival to the time of day c, before the children
// planned for then.
func (p *arrivalPlan) wait(childID int64, c wallClock) {
	i, _ := slices.BinarySearchFunc(p.pending, c, func(a scheduledArrival, c wallClock) int { return int(a.at - c) })
	p.pending = slices.Insert(p.pending, i, scheduledArrival{at: c, childID: childID})
}

type sickCallsAction struct{}

func (sickCallsAction) Name() string { return "report sick children" }

func (sickCallsAction) Run(_ context.Context, rt *Runtime) error {
	for i := fullDaySickFrom; i < len(rt.State.Students) && i < fullDaySickTo; i++ {
		student := rt.State.Students[i]
		if _, err := rt.Client.Put(fmt.Sprintf("/api/students/%d", student.ID), map[string]any{"sick": true}); err != nil {
			return fmt.Errorf("mark student %d sick: %w", student.ID, err)
		}
		rt.markAbsent(student.ID)
		rt.Counts.StudentsSick++
	}
	fmt.Printf("  %d children reported sick\n", rt.Counts.StudentsSick)
	return nil
}

// arrivalsAction lets the children arrive whose time lies before until. When
// no room is open yet (a class arriving before the midday rooms), a caregiver
// opens one.
type arrivalsAction struct {
	name           string
	until          wallClock
	unregisteredOK bool
}

func (a arrivalsAction) Name() string { return a.name }

func (a arrivalsAction) Run(ctx context.Context, rt *Runtime) error {
	if !rt.Arrivals.planned {
		rt.Arrivals = planArrivals(rt.Children)
	}
	if a.unregisteredOK {
		fmt.Println("\nPhase 4: Recording attendance and checking in students...")
	}
	if a.unregisteredOK {
		device, err := rt.kioskOf(0)
		if err != nil {
			return err
		}
		if err := recordUnregisteredTag(rt, device); err != nil {
			return err
		}
	}
	for len(rt.Arrivals.pending) > 0 && rt.Arrivals.pending[0].at < a.until {
		next := rt.Arrivals.pending[0]
		rt.Arrivals.pending = rt.Arrivals.pending[1:]
		if err := rt.arriveAt(ctx, next); err != nil {
			return err
		}
	}
	if a.unregisteredOK {
		fmt.Printf("  %d attendance records, %d students checked in\n", rt.Counts.AttendanceRecords, rt.Counts.StudentsCheckedIn)
	}
	return nil
}

// arriveAt lets the child come at its time. A child arriving before the
// midday rooms while nobody is free to open a room waits for them.
func (rt *Runtime) arriveAt(ctx context.Context, next scheduledArrival) error {
	if err := rt.advanceClock(ctx, next.at); err != nil {
		return err
	}
	if err := rt.ensureRoom(ctx, next.at); err != nil {
		return err
	}
	if !rt.Rooms.open() && next.at < sessionsStart {
		rt.Arrivals.wait(next.childID, sessionsStart)
		return nil
	}
	return rt.arrive(next.childID)
}

// recordUnregisteredTag exercises the real kiosk failure path too. The
// endpoint intentionally returns 404, while recording the unknown tag for
// operator follow-up.
func recordUnregisteredTag(rt *Runtime, device SeedDevice) error {
	_, err := rt.Client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": "DEMO-UNREGISTERED-TAG", "action": "checkin",
	}, device.APIKey, rt.State.DevicePIN)
	var expectedErr interface {
		HTTPStatusCode() int
		HTTPErrorCode() string
	}
	if !errors.As(err, &expectedErr) || expectedErr.HTTPStatusCode() != 404 || expectedErr.HTTPErrorCode() != codeRFIDTagNotFound {
		return fmt.Errorf("record unregistered tag scan: expected 404 (rfid_tag_not_found), got %w", err)
	}
	return nil
}

// arrive checks the child into a room. The check-in records the day's
// attendance itself, as a kiosk scan or a block's check-in does. A child no
// room takes is confirmed present at a running kiosk.
func (rt *Runtime) arrive(childID int64) error {
	child, ok := rt.child(childID)
	if !ok {
		return fmt.Errorf("arriving child %d is not a child of the day", childID)
	}
	placed, err := rt.place(child.SeedStudent)
	if err != nil || placed || rt.Present[child.ID] {
		return err
	}
	return rt.confirmAttendance(child.SeedStudent)
}

// place checks the child into the room of its running block, otherwise into
// the session rooms in turn. A full room passes the child on; it reports
// whether a room took the child.
func (rt *Runtime) place(student SeedStudent) (bool, error) {
	for _, roomID := range rt.Rooms.candidates(student.ID) {
		err := rt.checkInto(student, roomID)
		if isCapacityError(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		rt.Rooms.record(student.ID, roomID)
		rt.Counts.StudentsCheckedIn++
		return true, nil
	}
	return false, nil
}

func isCapacityError(err error) bool {
	var coded interface{ HTTPErrorCode() string }
	if !errors.As(err, &coded) {
		return false
	}
	code := coded.HTTPErrorCode()
	return code == codeRoomCapacityExceeded || code == codeActivityCapacityExceeded || code == codeBlockRoomCapacityExceeded
}

func (rt *Runtime) studentTag(student SeedStudent) (string, error) {
	tag, ok := rt.RFIDTags[student.ID]
	if !ok {
		return "", fmt.Errorf("RFID assignment missing for student %d", student.ID)
	}
	return tag, nil
}

// markPresent notes that the day's attendance of the child is recorded.
func (rt *Runtime) markPresent(id int64) {
	if !rt.Present[id] {
		rt.Present[id] = true
		rt.Counts.AttendanceRecords++
	}
}

// confirmAttendance records the child present without a room. The kiosk
// attributes the confirmation to the supervisor of its session, so only a
// kiosk that runs one can confirm; without one the child stays unrecorded.
func (rt *Runtime) confirmAttendance(student SeedStudent) error {
	device, ok := rt.runningKiosk()
	if !ok {
		fmt.Printf("  No kiosk runs a session; student %d waits without a room\n", student.ID)
		return nil
	}
	tag, err := rt.studentTag(student)
	if err != nil {
		return err
	}
	if _, err := rt.Client.DevicePost("/api/iot/attendance/toggle", map[string]string{"rfid": tag, "action": "confirm"}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("record attendance for student %d: %w", student.ID, err)
	}
	rt.markPresent(student.ID)
	return nil
}

// checkInto checks the child into the room the way the room takes children:
// a running planned block through the check-in of its caregiver, a kiosk
// session through a scan at its kiosk.
func (rt *Runtime) checkInto(student SeedStudent, roomID int64) error {
	if block, ok := rt.runningBlockIn(roomID); ok {
		if err := rt.blockAttendance(block, student.ID, "check-in"); err != nil {
			return err
		}
		rt.markPresent(student.ID)
		return nil
	}
	tag, err := rt.studentTag(student)
	if err != nil {
		return err
	}
	device, err := rt.kioskOf(roomID)
	if err != nil {
		return err
	}
	if _, err := rt.Client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": tag, "action": "checkin", "room_id": roomID,
	}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("check in student %d: %w", student.ID, err)
	}
	rt.markPresent(student.ID)
	return nil
}

// leaveRoom ends the child's room visit the way the room took the child; the
// child stays present for the day.
func (rt *Runtime) leaveRoom(student SeedStudent) error {
	roomID := rt.Rooms.childRoom[student.ID]
	if block, ok := rt.runningBlockIn(roomID); ok {
		return rt.blockAttendance(block, student.ID, "check-out")
	}
	tag, err := rt.studentTag(student)
	if err != nil {
		return err
	}
	device, err := rt.kioskOf(roomID)
	if err != nil {
		return err
	}
	if _, err := rt.Client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": tag, "action": "checkout",
	}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("check out student %d: %w", student.ID, err)
	}
	return nil
}

// kioskOf returns the kiosk a child in the room scans at: the kiosk of the
// room's session, else any kiosk that runs a session, else the first one.
func (rt *Runtime) kioskOf(roomID int64) (SeedDevice, error) {
	if key, ok := rt.Rooms.sessionDevice[roomID]; ok {
		return rt.State.Devices[key], nil
	}
	if device, ok := rt.runningKiosk(); ok {
		return device, nil
	}
	return rt.primaryDevice()
}

// runningKiosk returns the first kiosk that runs a session. Only such a kiosk
// can confirm an arrival or a daily checkout: the device API attributes both
// to the supervisor of its session.
func (rt *Runtime) runningKiosk() (SeedDevice, bool) {
	if len(rt.ActiveSessionDeviceKeys) == 0 {
		return SeedDevice{}, false
	}
	return rt.State.Devices[rt.ActiveSessionDeviceKeys[0]], true
}

// checkedInChildrenOutside lists, in check-in order, up to limit children who
// stay in their room through the day and were checked in outside roomID,
// after skipping the first skip of them.
func checkedInChildrenOutside(rt *Runtime, roomID int64, skip, limit int) []int64 {
	ids := make([]int64, 0, limit)
	for _, id := range rt.Rooms.order {
		if rt.Rooms.childRoom[id] == roomID || slices.Contains(rt.Arrivals.unterwegs, id) || slices.Contains(rt.Arrivals.pickups, id) {
			continue
		}
		if skip > 0 {
			skip--
			continue
		}
		ids = append(ids, id)
		if len(ids) == limit {
			break
		}
	}
	return ids
}

type middayActivityAction struct{}

func (middayActivityAction) Name() string { return "run mid-day activity" }

// Run lets two children leave their room without going home, so the day
// shows a few children unterwegs.
func (middayActivityAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 5: Mid-day activity (children unterwegs)...")

	for _, id := range rt.Arrivals.unterwegs {
		child, ok := rt.child(id)
		if _, inRoom := rt.Rooms.childRoom[id]; !ok || !inRoom {
			continue
		}
		if err := rt.leaveRoom(child.SeedStudent); err != nil {
			return err
		}
		rt.Rooms.leave(id)
		rt.Counts.StudentsCheckedOut++
	}

	fmt.Printf("  %d children left their room\n", rt.Counts.StudentsCheckedOut)
	return nil
}

type earlyPickupsAction struct{}

func (earlyPickupsAction) Name() string { return "pick up children early" }

// Run sends a few children of the first wave home after lunch.
func (earlyPickupsAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 5e: Early pickups...")
	for _, id := range rt.Arrivals.pickups {
		child, ok := rt.child(id)
		if !ok || !rt.Present[id] {
			continue
		}
		if err := rt.dailyCheckout(child.SeedStudent); err != nil {
			return err
		}
		rt.Rooms.leave(id)
		rt.Arrivals.home[id] = true
		rt.Counts.EarlyPickups++
	}
	fmt.Printf("  %d children went home\n", rt.Counts.EarlyPickups)
	return nil
}

type endOfDayAction struct{}

func (endOfDayAction) Name() string { return "run end-of-day flow" }

func (endOfDayAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 6: End of day (daily checkout + end sessions)...")

	for _, child := range rt.Children {
		if !rt.Present[child.ID] || rt.Arrivals.home[child.ID] {
			continue
		}
		if err := rt.dailyCheckout(child.SeedStudent); err != nil {
			return err
		}
	}
	fmt.Printf("  %d daily checkouts, %d feedback submitted\n", rt.Counts.DailyCheckouts, rt.Counts.FeedbackSubmitted)

	if err := completeOpenRoomBlock(rt); err != nil {
		return err
	}
	if err := endActiveSessions(rt); err != nil {
		return err
	}
	fmt.Printf("  %d sessions ended\n", rt.Counts.SessionsEnded)
	return nil
}

// dailyCheckout sends a child home through the kiosk: pickup query, daily
// checkout and the smiley feedback, as PyrePortal does. The child scans at
// the kiosk of its room, or at another running kiosk when its room has none.
func (rt *Runtime) dailyCheckout(student SeedStudent) error {
	tag, err := rt.studentTag(student)
	if err != nil {
		return err
	}
	device, ok := rt.runningKiosk()
	if key, inSession := rt.Rooms.sessionDevice[rt.Rooms.childRoom[student.ID]]; inSession {
		device, ok = rt.State.Devices[key], true
	}
	if !ok {
		return fmt.Errorf("daily checkout of student %d: no kiosk runs a session", student.ID)
	}
	if _, err := rt.Client.DevicePost("/api/iot/pickup-query", map[string]any{
		"student_rfid": tag,
	}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("query pickup for student %d: %w", student.ID, err)
	}
	if _, err := rt.Client.DevicePost("/api/iot/attendance/toggle", map[string]any{
		"rfid": tag, "action": "confirm_daily_checkout", "destination": "zuhause",
	}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("record daily checkout for student %d: %w", student.ID, err)
	}
	rt.Counts.DailyCheckouts++

	if _, err := rt.Client.DevicePost("/api/iot/feedback", map[string]any{
		"student_id": student.ID,
		"value":      feedbackValues[rand.Intn(len(feedbackValues))],
	}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("submit feedback for student %d: %w", student.ID, err)
	}
	rt.Counts.FeedbackSubmitted++
	return nil
}
