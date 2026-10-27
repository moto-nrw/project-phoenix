package simulate

import (
	"context"
	"fmt"
	"time"
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
	ActiveSessionDeviceKeys []string
	DeviceKeys              []string
	// OpenRoomBlockInstanceID is the block started in a released room (#3281),
	// 0 when none runs.
	OpenRoomBlockInstanceID int64
	// SupervisorStaffIDs supervise the running kiosk sessions.
	SupervisorStaffIDs []int64
	// Today is what the school plans for the simulated day (fullday_today.go).
	Today todayPlan
	// Blocks are the planned blocks the run started, in start order.
	Blocks []startedBlock
	// Children are the children of the day: the seed state's, then the
	// children only the API lists (enrollment admissions), with today's
	// status and arrival.
	Children []todayChild
	// Rooms tracks where the children of the day were checked in.
	Rooms roomPlan
	// Arrivals is when the children come and what they do at midday.
	Arrivals arrivalPlan
	// messages remembers what the run wrote (fullday_messages.go).
	messages messageLog
	// Present holds the children whose attendance is recorded today.
	Present map[int64]bool
	// ClockedInStaff holds the staff members the run stamped in or found on
	// leave, so a later shift check leaves them alone.
	ClockedInStaff map[int64]bool
	// StaffOnLeave remembers, per checked staff member, whether an approved
	// absence covers today.
	StaffOnLeave map[int64]bool
	// staffLogins are the signed-in clients of the caregivers who run the
	// started blocks (fullday_blocks.go).
	staffLogins map[int64]staffLogin
	// adminLoginAt is when the admin last signed in, on the shared clock.
	adminLoginAt time.Time
	Counts       RuntimeCounts
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
	// IndependentStays counts children moved into a released room without
	// joining an activity there (#3066).
	IndependentStays int
	// OpenRoomBlockChildren counts children checked into the block running in
	// a released room (#3281).
	OpenRoomBlockChildren int
	// EarlyCareChildren counts the children of the Frühbetreuung.
	EarlyCareChildren int
	// EarlyPickups counts the children sent home at midday.
	EarlyPickups int
	// StaffClockedIn and StaffAbsent count the shift clock-ins and the staff
	// members skipped for approved leave.
	StaffClockedIn int
	StaffAbsent    int
	// BlocksStarted counts the planned blocks the run started.
	BlocksStarted int
	// Messages counts the team-chat and parent messages the run wrote.
	Messages int
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
		State:          state,
		Client:         client,
		Options:        opts,
		RFIDTags:       make(map[int64]string),
		Present:        make(map[int64]bool),
		ClockedInStaff: make(map[int64]bool),
		StaffOnLeave:   make(map[int64]bool),
		staffLogins:    make(map[int64]staffLogin),
		Rooms:          newRoomPlan(),
		messages:       messageLog{threads: map[int]parentThread{}},
	}
}

func (r *Runtime) primaryDevice() (SeedDevice, error) {
	if len(r.DeviceKeys) == 0 {
		return SeedDevice{}, fmt.Errorf("no devices available")
	}
	return r.State.Devices[r.DeviceKeys[0]], nil
}
