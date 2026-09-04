package simulate

import (
	"context"
	"fmt"
)

type Client interface {
	CheckHealth() error
	Login(email, password string, tenantSlug ...string) error
	Get(path string) ([]byte, error)
	Post(path string, body any) ([]byte, error)
	Put(path string, body any) ([]byte, error)
	Delete(path string) ([]byte, error)
	DeviceGet(path, apiKey, pin string) ([]byte, error)
	DevicePost(path string, body any, apiKey, pin string) ([]byte, error)
	DevicePut(path string, body any, apiKey, pin string) ([]byte, error)
}

type ClientFactory func(baseURL string, verbose bool) (Client, error)

func buildClient(factory ClientFactory, baseURL string, verbose bool) (Client, error) {
	if factory == nil {
		return nil, fmt.Errorf("simulation client dependency is required")
	}
	client, err := factory(baseURL, verbose)
	if err != nil {
		return nil, fmt.Errorf("build simulation client: %w", err)
	}
	if client == nil {
		return nil, fmt.Errorf("simulation client dependency returned nil")
	}
	return client, nil
}

type Runtime struct {
	State                   *SeedState
	Client                  Client
	Options                 FullDayOptions
	RFIDTags                map[int64]string
	ActiveRoomIDs           []int64
	ActiveSessionDeviceKeys []string
	DeviceKeys              []string
	ActivityNames           []string
	Counts                  RuntimeCounts
}

type RuntimeCounts struct {
	RFIDAssigned       int
	SessionsStarted    int
	AttendanceRecords  int
	StudentsCheckedIn  int
	StudentsSick       int
	StudentsCheckedOut int
	DailyCheckouts     int
	FeedbackSubmitted  int
	SessionsEnded      int
}

type Action interface {
	Name() string
	Run(context.Context, *Runtime) error
}

type Scenario struct {
	Name    string
	Actions []Action
}

type ActionError struct {
	Action string
	Err    error
}

func (e *ActionError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %v", e.Action, e.Err)
}

func (e *ActionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (s Scenario) Run(ctx context.Context, rt *Runtime) error {
	for _, action := range s.Actions {
		if err := action.Run(ctx, rt); err != nil {
			return &ActionError{
				Action: action.Name(),
				Err:    err,
			}
		}
	}
	return nil
}

func newRuntime(state *SeedState, client Client, opts FullDayOptions) *Runtime {
	return &Runtime{
		State:    state,
		Client:   client,
		Options:  opts,
		RFIDTags: make(map[int64]string),
	}
}

func (r *Runtime) primaryDevice() (SeedDevice, error) {
	if len(r.DeviceKeys) == 0 {
		return SeedDevice{}, fmt.Errorf("no devices available")
	}
	return r.State.Devices[r.DeviceKeys[0]], nil
}
