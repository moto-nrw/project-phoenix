package simulate

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"
)

// The demo day (#3921) keeps a weekday of a demo school plausible at any
// hour. Outside the school's own day, today's planned blocks and the
// children's arrival and pickup times move together, so that the current
// hour falls into the afternoon. The ticker then runs the day the way a
// school would: blocks that are over are held and closed, the current ones
// start and end on time, and children come at their arrival and go home at
// their pickup. Weekends keep the open simulation, because they are never
// care days (#3907).
const (
	// demoDayReference is the afternoon hour the current hour maps to.
	demoDayReference = 15*60 + 15
	// demoDayLead keeps the half hour before the first block in the
	// school's own day: the children arrive then.
	demoDayLead = 30
	// demoDayStartLead starts a block shortly before its start, as staff do.
	demoDayStartLead = 15
	// demoDayLastStart is the latest start a moved block may get; a block
	// cannot cross midnight.
	demoDayLastStart  = 23*60 + 45
	demoDayLastMinute = 23*60 + 59
	// demoDayRefresh is how often the ticker rereads the day's blocks.
	demoDayRefresh = time.Minute
	// demoDayHomeGoingPerTick and demoDayArrivalsPerTick bound the children
	// who go home or arrive in one tick.
	demoDayHomeGoingPerTick = 4
	demoDayArrivalsPerTick  = 6
	// demoDayReason is what the moved arrival and pickup times say.
	demoDayReason = "Demo: Tag an die Uhrzeit angepasst"
	// demoActivityLength is how long a planned AG runs; the second wave
	// follows the first.
	demoActivityLength = 60
	// demoActivityChildren bounds an AG's planned children.
	demoActivityChildren = 12
)

// demoActivity is an AG the demo plans for today, in its room and with its
// caregiver.
type demoActivity struct {
	name                string
	id, roomID, staffID int64
}

// demoActivities pairs the school's AGs with a room and a caregiver each,
// the way the kiosk sessions of the open simulation do.
func demoActivities(state *SeedState) []demoActivity {
	names := sortedStringKeys(state.Activities)
	count := min(len(names), len(state.Accounts.Betreuer), 10)
	activities := make([]demoActivity, 0, count)
	for i := range count {
		room := findRoomForActivity(names[i], state.Rooms)
		if room == 0 {
			continue
		}
		activities = append(activities, demoActivity{
			name: names[i], id: state.Activities[names[i]], roomID: room, staffID: state.Accounts.Betreuer[i].StaffID,
		})
	}
	return activities
}

// demoBlock is one timetable block of today as the instance list returns it.
type demoBlock struct {
	ID              int64   `json:"id"`
	Date            string  `json:"date"`
	StartTime       string  `json:"start_time"`
	EndTime         string  `json:"end_time"`
	Title           string  `json:"title"`
	Description     *string `json:"description,omitempty"`
	Notes           *string `json:"notes,omitempty"`
	Status          string  `json:"status"`
	IsSpontaneous   bool    `json:"is_spontaneous"`
	ActivityGroupID *int64  `json:"activity_group_id,omitempty"`
	ListKind        *string `json:"list_kind,omitempty"`
	RoomID          int64   `json:"room_id"`
	Staff           []struct {
		StaffID int64 `json:"staff_id"`
	} `json:"staff"`
	StudentIDs []int64 `json:"student_ids"`
	Students   []struct {
		StudentID int64  `json:"student_id"`
		Status    string `json:"status"`
	} `json:"students"`
}

// demoTimes are a child's effective arrival and pickup today, in minutes
// after midnight; -1 when the plan has none.
type demoTimes struct {
	arrival, pickup int
}

// demoDay is the ticker's view of one weekday of one school.
type demoDay struct {
	date       string
	planned    bool
	activities bool
	devices    bool
	blocks     []demoBlock
	loadedAt   time.Time
	times      map[int64]demoTimes
	timesAt    time.Time
	// failed are blocks whose step failed today; they are not retried, so
	// a refused start does not repeat every tick.
	failed map[int64]bool
	// home are the children who went home today.
	home map[int64]bool
}

var demoBerlin = func() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(fmt.Sprintf("load Europe/Berlin: %v", err))
	}
	return location
}()

