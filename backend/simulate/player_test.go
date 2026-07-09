package simulate

import (
	"context"
	"errors"
	"testing"

	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// ActionError Tests
// =============================================================================

func TestActionError_Error(t *testing.T) {
	e := &ActionError{Action: "assign RFID tags", Err: errors.New("timeout")}
	assert.Equal(t, "assign RFID tags: timeout", e.Error())
}

func TestActionError_Error_Nil(t *testing.T) {
	var e *ActionError
	assert.Equal(t, "", e.Error())
}

func TestActionError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	e := &ActionError{Action: "test", Err: inner}
	assert.Equal(t, inner, e.Unwrap())
}

func TestActionError_Unwrap_Nil(t *testing.T) {
	var e *ActionError
	assert.Nil(t, e.Unwrap())
}

func TestActionError_Unwrap_NilErr(t *testing.T) {
	e := &ActionError{Action: "test", Err: nil}
	assert.Nil(t, e.Unwrap())
}

func TestActionError_ErrorsAs(t *testing.T) {
	inner := errors.New("db connection failed")
	e := &ActionError{Action: "login", Err: inner}

	var ae *ActionError
	require.ErrorAs(t, e, &ae)
	assert.Equal(t, "login", ae.Action)
}

// =============================================================================
// Scenario Tests
// =============================================================================

type mockAction struct {
	name string
	err  error
	ran  bool
}

func (m *mockAction) Name() string { return m.name }
func (m *mockAction) Run(_ context.Context, _ *Runtime) error {
	m.ran = true
	return m.err
}

func TestScenario_Run_AllSuccess(t *testing.T) {
	a1 := &mockAction{name: "step1"}
	a2 := &mockAction{name: "step2"}
	a3 := &mockAction{name: "step3"}

	s := Scenario{Name: "test", Actions: []Action{a1, a2, a3}}
	err := s.Run(context.Background(), &Runtime{})
	assert.NoError(t, err)
	assert.True(t, a1.ran)
	assert.True(t, a2.ran)
	assert.True(t, a3.ran)
}

func TestScenario_Run_StopsOnError(t *testing.T) {
	a1 := &mockAction{name: "step1"}
	a2 := &mockAction{name: "step2", err: errors.New("fail")}
	a3 := &mockAction{name: "step3"}

	s := Scenario{Name: "test", Actions: []Action{a1, a2, a3}}
	err := s.Run(context.Background(), &Runtime{})
	require.Error(t, err)

	var ae *ActionError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "step2", ae.Action)
	assert.True(t, a1.ran)
	assert.True(t, a2.ran)
	assert.False(t, a3.ran) // should NOT have run
}

func TestScenario_Run_Empty(t *testing.T) {
	s := Scenario{Name: "empty", Actions: nil}
	err := s.Run(context.Background(), &Runtime{})
	assert.NoError(t, err)
}

// =============================================================================
// newRuntime Tests
// =============================================================================

func TestNewRuntime(t *testing.T) {
	state := &seedapi.SeedState{BaseURL: "http://localhost:8080"}
	client := newClient("http://localhost:8080", false)
	opts := FullDayOptions{StatePath: "state.json", Close: true, Verbose: true}

	rt := newRuntime(state, client, opts)
	assert.Same(t, state, rt.State)
	assert.Same(t, client, rt.Client)
	assert.True(t, rt.Options.Close)
	assert.True(t, rt.Options.Verbose)
	assert.NotNil(t, rt.RFIDTags)
	assert.Empty(t, rt.RFIDTags)
	assert.Nil(t, rt.ActiveRoomIDs)
	assert.Nil(t, rt.DeviceKeys)
}

// =============================================================================
// Runtime.primaryDevice Tests
// =============================================================================

func TestRuntime_PrimaryDevice_Success(t *testing.T) {
	state := &seedapi.SeedState{
		Devices: map[string]seedapi.SeedDevice{
			"d1": {APIKey: "k1", Name: "Scanner 1"},
			"d2": {APIKey: "k2", Name: "Scanner 2"},
		},
	}
	rt := &Runtime{
		State:      state,
		DeviceKeys: []string{"d1", "d2"},
	}

	dev, err := rt.primaryDevice()
	require.NoError(t, err)
	assert.Equal(t, "k1", dev.APIKey)
	assert.Equal(t, "Scanner 1", dev.Name)
}

func TestRuntime_PrimaryDevice_NoDevices(t *testing.T) {
	rt := &Runtime{
		State:      &seedapi.SeedState{},
		DeviceKeys: []string{},
	}

	_, err := rt.primaryDevice()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no devices available")
}

func TestRuntime_PrimaryDevice_NilDeviceKeys(t *testing.T) {
	rt := &Runtime{
		State:      &seedapi.SeedState{},
		DeviceKeys: nil,
	}

	_, err := rt.primaryDevice()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no devices available")
}

// =============================================================================
// RuntimeCounts Tests
// =============================================================================

func TestRuntimeCounts_Defaults(t *testing.T) {
	c := RuntimeCounts{}
	assert.Equal(t, 0, c.RFIDAssigned)
	assert.Equal(t, 0, c.SessionsStarted)
	assert.Equal(t, 0, c.AttendanceRecords)
	assert.Equal(t, 0, c.StudentsCheckedIn)
	assert.Equal(t, 0, c.StudentsSick)
	assert.Equal(t, 0, c.StudentsCheckedOut)
	assert.Equal(t, 0, c.DailyCheckouts)
	assert.Equal(t, 0, c.FeedbackSubmitted)
	assert.Equal(t, 0, c.SessionsEnded)
}
