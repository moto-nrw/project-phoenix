package simulate

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type demoCapacityClient struct {
	demoRecordingClient
	conflicts int
}

type demoCapacityError struct{}

func (demoCapacityError) Error() string         { return "room is full" }
func (demoCapacityError) HTTPErrorCode() string { return "ROOM_CAPACITY_EXCEEDED" }

func (c *demoCapacityClient) DevicePost(path string, body any, key, pin string) ([]byte, error) {
	if path == "/api/iot/checkin" {
		c.conflicts++
		return nil, demoCapacityError{}
	}
	return c.demoRecordingClient.DevicePost(path, body, key, pin)
}

func TestDemoTickSkipsFullRoomsWithoutFailingTheRunner(t *testing.T) {
	t.Parallel()
	state := minimalLiveState("")
	state.Accounts.Betreuer = []AccountCredentials{{StaffID: 17}}
	state.Activities = map[string]int64{"Hausaufgaben": 23}
	client := &demoCapacityClient{}
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	ticker, err := NewDemoTicker(DemoTickOptions{
		State: state, Client: client, Now: func() time.Time { return now },
		Visits: func(context.Context) ([]DemoVisit, error) {
			return []DemoVisit{{StudentID: state.Students[0].ID, Active: true}}, nil
		},
	})
	require.NoError(t, err)
	for range 100 {
		now = now.Add(time.Minute)
		require.NoError(t, ticker.Tick(t.Context()))
	}
	assert.Positive(t, client.conflicts, "exercise the capacity conflict path")
}

func TestDemoTickProtectsWebVisitUntilFifteenMinutes(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 20, 23, 10, 0, 0, time.UTC)
	booked := now.Add(-14 * time.Minute)
	state := minimalLiveState("")
	state.Accounts.Betreuer = []AccountCredentials{{StaffID: 17}}
	state.Activities = map[string]int64{"Hausaufgaben": 23}
	client := &demoRecordingClient{}
	ticker, err := NewDemoTicker(DemoTickOptions{
		State: state, Client: client, Now: func() time.Time { return now },
		Visits: func(context.Context) ([]DemoVisit, error) {
			return []DemoVisit{{StudentID: state.Students[0].ID, Web: true, ChangedAt: booked}}, nil
		},
	})
	require.NoError(t, err)
	require.NoError(t, ticker.Tick(t.Context()))
	assert.Empty(t, client.studentActions, "a web checkout must not be immediately undone by rebuilding")
	now = now.Add(time.Minute)
	require.NoError(t, ticker.Tick(t.Context()))
	assert.Contains(t, client.studentActions, "/api/iot/checkin", "eligible at exactly fifteen minutes")
}

func TestDemoTickRebuildsEmptyDayOnWeekend(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 20, 22, 0, 0, 0, time.UTC)
	state := minimalLiveState("")
	state.Accounts.Betreuer = []AccountCredentials{{StaffID: 17}}
	state.Activities = map[string]int64{"Hausaufgaben": 23}
	client := &demoRecordingClient{}
	ticker, err := NewDemoTicker(DemoTickOptions{
		State: state, Client: client, Now: func() time.Time { return now },
		Visits: func(context.Context) ([]DemoVisit, error) { return nil, nil },
	})
	require.NoError(t, err)
	require.NoError(t, ticker.Tick(t.Context()))
	assert.Contains(t, client.studentActions, "/api/iot/attendance/toggle")
	assert.Contains(t, client.studentActions, "/api/iot/checkin")
	assert.Equal(t, 1, client.sessionsStarted)
	client.studentActions = nil
	require.NoError(t, ticker.Tick(t.Context()))
	assert.Contains(t, client.studentActions, "/api/iot/checkin")
	assert.Equal(t, 1, client.sessionsStarted, "reuse an existing device session after a pause")
}

