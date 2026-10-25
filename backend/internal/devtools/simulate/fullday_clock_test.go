package simulate

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClock is the clock the simulation and the mock server share.
type fakeClock struct {
	mu    sync.Mutex
	now   time.Time
	moves []time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Set(_ context.Context, to time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = to
	c.moves = append(c.moves, to)
	return nil
}

func (c *fakeClock) Moves() []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Time(nil), c.moves...)
}

// stampedCall is one request with the shared clock's time when it arrived and
// the login (the e-mail address in the mock token) or device that sent it.
type stampedCall struct {
	Method, Path, Clock, Caller string
	Body                        map[string]any
}

func berlinTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04", value, simulationBerlin)
	require.NoError(t, err)
	return parsed
}

func callsTo(calls []stampedCall, path string) []stampedCall {
	var matched []stampedCall
	for _, call := range calls {
		if call.Path == path {
			matched = append(matched, call)
		}
	}
	return matched
}

func TestAdvanceClock_NeverMovesBack(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: berlinTime(t, "2026-10-20 12:20")}
	srv, _ := clockedDayServer(t, clock)
	rt := newRuntime(clockedDayState(srv.URL), newClient(srv.URL, false), FullDayOptions{
		Client: newTestClientFactory, Now: clock.Now, SetClock: clock.Set,
	})
	rt.adminLoginAt = clock.Now()

	require.NoError(t, rt.advanceClock(context.Background(), at(12, 0)))
	assert.Empty(t, clock.Moves(), "a wave whose time has passed runs at once")

	require.NoError(t, rt.advanceClock(context.Background(), at(12, 25)))
	assert.Equal(t, []time.Time{berlinTime(t, "2026-10-20 12:25")}, clock.Moves())
}

func TestAdvanceClock_FailsWhenTheClockDoesNotMove(t *testing.T) {
	t.Parallel()

	start := berlinTime(t, "2026-10-20 07:30")
	rt := newRuntime(&SeedState{}, nil, FullDayOptions{
		Now:      func() time.Time { return start },
		SetClock: func(context.Context, time.Time) error { return nil },
	})

	err := rt.advanceClock(context.Background(), at(8, 0))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the clock still reads 07:30")
}

func TestAdvanceClock_WithoutSetterRunsAtOnce(t *testing.T) {
	t.Parallel()

	rt := newRuntime(&SeedState{}, nil, FullDayOptions{})
	require.NoError(t, rt.advanceClock(context.Background(), at(23, 59)))
}

func TestCommandClock_PassesTheTargetAsUnixSeconds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := filepath.Join(dir, "set-clock")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s %s' \"$1\" \"$2\" > \""+dir+"/args\"\n"), 0o700))

	setter := CommandClock(script + " --berlin")
	require.NotNil(t, setter)
	require.NoError(t, setter(context.Background(), time.Unix(1792491600, 0)))

	args, err := os.ReadFile(filepath.Join(dir, "args"))
	require.NoError(t, err)
	assert.Equal(t, "--berlin 1792491600", string(args))

	assert.Nil(t, CommandClock("  "), "no command moves no clock")
	assert.Error(t, CommandClock(filepath.Join(dir, "missing"))(context.Background(), time.Unix(0, 0)))
}

func TestShiftDue_StampsInAtMostFiveMinutesEarly(t *testing.T) {
	t.Parallel()

	now := berlinTime(t, "2026-10-20 07:58")
	assert.True(t, shiftDue(plannedShift{StartTime: "08:00:00"}, now))
	assert.True(t, shiftDue(plannedShift{StartTime: "07:30"}, now))
	assert.False(t, shiftDue(plannedShift{StartTime: "08:04"}, now))
	assert.False(t, shiftDue(plannedShift{StartTime: "11:30:00"}, now))
	assert.False(t, shiftDue(plannedShift{StartTime: ""}, now))
}