// demoWeekday reports whether now is a care day of the week in Berlin.
func demoWeekday(now time.Time) bool {
	weekday := now.In(demoBerlin).Weekday()
	return weekday != time.Saturday && weekday != time.Sunday
}

// sync brings today's plan up to date: it reads the blocks, moves the day
// once if the current hour lies outside it, and starts and closes blocks.
// It reports whether it changed anything the children's state depends on.
func (day *demoDay) sync(client Client, now time.Time, studentIDs []int64, activities []demoActivity) (bool, error) {
	local := now.In(demoBerlin)
	date, minute := local.Format(isoDate), local.Hour()*60+local.Minute()
	if day.date != date {
		*day = demoDay{date: date, failed: map[int64]bool{}, home: map[int64]bool{}}
	}
	if day.blocks == nil || now.Sub(day.loadedAt) >= demoDayRefresh {
		if err := day.load(client, now); err != nil {
			return false, err
		}
	}
	if !day.planned {
		if shift := demoDayShift(day.blocks, minute); shift != 0 {
			if err := day.move(client, shift, studentIDs); err != nil {
				return false, err
			}
			if err := day.load(client, now); err != nil {
				return false, err
			}
		}
		day.planned = true
	}
	if day.times == nil || now.Sub(day.timesAt) >= 5*demoDayRefresh {
		if err := day.loadTimes(client, studentIDs, now); err != nil {
			return false, err
		}
	}
	if !day.activities {
		if day.planActivities(client, activities, studentIDs, minute) {
			if err := day.load(client, now); err != nil {
				return false, err
			}
		}
		day.activities = true
	}
	if !day.run(client, minute) {
		return false, nil
	}
	return true, day.load(client, now)
}

