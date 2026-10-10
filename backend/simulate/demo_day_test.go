package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Friday 2026-09-11, a care day; the clock is set per test in Berlin time.
func demoDayAt(t *testing.T, clock string) time.Time {
	t.Helper()
	at, err := time.ParseInLocation("2006-01-02 15:04", "2026-09-11 "+clock, demoBerlin)
	require.NoError(t, err)
	return at
}

func plannedBlock(id int64, start, end string, room int64, students ...int64) demoBlock {
	block := demoBlock{ID: id, Date: "2026-09-11", StartTime: start, EndTime: end, Title: fmt.Sprintf("Block %d", id), Status: "planned", RoomID: room, StudentIDs: students}
	for _, studentID := range students {
		block.Students = append(block.Students, struct {
			StudentID int64  `json:"student_id"`
			Status    string `json:"status"`
		}{StudentID: studentID, Status: "expected"})
	}
	return block
}

func TestDemoDayShift(t *testing.T) {
	t.Parallel()
	day := []demoBlock{plannedBlock(1, "07:30", "08:30", 1), plannedBlock(2, "14:45", "17:00", 1)}
	active := append([]demoBlock{}, day...)
	active[1].Status = "active"
	completed := append([]demoBlock{}, day...)
	completed[0].Status = "completed"
	spontaneous := plannedBlock(3, "20:00", "21:00", 1)
	spontaneous.IsSpontaneous = true

	for name, tc := range map[string]struct {
		blocks []demoBlock
		minute int
		want   int
	}{
		"evening moves now to the afternoon":         {day, 20*60 + 3, 4*60 + 45},
		"night moves the day back":                   {day, 2 * 60, -(13*60 + 15)},
		"the school's own day stays":                 {day, 15 * 60, 0},
		"the half hour before the first block stays": {day, 7 * 60, 0},
		"the end of the last block moves":            {day, 17 * 60, 105},
		"a running block keeps the day":              {active, 20 * 60, 0},
		"a block that is over keeps its place":       {completed, 20 * 60, 4*60 + 45},
		"spontaneous blocks do not count":            {[]demoBlock{spontaneous}, 20 * 60, 0},
		"no blocks, no move":                         {nil, 20 * 60, 0},
	} {
		assert.Equal(t, tc.want, demoDayShift(tc.blocks, tc.minute), name)
	}
}

func TestMovedWindowStaysOnTheDay(t *testing.T) {
	t.Parallel()
	start, end, ok := movedWindow(14*60+45, 17*60, 7*60+30)
	assert.True(t, ok)
	assert.Equal(t, [2]int{22*60 + 15, demoDayLastMinute}, [2]int{start, end}, "the end is capped at 23:59")
	_, _, ok = movedWindow(16*60, 16*60+30, 7*60+50)
	assert.False(t, ok, "a block starting after 23:45 stays where it is")
	_, _, ok = movedWindow(7*60+30, 8*60+30, -(13*60 + 15))
	assert.False(t, ok, "a block before midnight stays where it is")
}

func TestDemoDayPresence(t *testing.T) {
	t.Parallel()
	day := demoDay{home: map[int64]bool{4: true}, times: map[int64]demoTimes{
		1: {arrival: 12 * 60, pickup: 16 * 60}, 2: {arrival: -1, pickup: -1},
		3: {arrival: 12 * 60, pickup: -1}, 4: {arrival: 12 * 60, pickup: 13 * 60},
	}}
	assert.False(t, day.present(1, 11*60+59), "not arrived yet")
	assert.True(t, day.present(1, 12*60))
	assert.False(t, day.present(1, 16*60), "picked up")
	assert.True(t, day.pickedUp(1, 16*60))
	assert.False(t, day.present(2, 13*60), "no care today")
	assert.False(t, day.pickedUp(2, 23*60))
	assert.True(t, day.present(3, 23*60), "without a pickup the child stays")
	assert.False(t, day.pickedUp(4, 14*60), "already went home")
}

// demoDayClient keeps the day's blocks and records the writes, the way the
// timetable and care plan endpoints change them.
type demoDayClient struct {
	Client
	blocks     []demoBlock
	arrivals   map[int64]string
	pickups    map[int64]string
	writes     []string
	exceptions map[string]map[string]any
	created    []map[string]any
	tasks      []map[string]any
}

