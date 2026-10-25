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

// Registered error codes of a full room or activity (error-registry.json).
const codeActivityCapacityExceeded = "iot.activity_capacity_exceeded"

// roomPlan tracks the open rooms and where each child of the day is.
type roomPlan struct {
	// blockRooms are the rooms of the running planned blocks; blockRoom is
	// the room of the running block each child is on.
	blockRooms []int64
	blockRoom  map[int64]int64
	// sessionRooms are the rooms of the running kiosk sessions, in turn.
	sessionRooms []int64
	next         int
	// childRoom is the room each child was checked into; order lists the
	// children in check-in order.
	childRoom map[int64]int64
	order     []int64
}

func newRoomPlan() roomPlan {
	return roomPlan{blockRoom: map[int64]int64{}, childRoom: map[int64]int64{}}
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

func (p *roomPlan) openSession(roomID int64) { p.sessionRooms = append(p.sessionRooms, roomID) }

func (p *roomPlan) closeSessions() { p.sessionRooms, p.next = nil, 0 }

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
	device, err := rt.primaryDevice()
	if err != nil {
		return err
	}
	if a.unregisteredOK {
		if err := recordUnregisteredTag(rt, device); err != nil {
			return err
		}
	}
	for len(rt.Arrivals.pending) > 0 && rt.Arrivals.pending[0].at < a.until {
		next := rt.Arrivals.pending[0]
		rt.Arrivals.pending = rt.Arrivals.pending[1:]
		if err := rt.advanceClock(ctx, next.at); err != nil {
			return err
		}
		if err := rt.ensureRoom(ctx, next.at); err != nil {
			return err
		}
		if err := rt.arrive(next.childID, device); err != nil {
			return err
		}
	}
	if a.unregisteredOK {
		fmt.Printf("  %d attendance records, %d students checked in\n", rt.Counts.AttendanceRecords, rt.Counts.StudentsCheckedIn)
	}
	return nil
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

// arrive records the child's attendance unless the early care already did,
// and checks the child into a room.
func (rt *Runtime) arrive(childID int64, device SeedDevice) error {
	child, ok := rt.child(childID)
	if !ok {
		return fmt.Errorf("arriving child %d is not a child of the day", childID)
	}
	if !rt.Present[child.ID] {
		if err := rt.confirmAttendance(child.SeedStudent, device); err != nil {
			return err
		}
	}
	return rt.place(child.SeedStudent, device)
}

// place checks the child into the room of its running block, otherwise into
// the session rooms in turn. A full room passes the child on; a child no room
// takes stays present without a room.
func (rt *Runtime) place(student SeedStudent, device SeedDevice) error {
	for _, roomID := range rt.Rooms.candidates(student.ID) {
		err := rt.checkInto(student, roomID, device)
		if isCapacityError(err) {
			continue
		}
		if err != nil {
			return err
		}
		rt.Rooms.record(student.ID, roomID)
		rt.Counts.StudentsCheckedIn++
		return nil
	}
	return nil
}

func isCapacityError(err error) bool {
	var coded interface{ HTTPErrorCode() string }
	if !errors.As(err, &coded) {
		return false
	}
	code := coded.HTTPErrorCode()
	return code == codeRoomCapacityExceeded || code == codeActivityCapacityExceeded
}

func (rt *Runtime) studentTag(student SeedStudent) (string, error) {
	tag, ok := rt.RFIDTags[student.ID]
	if !ok {
		return "", fmt.Errorf("RFID assignment missing for student %d", student.ID)
	}
	return tag, nil
}

func (rt *Runtime) confirmAttendance(student SeedStudent, device SeedDevice) error {
	tag, err := rt.studentTag(student)
	if err != nil {
		return err
	}
	if _, err := rt.Client.DevicePost("/api/iot/attendance/toggle", map[string]string{"rfid": tag, "action": "confirm"}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("record attendance for student %d: %w", student.ID, err)
	}
	rt.Present[student.ID] = true
	rt.Counts.AttendanceRecords++
	return nil
}

func (rt *Runtime) checkInto(student SeedStudent, roomID int64, device SeedDevice) error {
	tag, err := rt.studentTag(student)
	if err != nil {
		return err
	}
	if _, err := rt.Client.DevicePost("/api/iot/checkin", map[string]any{
		"student_rfid": tag, "action": "checkin", "room_id": roomID,
	}, device.APIKey, rt.State.DevicePIN); err != nil {
		return fmt.Errorf("check in student %d: %w", student.ID, err)
	}
	return nil
}

// leaveRoom ends the child's room visit; the child stays present for the day.
func (rt *Runtime) leaveRoom(student SeedStudent, device SeedDevice) error {
	tag, err := rt.studentTag(student)
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

	primaryDevice, err := rt.primaryDevice()
	if err != nil {
		return err
	}
	for _, id := range rt.Arrivals.unterwegs {
		child, ok := rt.child(id)
		if _, inRoom := rt.Rooms.childRoom[id]; !ok || !inRoom {
			continue
		}
		if err := rt.leaveRoom(child.SeedStudent, primaryDevice); err != nil {
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
	primaryDevice, err := rt.primaryDevice()
	if err != nil {
		return err
	}
	for _, id := range rt.Arrivals.pickups {
		child, ok := rt.child(id)
		if !ok || !rt.Present[id] {
			continue
		}
		if err := rt.dailyCheckout(child.SeedStudent, primaryDevice); err != nil {
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

	primaryDevice, err := rt.primaryDevice()
	if err != nil {
		return err
	}
	for _, child := range rt.Children {
		if !rt.Present[child.ID] || rt.Arrivals.home[child.ID] {
			continue
		}
		if err := rt.dailyCheckout(child.SeedStudent, primaryDevice); err != nil {
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
// checkout and the smiley feedback, as PyrePortal does.
func (rt *Runtime) dailyCheckout(student SeedStudent, device SeedDevice) error {
	tag, err := rt.studentTag(student)
	if err != nil {
		return err
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