func (day *demoDay) load(client Client, now time.Time) error {
	raw, err := client.Get(fmt.Sprintf("/api/timetable/instances?from=%s&to=%s", day.date, day.date))
	if err != nil {
		return fmt.Errorf("read demo day blocks: %w", err)
	}
	var envelope struct {
		Data struct {
			Instances []demoBlock `json:"instances"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode demo day blocks: %w", err)
	}
	day.blocks = envelope.Data.Instances
	if day.blocks == nil {
		day.blocks = []demoBlock{}
	}
	day.loadedAt = now
	return nil
}

// demoDayShift returns the minutes today's planned blocks move by, or 0
// when they stay: while a block runs, and while the current hour lies within
// the day the planned blocks span. Blocks that are over keep their place.
func demoDayShift(blocks []demoBlock, minute int) int {
	first, last := -1, -1
	for _, block := range blocks {
		if block.IsSpontaneous {
			continue
		}
		if block.Status == "active" {
			return 0
		}
		if block.Status != "planned" {
			continue
		}
		start, end, ok := blockMinutes(block)
		if !ok {
			continue
		}
		if first < 0 || start < first {
			first = start
		}
		if end > last {
			last = end
		}
	}
	if first < 0 || (minute >= first-demoDayLead && minute < last) {
		return 0
	}
	return (minute - demoDayReference) / 5 * 5
}

// movedWindow is a block's window after the shift; ok is false when the
// block would leave the day and therefore stays where it is.
func movedWindow(start, end, shift int) (int, int, bool) {
	start, end = start+shift, end+shift
	if start < 0 || start > demoDayLastStart {
		return 0, 0, false
	}
	return start, min(end, demoDayLastMinute), true
}

// move shifts today's planned blocks and the children's arrival and pickup
// times by shift minutes.
func (day *demoDay) move(client Client, shift int, studentIDs []int64) error {
	for _, block := range day.blocks {
		if block.IsSpontaneous || block.Status != "planned" {
			continue
		}
		start, end, ok := blockMinutes(block)
		if !ok {
			continue
		}
		start, end, ok = movedWindow(start, end, shift)
		if !ok {
			continue
		}
		if _, err := client.Put(fmt.Sprintf("/api/timetable/instances/%d", block.ID), block.updateBody(start, end)); err != nil {
			slog.Warn("demo day: block not moved",
				"instance_id", block.ID,
				"error", err,
			)
		}
	}
	if err := day.loadTimes(client, studentIDs, time.Time{}); err != nil {
		return err
	}
	movedPickups := map[int64]string{}
	for _, studentID := range studentIDs {
		times, ok := day.times[studentID]
		if !ok || times.arrival < 0 {
			continue // No care today: the child stays at home.
		}
		arrival := max(times.arrival+shift, 0)
		if arrival > demoDayLastStart {
			continue
		}
		path := fmt.Sprintf("/api/students/%d/arrival-exceptions", studentID)
		if _, err := client.Post(path, map[string]any{
			"exception_date": day.date, "expected_arrival": clockOf(arrival), "reason": demoDayReason,
		}); err != nil {
			slog.Warn("demo day: arrival not moved",
				"student_id", studentID,
				"error", err,
			)
			continue
		}
		if times.pickup < 0 {
			continue
		}
		pickup := max(min(times.pickup+shift, demoDayLastMinute), arrival+1)
		path = fmt.Sprintf("/api/students/%d/pickup-exceptions", studentID)
		movedPickups[studentID] = clockOf(pickup)
		if _, err := client.Post(path, map[string]any{
			"exception_date": day.date, "pickup_time": clockOf(pickup), "reason": demoDayReason,
		}); err != nil {
			slog.Warn("demo day: pickup not moved",
				"student_id", studentID,
				"error", err,
			)
		}
	}
	day.times = nil
	day.settlePickupTasks(client, movedPickups)
	return nil
}

// settlePickupTasks closes the tasks the moved pickups opened (#3261): a
// later pickup asks the Leitung for a block, but the whole day moved with it.
// Tasks the demo was seeded with stay open.
func (day *demoDay) settlePickupTasks(client Client, moved map[int64]string) {
	if len(moved) == 0 {
		return
	}
	raw, err := client.Get("/api/timetable/pickup-extensions")
	if err != nil {
		slog.Info("demo day: pickup tasks not read", "error", err)
		return
	}
	var envelope struct {
		Data struct {
			Tasks []struct {
				ID         int64  `json:"id"`
				StudentID  int64  `json:"student_id"`
				Date       string `json:"date"`
				PickupTime string `json:"pickup_time"`
			} `json:"tasks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		slog.Info("demo day: pickup tasks not decoded", "error", err)
		return
	}
	for _, task := range envelope.Data.Tasks {
		if task.Date != day.date || minutesOf(&task.PickupTime) != minutesOf(ptr(moved[task.StudentID])) {
			continue
		}
		path := fmt.Sprintf("/api/timetable/pickup-extensions/%d/resolve", task.ID)
		if _, err := client.Post(path, map[string]any{"block_ids": []int64{}}); err != nil {
			slog.Info("demo day: pickup task not closed",
				"task_id", task.ID,
				"error", err,
			)
		}
	}
}

func ptr(value string) *string { return &value }

// planActivities plans the school's AGs for today as one-off blocks around
// the current hour, in two waves, each with children who come today. An AG
// that already has a block today, or whose room another block with children
// uses at the time, is left out. It reports whether it planned any.
func (day *demoDay) planActivities(client Client, activities []demoActivity, studentIDs []int64, minute int) bool {
	planned := map[int64]bool{}
	for _, block := range day.blocks {
		if !block.IsSpontaneous && block.ActivityGroupID != nil {
			planned[*block.ActivityGroupID] = true
		}
	}
	var children []int64
	for _, studentID := range studentIDs {
		if times, ok := day.times[studentID]; ok && times.arrival >= 0 {
			children = append(children, studentID)
		}
	}
	base := minute / 5 * 5
	waves := [2]int{base - demoActivityLength/2, base + demoActivityLength/2}
	created, placed := false, 0
	next := [2]int{}
	for _, activity := range activities {
		if planned[activity.id] {
			continue
		}
		// The waves alternate over the AGs that get a block, so a skipped AG
		// does not leave one wave short.
		wave := placed % 2
		start, end := max(waves[wave], 0), min(waves[wave]+demoActivityLength, demoDayLastMinute)
		if start > demoDayLastStart || end <= start || day.roomTaken(activity.roomID, start, end) {
			continue
		}
		placed++
		roster := make([]int64, 0, demoActivityChildren)
		for len(roster) < demoActivityChildren && next[wave] < len(children) {
			roster = append(roster, children[next[wave]])
			next[wave]++
		}
		_, err := client.Post("/api/timetable/instances", map[string]any{
			"date": day.date, "start_time": clockOf(start), "end_time": clockOf(end),
			"title": activity.name, "room_id": activity.roomID, "activity_group_id": activity.id,
			"staff_ids": []int64{activity.staffID}, "student_ids": roster,
		})
		if err != nil {
			slog.Info("demo day: activity not planned",
				"activity_id", activity.id,
				"error", err,
			)
			continue
		}
		created = true
	}
	return created
}

// roomTaken reports whether another block with children uses the room at
// some point between start and end.
func (day *demoDay) roomTaken(roomID int64, start, end int) bool {
	for _, block := range day.blocks {
		if block.IsSpontaneous || block.Status == "cancelled" || block.RoomID != roomID || len(block.StudentIDs) == 0 {
			continue
		}
		blockStart, blockEnd, ok := blockMinutes(block)
		if ok && blockStart < end && start < blockEnd {
			return true
		}
	}
	return false
}

func (block demoBlock) updateBody(start, end int) map[string]any {
	staffIDs := make([]int64, 0, len(block.Staff))
	for _, staff := range block.Staff {
		staffIDs = append(staffIDs, staff.StaffID)
	}
	body := map[string]any{
		"date": block.Date, "start_time": clockOf(start), "end_time": clockOf(end),
		"title": block.Title, "room_id": block.RoomID,
		"staff_ids": staffIDs, "student_ids": block.StudentIDs,
	}
	if block.Description != nil {
		body["description"] = *block.Description
	}
	if block.Notes != nil {
		body["notes"] = *block.Notes
	}
	if block.ActivityGroupID != nil {
		body["activity_group_id"] = *block.ActivityGroupID
	}
	if block.ListKind != nil {
		body["list_kind"] = *block.ListKind
	}
	return body
}

// loadTimes reads the children's effective arrival and pickup today.
func (day *demoDay) loadTimes(client Client, studentIDs []int64, now time.Time) error {
	day.times = make(map[int64]demoTimes, len(studentIDs))
	if len(studentIDs) == 0 {
		day.timesAt = now
		return nil
	}
	for _, studentID := range studentIDs {
		day.times[studentID] = demoTimes{arrival: -1, pickup: -1}
	}
	body := map[string]any{"student_ids": studentIDs, "date": day.date}
	var arrivals, pickups struct {
		Data []struct {
			StudentID       int64   `json:"student_id"`
			ExpectedArrival *string `json:"expected_arrival"`
			PickupTime      *string `json:"pickup_time"`
		} `json:"data"`
	}
	for path, target := range map[string]any{
		"/api/students/arrival-times/bulk": &arrivals, "/api/students/pickup-times/bulk": &pickups,
	} {
		raw, err := client.Post(path, body)
		if err != nil {
			return fmt.Errorf("read demo day times: %w", err)
		}
		if err := json.Unmarshal(raw, target); err != nil {
			return fmt.Errorf("decode demo day times: %w", err)
		}
	}
	for _, row := range arrivals.Data {
		times := day.times[row.StudentID]
		times.arrival = minutesOf(row.ExpectedArrival)
		day.times[row.StudentID] = times
	}
	for _, row := range pickups.Data {
		times := day.times[row.StudentID]
		times.pickup = minutesOf(row.PickupTime)
		day.times[row.StudentID] = times
	}
	day.timesAt = now
	return nil
}

// run starts, holds and closes today's blocks; it reports whether anything
// changed, so the caller rereads the blocks.
func (day *demoDay) run(client Client, minute int) bool {
	changed := false
	for _, block := range day.blocks {
		if block.IsSpontaneous || day.failed[block.ID] {
			continue
		}
		start, end, ok := blockMinutes(block)
		if !ok {
			continue
		}
		var err error
		switch {
		case block.Status == "planned" && end <= minute:
			err = day.hold(client, block)
		case block.Status == "planned" && start-demoDayStartLead <= minute && len(block.StudentIDs) == 0 && day.roomTaken(block.RoomID, start, end):
			// A block without children (a team meeting) in the room of a block
			// with children counts as held at once: running, it would take
			// the room's check-ins, since the kiosk joins the newest session.
			err = day.hold(client, block)
		case block.Status == "planned" && start-demoDayStartLead <= minute:
			err = startBlock(client, block.ID)
		case block.Status == "active" && end <= minute:
			err = completeBlock(client, block.ID)
		default:
			continue
		}
		changed = true
		if err != nil {
			// A duty is never started (#3822); its refusal lands here too.
			day.failed[block.ID] = true
			slog.Info("demo day: block step refused",
				"instance_id", block.ID,
				"error", err,
			)
		}
	}
	return changed
}

// hold runs a block that is already over: the children planned for it are
// checked in and the block is closed, so it reads as held, not as missed.
func (day *demoDay) hold(client Client, block demoBlock) error {
	if err := startBlock(client, block.ID); err != nil {
		return err
	}
	expected := make([]int64, 0, len(block.Students))
	for _, student := range block.Students {
		if student.Status == "expected" {
			expected = append(expected, student.StudentID)
		}
	}
	var checkInErr error
	if len(expected) > 0 {
		path := fmt.Sprintf("/api/timetable/operations/instances/%d/students/check-in", block.ID)
		_, checkInErr = client.Post(path, map[string]any{"student_ids": expected})
	}
	// Close it even after a failed check-in: a past block must not keep running.
	return errors.Join(checkInErr, completeBlock(client, block.ID))
}

func startBlock(client Client, id int64) error {
	_, err := client.Post(fmt.Sprintf("/api/timetable/operations/instances/%d/start", id), nil)
	return err
}

// completeBlock closes a block the way the planner does: without a live
// roster to confirm, the children still expected count as absent.
func completeBlock(client Client, id int64) error {
	_, err := client.Post(fmt.Sprintf("/api/timetable/instances/%d/complete", id), nil)
	return err
}

// rooms are the rooms of the blocks running now with children planned, and
// for each planned child the room of its block.
func (day *demoDay) rooms() ([]int64, map[int64]int64) {
	var rooms []int64
	seen := map[int64]bool{}
	planned := map[int64]int64{}
	for _, block := range day.blocks {
		if block.IsSpontaneous || block.Status != "active" || block.RoomID == 0 || len(block.StudentIDs) == 0 {
			continue
		}
		if !seen[block.RoomID] {
			seen[block.RoomID] = true
			rooms = append(rooms, block.RoomID)
		}
		for _, studentID := range block.StudentIDs {
			planned[studentID] = block.RoomID
		}
	}
	return rooms, planned
}

// present reports whether the child is due at the OGS at minute: arrived
// and not yet picked up. A child without a plan for today stays at home.
func (day *demoDay) present(studentID int64, minute int) bool {
	times, ok := day.times[studentID]
	if !ok || times.arrival < 0 || day.home[studentID] {
		return false
	}
	return times.arrival <= minute && (times.pickup < 0 || minute < times.pickup)
}

// pickedUp reports whether the child's pickup is due and it has not left.
func (day *demoDay) pickedUp(studentID int64, minute int) bool {
	times, ok := day.times[studentID]
	return ok && times.arrival >= 0 && times.pickup >= 0 && minute >= times.pickup && !day.home[studentID]
}

func blockMinutes(block demoBlock) (int, int, bool) {
	start, end := minutesOf(&block.StartTime), minutesOf(&block.EndTime)
	return start, end, start >= 0 && end > start
}

// minutesOf parses HH:MM or HH:MM:SS; -1 for nil or malformed.
func minutesOf(clock *string) int {
	if clock == nil || len(*clock) < 5 || (*clock)[2] != ':' {
		return -1
	}
	hours, errHours := strconv.Atoi((*clock)[:2])
	minutes, errMinutes := strconv.Atoi((*clock)[3:5])
	if errHours != nil || errMinutes != nil || hours > 23 || minutes > 59 {
		return -1
	}
	return hours*60 + minutes
}

func clockOf(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}