func newDemoDayClient(blocks ...demoBlock) *demoDayClient {
	return &demoDayClient{
		blocks:     blocks,
		arrivals:   map[int64]string{},
		pickups:    map[int64]string{},
		exceptions: map[string]map[string]any{},
	}
}

func (c *demoDayClient) Get(path string) ([]byte, error) {
	if path == "/api/timetable/pickup-extensions" {
		return json.Marshal(map[string]any{"data": map[string]any{"tasks": c.tasks}})
	}
	if !strings.HasPrefix(path, "/api/timetable/instances?") {
		return nil, fmt.Errorf("unexpected GET %s", path)
	}
	query, err := url.ParseQuery(strings.TrimPrefix(path, "/api/timetable/instances?"))
	if err != nil {
		return nil, err
	}
	from, to := query.Get("from"), query.Get("to")
	blocks := []demoBlock{}
	for _, block := range c.blocks {
		if block.Date >= from && block.Date <= to {
			blocks = append(blocks, block)
		}
	}
	return json.Marshal(map[string]any{"data": map[string]any{"instances": blocks}})
}

func (c *demoDayClient) Put(path string, body any) ([]byte, error) {
	c.writes = append(c.writes, "PUT "+path)
	values := body.(map[string]any)
	for i := range c.blocks {
		if path == fmt.Sprintf("/api/timetable/instances/%d", c.blocks[i].ID) {
			c.blocks[i].StartTime, c.blocks[i].EndTime = values["start_time"].(string), values["end_time"].(string)
		}
	}
	return []byte(`{"data":{}}`), nil
}

func (c *demoDayClient) Post(path string, body any) ([]byte, error) {
	switch {
	case path == "/api/students/arrival-times/bulk" || path == "/api/students/pickup-times/bulk":
		source, field := c.arrivals, "expected_arrival"
		if strings.Contains(path, "pickup") {
			source, field = c.pickups, "pickup_time"
		}
		var rows []map[string]any
		for _, studentID := range body.(map[string]any)["student_ids"].([]int64) {
			row := map[string]any{"student_id": studentID}
			if clock, ok := source[studentID]; ok {
				row[field] = clock
			}
			rows = append(rows, row)
		}
		return json.Marshal(map[string]any{"data": rows})
	case strings.HasSuffix(path, "-exceptions"):
		c.exceptions[path] = body.(map[string]any)
	case path == "/api/timetable/instances":
		c.created = append(c.created, body.(map[string]any))
	}
	c.writes = append(c.writes, "POST "+path)
	for i := range c.blocks {
		switch path {
		case fmt.Sprintf("/api/timetable/operations/instances/%d/start", c.blocks[i].ID):
			c.blocks[i].Status = "active"
		case fmt.Sprintf("/api/timetable/instances/%d/complete", c.blocks[i].ID):
			c.blocks[i].Status = "completed"
		}
	}
	return []byte(`{"data":{}}`), nil
}