func TestDemoRebuildPreparesChildrenBeyondInitialCheckinLimit(t *testing.T) {
	t.Parallel()
	state := minimalLiveState("")
	state.Students = nil
	for id := int64(1); id <= 85; id++ {
		state.Students = append(state.Students, SeedStudent{ID: id})
	}
	state.Accounts.Betreuer = []AccountCredentials{{StaffID: 17}}
	state.Activities = map[string]int64{"Hausaufgaben": 23}
	client := &demoRecordingClient{}
	ticker, err := NewDemoTicker(DemoTickOptions{
		State: state, Client: client, Now: time.Now,
		Visits: func(context.Context) ([]DemoVisit, error) { return nil, nil },
	})
	require.NoError(t, err)
	require.NoError(t, ticker.Tick(t.Context()))
	assert.Contains(t, client.studentActions, "/api/students/85/rfid", "later attendance ticks may select children not initially checked in")
	checkins := 0
	for _, path := range client.studentActions {
		if path == "/api/iot/checkin" {
			checkins++
		}
	}
	assert.Equal(t, 84, checkins, "retain the initial occupancy limit")
}

type demoRecordingClient struct {
	Client
	studentActions  []string
	sessionsStarted int
	studentRFIDs    []string
}

func (c *demoRecordingClient) DeviceGet(string, string, string) ([]byte, error) {
	if c.sessionsStarted > 0 {
		return []byte(`{"data":{"is_active":true,"active_group_id":31,"room_id":10}}`), nil
	}
	return []byte(`{"data":{"is_active":false}}`), nil
}
func (c *demoRecordingClient) DevicePost(path string, body any, _, _ string) ([]byte, error) {
	switch path {
	case "/api/iot/session/start":
		c.sessionsStarted++
	case "/api/iot/ping":
	default:
		c.studentActions = append(c.studentActions, path)
	}
	if value, ok := body.(map[string]any); ok {
		if tag, ok := value["student_rfid"].(string); ok {
			c.studentRFIDs = append(c.studentRFIDs, tag)
		}
	}
	if value, ok := body.(map[string]string); ok {
		if tag := value["rfid"]; tag != "" {
			c.studentRFIDs = append(c.studentRFIDs, tag)
		}
	}
	return []byte(`{"data":{}}`), nil
}

func (c *demoRecordingClient) Put(path string, _ any) ([]byte, error) {
	c.studentActions = append(c.studentActions, path)
	return []byte(`{"data":{}}`), nil
}
func (c *demoRecordingClient) DevicePut(string, any, string, string) ([]byte, error) {
	return []byte(`{"data":{}}`), nil
}

func TestDemoTickKeepsWebChildUntouchedWhileOtherChildrenMove(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 20, 22, 0, 0, 0, time.UTC)
	state := minimalLiveState("")
	state.Students = append(state.Students, SeedStudent{ID: 7})
	state.Accounts.Betreuer = []AccountCredentials{{StaffID: 17}}
	state.Activities = map[string]int64{"Hausaufgaben": 23}
	client := &demoRecordingClient{}
	ticker, err := NewDemoTicker(DemoTickOptions{
		State: state, Client: client, Now: func() time.Time { return now },
		Visits: func(context.Context) ([]DemoVisit, error) {
			return []DemoVisit{
				{StudentID: state.Students[0].ID, Active: true, Web: true, ChangedAt: now.Add(-time.Minute)},
				{StudentID: state.Students[1].ID, Active: true},
			}, nil
		},
	})
	require.NoError(t, err)
	for range 100 {
		require.NoError(t, ticker.Tick(t.Context()))
		now = now.Add(5 * time.Second)
	}
	assert.NotEmpty(t, client.studentRFIDs)
	for _, tag := range client.studentRFIDs {
		assert.Equal(t, "DE000007", tag)
	}
	assert.NotContains(t, client.studentActions, "/api/students/1", "sick toggles must respect the grace period too")
}

func TestDemoTickDoesNotGuessWhenVisitReadFails(t *testing.T) {
	t.Parallel()
	state := minimalLiveState("")
	state.Accounts.Betreuer = []AccountCredentials{{StaffID: 17}}
	state.Activities = map[string]int64{"Hausaufgaben": 23}
	client := &demoRecordingClient{}
	ticker, err := NewDemoTicker(DemoTickOptions{
		State: state, Client: client, Now: time.Now,
		Visits: func(context.Context) ([]DemoVisit, error) { return nil, fmt.Errorf("presence unavailable") },
	})
	require.NoError(t, err)
	require.ErrorContains(t, ticker.Tick(t.Context()), "presence unavailable")
	assert.Empty(t, client.studentActions)
	assert.Zero(t, client.sessionsStarted)
}
