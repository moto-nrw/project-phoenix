package simulate

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"
	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
)

// newClient builds a seed/api client for the simulation flows.
func newClient(baseURL string, verbose bool) *seedapi.Client {
	return seedapi.NewClientWithAdapter(phoenixapi.New(baseURL, verbose), verbose)
}

type Runtime struct {
	State         *seedapi.SeedState
	Client        *seedapi.Client
	Options       FullDayOptions
	RFIDTags      map[int64]string
	ActiveRoomIDs []int64
	DeviceKeys    []string
	ActivityNames []string
	Counts        RuntimeCounts
}

type RuntimeCounts struct {
	RFIDAssigned       int
	SessionsStarted    int
	AttendanceRecords  int
	StudentsCheckedIn  int
	StudentsSick       int
	StudentsCheckedOut int
	DailyCheckouts     int
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

func newRuntime(state *seedapi.SeedState, client *seedapi.Client, opts FullDayOptions) *Runtime {
	return &Runtime{
		State:    state,
		Client:   client,
		Options:  opts,
		RFIDTags: make(map[int64]string),
	}
}

func (r *Runtime) primaryDevice() (seedapi.SeedDevice, error) {
	if len(r.DeviceKeys) == 0 {
		return seedapi.SeedDevice{}, fmt.Errorf("no devices available")
	}
	return r.State.Devices[r.DeviceKeys[0]], nil
}