func TestDemoDayMovesTheEveningAndRunsIt(t *testing.T) {
	t.Parallel()
	client := newDemoDayClient(
		plannedBlock(1, "13:00", "14:00", 5, 11, 12),
		plannedBlock(2, "14:45", "17:00", 5, 11),
		plannedBlock(3, "16:00", "16:30", 0),
		plannedBlock(4, "15:00", "15:45", 5),
	)
	client.arrivals[11], client.pickups[11] = "11:45", "16:30"
	client.arrivals[12] = "12:45"
	client.tasks = []map[string]any{
		{"id": 71, "student_id": 11, "date": "2026-09-11", "pickup_time": "21:15"},
		{"id": 72, "student_id": 12, "date": "2026-09-11", "pickup_time": "17:00"},
	}
	var day demoDay

	changed, err := day.sync(client, demoDayAt(t, "20:00"), []int64{11, 12, 13}, nil)
	require.NoError(t, err)
	assert.True(t, changed)

	// 20:00 is 15:15 in the moved day: 4 h 45 min later than planned.
	assert.Equal(t, [2]string{"17:45", "18:45"}, [2]string{client.blocks[0].StartTime, client.blocks[0].EndTime})
	assert.Equal(t, [2]string{"19:30", "21:45"}, [2]string{client.blocks[1].StartTime, client.blocks[1].EndTime})
	assert.Equal(t, [2]string{"20:45", "21:15"}, [2]string{client.blocks[2].StartTime, client.blocks[2].EndTime})
	assert.Equal(t, "16:30", client.exceptions["/api/students/11/arrival-exceptions"]["expected_arrival"])
	assert.Equal(t, "21:15", client.exceptions["/api/students/11/pickup-exceptions"]["pickup_time"])
	assert.Equal(t, "17:30", client.exceptions["/api/students/12/arrival-exceptions"]["expected_arrival"])
	assert.NotContains(t, client.exceptions, "/api/students/12/pickup-exceptions", "no pickup, nothing to move")
	assert.NotContains(t, client.exceptions, "/api/students/13/arrival-exceptions", "no care today")
	assert.Contains(t, client.writes, "POST /api/timetable/pickup-extensions/71/resolve", "the moved pickup needs no block")
	assert.NotContains(t, client.writes, "POST /api/timetable/pickup-extensions/72/resolve", "a seeded task stays open")

	// The block that is over was held: its children checked in, then closed.
	assert.Equal(t, "completed", client.blocks[0].Status)
	assert.Contains(t, client.writes, "POST /api/timetable/operations/instances/1/students/check-in")
	assert.Equal(t, "active", client.blocks[1].Status, "the current block runs")
	assert.Equal(t, "planned", client.blocks[2].Status, "a block without children does not run early")
	assert.Equal(t, "completed", client.blocks[3].Status, "a team meeting in the room of a running block counts as held at once")

	// A second sync the same evening moves nothing again.
	client.writes = nil
	_, err = day.sync(client, demoDayAt(t, "20:05"), []int64{11, 12, 13}, nil)
	require.NoError(t, err)
	for _, write := range client.writes {
		assert.NotContains(t, write, "PUT", "the day moves once")
	}

	// At its end the running block closes; the team meeting counts as held.
	_, err = day.sync(client, demoDayAt(t, "21:45"), []int64{11, 12, 13}, nil)
	require.NoError(t, err)
	assert.Equal(t, "completed", client.blocks[1].Status)
	assert.Equal(t, "completed", client.blocks[2].Status)
}

func TestDemoDayKeepsTheSchoolsOwnDay(t *testing.T) {
	t.Parallel()
	client := newDemoDayClient(plannedBlock(1, "13:00", "14:00", 5, 11), plannedBlock(2, "14:45", "17:00", 5, 11))
	var day demoDay
	_, err := day.sync(client, demoDayAt(t, "15:00"), []int64{11}, nil)
	require.NoError(t, err)
	for _, write := range client.writes {
		assert.NotContains(t, write, "PUT")
		assert.NotContains(t, write, "-exceptions")
	}
	assert.Equal(t, "completed", client.blocks[0].Status, "the lunch block was held")
	assert.Equal(t, "active", client.blocks[1].Status)
}

func demoDayStudents(first, last int64) []int64 {
	ids := make([]int64, 0, last-first+1)
	for id := first; id <= last; id++ {
		ids = append(ids, id)
	}
	return ids
}

var demoDayActivities = []demoActivity{
	{name: "Basteln", id: 101, roomID: 8, staffID: 201},
	{name: "Fußball", id: 102, roomID: 9, staffID: 202},
	{name: "Hausaufgaben", id: 103, roomID: 7, staffID: 203},
	{name: "Kochen", id: 104, roomID: 10, staffID: 204},
}

func createdByTitle(client *demoDayClient) map[string]map[string]any {
	byTitle := map[string]map[string]any{}
	for _, body := range client.created {
		byTitle[body["title"].(string)] = body
	}
	return byTitle
}

func TestDemoDayPlansTheActivitiesInTheAfternoon(t *testing.T) {
	t.Parallel()
	client := newDemoDayClient(plannedBlock(1, "07:30", "08:30", 6, 11), plannedBlock(2, "13:00", "17:00", 7, 11))
	for _, id := range demoDayStudents(11, 40) {
		client.arrivals[id] = "11:45"
	}
	var day demoDay
	// In the morning the AGs are still to come, at their afternoon hour.
	_, err := day.sync(client, demoDayAt(t, "08:00"), demoDayStudents(11, 40), demoDayActivities)
	require.NoError(t, err)

	require.Len(t, client.created, 3, "Hausaufgaben shares its room with the long block")
	byTitle := createdByTitle(client)
	assert.Equal(t, [2]any{"14:45", "15:45"}, [2]any{byTitle["Basteln"]["start_time"], byTitle["Basteln"]["end_time"]})
	assert.Equal(t, [2]any{"15:45", "16:45"}, [2]any{byTitle["Fußball"]["start_time"], byTitle["Fußball"]["end_time"]})
	assert.Equal(t, "14:45", byTitle["Kochen"]["start_time"])
	assert.Len(t, byTitle["Basteln"]["student_ids"], demoActivityChildren)
	assert.NotEqual(t, byTitle["Basteln"]["student_ids"], byTitle["Kochen"]["student_ids"], "one AG per child and wave")

	client.created = nil
	_, err = day.sync(client, demoDayAt(t, "10:30"), demoDayStudents(11, 40), demoDayActivities)
	require.NoError(t, err)
	assert.Empty(t, client.created, "the activities are planned once")
}

func TestDemoDaySkipsActivitiesTheTimetableAlreadyHas(t *testing.T) {
	t.Parallel()
	own := plannedBlock(1, "14:00", "15:00", 8, 11)
	own.Title = "Basteln"
	client := newDemoDayClient(plannedBlock(2, "07:30", "08:30", 6, 11), own)
	client.arrivals[11] = "11:45"
	var day demoDay
	_, err := day.sync(client, demoDayAt(t, "10:00"), []int64{11}, demoDayActivities[:2])
	require.NoError(t, err)
	require.Len(t, client.created, 1, "Basteln is in the school's own timetable today")
	assert.Equal(t, "Fußball", client.created[0]["title"])
}

func TestDemoDayPlansTheActivitiesAroundAMovedAfternoon(t *testing.T) {
	t.Parallel()
	client := newDemoDayClient(plannedBlock(1, "13:00", "14:00", 7, 11))
	client.arrivals[11] = "11:45"
	var day demoDay
	_, err := day.sync(client, demoDayAt(t, "20:00"), []int64{11}, demoDayActivities[:1])
	require.NoError(t, err)
	require.Len(t, client.created, 1)
	assert.Equal(t, "19:30", client.created[0]["start_time"], "15:15 of the moved day is 20:00")
}

func TestDemoDayRebuildsAnEndedDayForALateVisitor(t *testing.T) {
	t.Parallel()
	lunch := plannedBlock(1, "13:00", "14:00", 7, 11, 12)
	lunch.Status = "completed"
	client := newDemoDayClient(lunch)
	client.arrivals[11], client.pickups[11] = "11:45", "15:30"
	client.arrivals[12], client.pickups[12] = "11:45", "16:00"
	var day demoDay
	// The visitor saw the afternoon at 14:00; at 19:00 nothing is left.
	day.date, day.anchor, day.anchoredAt, day.planned, day.activities = "2026-09-11", demoDayReference, -1, true, true
	day.failed, day.home = map[int64]bool{}, map[int64]bool{11: true}

	_, err := day.sync(client, demoDayAt(t, "19:00"), []int64{11, 12, 13}, demoDayActivities[:2])
	require.NoError(t, err)

	assert.Equal(t, "18:00", client.exceptions["/api/students/11/arrival-exceptions"]["expected_arrival"])
	assert.Equal(t, "20:30", client.exceptions["/api/students/11/pickup-exceptions"]["pickup_time"])
	assert.Equal(t, "18:15", client.exceptions["/api/students/12/arrival-exceptions"]["expected_arrival"])
	assert.NotContains(t, client.exceptions, "/api/students/13/arrival-exceptions", "no care today")
	byTitle := createdByTitle(client)
	assert.Equal(t, "18:30", byTitle["Basteln"]["start_time"], "the AGs run again around the current hour")
	assert.Equal(t, "19:30", byTitle["Fußball"]["start_time"])
	assert.Empty(t, day.home, "the children come again")

	// While the new afternoon runs, nothing is rebuilt.
	client.created = nil
	_, err = day.sync(client, demoDayAt(t, "19:30"), []int64{11, 12, 13}, demoDayActivities[:2])
	require.NoError(t, err)
	assert.Empty(t, client.created)
}

// demoPlanClient serves a planned weekday to the ticker: the device sessions,
// the day's blocks and the children's times.
type demoPlanClient struct {
	demoDayClient
	device demoRecordingClient
	ended  int
}

type retryLeftoverClient struct {
	*demoDayClient
	failComplete bool
}

func (c *retryLeftoverClient) Post(path string, body any) ([]byte, error) {
	if c.failComplete && strings.HasSuffix(path, "/complete") {
		c.failComplete = false
		return nil, fmt.Errorf("temporary completion failure")
	}
	return c.demoDayClient.Post(path, body)
}

type retryDeviceSessionClient struct {
	*demoPlanClient
	failEnd  bool
	endCalls int
}

func (c *retryDeviceSessionClient) DevicePost(path string, body any, key, pin string) ([]byte, error) {
	if path == "/api/iot/session/end" {
		c.endCalls++
		if c.failEnd {
			c.failEnd = false
			return nil, fmt.Errorf("temporary session shutdown failure")
		}
	}
	return c.demoPlanClient.DevicePost(path, body, key, pin)
}

func TestDemoTickerRetriesFailedLeftoverClosure(t *testing.T) {
	t.Parallel()
	block := plannedBlock(9, "14:00", "15:00", 5)
	block.Date, block.Status = "2026-09-10", "active"
	state := minimalLiveState("")
	state.Accounts.Betreuer = []AccountCredentials{{StaffID: 17}}
	state.Activities = map[string]int64{"Hausaufgaben": 23}
	client := &retryLeftoverClient{
		demoDayClient: newDemoDayClient(block),
		failComplete:  true,
	}
	ticker, err := NewDemoTicker(DemoTickOptions{
		State: state, Client: client, Now: func() time.Time { return demoDayAt(t, "15:00") },
		Visits: func(context.Context) ([]DemoVisit, error) { return nil, nil },
	})
	require.NoError(t, err)

	require.NoError(t, ticker.closeLeftovers(demoDayAt(t, "15:00")))
	assert.Empty(t, ticker.settled, "a failed close must be retried")
	assert.Equal(t, "active", client.blocks[0].Status)

	require.NoError(t, ticker.closeLeftovers(demoDayAt(t, "15:01")))
	assert.Equal(t, "2026-09-11", ticker.settled)
	assert.Equal(t, "completed", client.blocks[0].Status)
}

func TestDemoTickerRetriesFailedDeviceSessionShutdown(t *testing.T) {
	t.Parallel()
	state := minimalLiveState("")
	state.Students = nil
	state.Accounts.Betreuer = []AccountCredentials{{StaffID: 17}}
	state.Activities = map[string]int64{"Hausaufgaben": 23}
	client := &retryDeviceSessionClient{
		demoPlanClient: &demoPlanClient{demoDayClient: *newDemoDayClient()},
		failEnd:        true,
	}
	now := demoDayAt(t, "15:00")
	ticker, err := NewDemoTicker(DemoTickOptions{
		State: state, Client: client, Now: func() time.Time { return now },
		Visits: func(context.Context) ([]DemoVisit, error) { return nil, nil },
	})
	require.NoError(t, err)

	_, err = ticker.syncDay(t.Context(), now, nil)
	require.Error(t, err)
	assert.False(t, ticker.day.devices, "a failed shutdown must be retried")

	_, err = ticker.syncDay(t.Context(), now.Add(5*time.Second), nil)
	require.NoError(t, err)
	assert.True(t, ticker.day.devices)
	assert.Equal(t, 2, client.endCalls)
}

func (c *demoPlanClient) DeviceGet(path, key, pin string) ([]byte, error) {
	if c.ended > 0 {
		return []byte(`{"data":{"is_active":false}}`), nil
	}
	return []byte(`{"data":{"is_active":true,"room_id":5}}`), nil
}

func (c *demoPlanClient) DevicePost(path string, body any, key, pin string) ([]byte, error) {
	if path == "/api/iot/session/end" {
		c.ended++
		return []byte(`{"data":{}}`), nil
	}
	return c.device.DevicePost(path, body, key, pin)
}

func TestDemoTickRunsAWeekdayWithoutKioskSessions(t *testing.T) {
	t.Parallel()
	state := minimalLiveState("")
	state.Accounts.Betreuer = []AccountCredentials{{StaffID: 17}}
	state.Activities = map[string]int64{"Hausaufgaben": 23}
	studentID := state.Students[0].ID
	client := &demoPlanClient{demoDayClient: *newDemoDayClient(plannedBlock(1, "14:45", "17:00", 5, studentID))}
	client.arrivals[studentID] = "11:45"
	now := demoDayAt(t, "15:00")
	ticker, err := NewDemoTicker(DemoTickOptions{
		State: state, Client: client, Now: func() time.Time { return now }, PlanWeekdays: true,
		Visits: func(context.Context) ([]DemoVisit, error) { return nil, nil },
	})
	require.NoError(t, err)

	require.NoError(t, ticker.Tick(t.Context()))
	assert.Equal(t, 1, client.ended, "the open simulation's kiosk session ends")
	assert.Equal(t, "active", client.blocks[0].Status)

	now = now.Add(5 * time.Second)
	require.NoError(t, ticker.Tick(t.Context()))
	assert.Zero(t, client.device.sessionsStarted, "a weekday runs on the planned blocks")
	assert.Contains(t, client.device.studentActions, "/api/iot/checkin", "the child checks into its block's room")
	assert.NotContains(t, client.device.studentActions, "/api/iot/attendance/toggle", "the kiosk has no session to confirm attendance in")
}

// demoWeekendClient records the kiosk side like demoRecordingClient and
// serves the past week's blocks like demoDayClient.
type demoWeekendClient struct {
	demoRecordingClient
	days *demoDayClient
}

func (c *demoWeekendClient) Get(path string) ([]byte, error) { return c.days.Get(path) }
func (c *demoWeekendClient) Post(path string, body any) ([]byte, error) {
	return c.days.Post(path, body)
}

func TestDemoTickKeepsKioskSessionsOnTheWeekend(t *testing.T) {
	t.Parallel()
	state := minimalLiveState("")
	state.Accounts.Betreuer = []AccountCredentials{{StaffID: 17}}
	state.Activities = map[string]int64{"Hausaufgaben": 23}
	// Friday's AG still runs: the daily close was missed.
	friday := plannedBlock(9, "19:05", "20:05", 5, 11)
	friday.Date, friday.Status = "2026-09-11", "active"
	client := &demoWeekendClient{days: newDemoDayClient(friday)}
	saturday := time.Date(2026, 9, 12, 20, 0, 0, 0, demoBerlin)
	ticker, err := NewDemoTicker(DemoTickOptions{
		State: state, Client: client, Now: func() time.Time { return saturday }, PlanWeekdays: true,
		Visits: func(context.Context) ([]DemoVisit, error) { return nil, nil },
	})
	require.NoError(t, err)
	require.NoError(t, ticker.Tick(t.Context()))
	assert.Equal(t, "completed", client.days.blocks[0].Status, "a block left running from an earlier day is closed")
	assert.Equal(t, 1, client.sessionsStarted, "weekends are never care days and keep the open simulation")

	client.days.writes = nil
	saturday = saturday.Add(5 * time.Second)
	require.NoError(t, ticker.Tick(t.Context()))
	assert.Empty(t, client.days.writes, "the leftovers are closed once a day")
}

func TestDemoTickRunsAWeekendThatFollowsFridaysPlan(t *testing.T) {
	t.Parallel()
	state := minimalLiveState("")
	state.Accounts.Betreuer = []AccountCredentials{{StaffID: 17}}
	state.Activities = map[string]int64{"Hausaufgaben": 23}
	studentID := state.Students[0].ID
	saturdayBlock := plannedBlock(1, "14:45", "17:00", 5, studentID)
	saturdayBlock.Date = "2026-09-12"
	client := &demoPlanClient{demoDayClient: *newDemoDayClient(saturdayBlock)}
	client.arrivals[studentID] = "11:45"
	saturday := time.Date(2026, 9, 12, 15, 0, 0, 0, demoBerlin)
	ticker, err := NewDemoTicker(DemoTickOptions{
		State: state, Client: client, Now: func() time.Time { return saturday }, PlanWeekdays: true,
		Visits: func(context.Context) ([]DemoVisit, error) { return nil, nil },
	})
	require.NoError(t, err)
	require.NoError(t, ticker.Tick(t.Context()))
	assert.Equal(t, "active", client.blocks[0].Status, "the Saturday block from Friday's plan runs")
	assert.Zero(t, client.device.sessionsStarted, "no kiosk session on a planned weekend")
}
