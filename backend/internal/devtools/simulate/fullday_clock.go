package simulate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ClockSetter moves the clock that the simulation and the server share to the
// given instant. The server stamps every check-in, session start and work
// block with its own clock, so a staggered school day needs that clock moved
// between the waves of the day.
type ClockSetter func(context.Context, time.Time) error

// CommandClock runs command with the target instant appended as Unix seconds,
// e.g. a CI script that calls `date -s`. An empty command moves no clock.
func CommandClock(command string) ClockSetter {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil
	}
	return func(ctx context.Context, at time.Time) error {
		cmd := exec.CommandContext(ctx, fields[0], append(fields[1:], strconv.FormatInt(at.Unix(), 10))...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("clock command %q: %w", command, err)
		}
		return nil
	}
}

// wallClock is a time of day on the simulated day, in minutes after Berlin
// midnight.
type wallClock int

func at(hour, minute int) wallClock { return wallClock(hour*60 + minute) }

func (c wallClock) String() string { return fmt.Sprintf("%02d:%02d", int(c)/60, int(c)%60) }

// The schedule of the simulated day. With a clock setter each wave runs at its
// time; without one (a local run) every wave runs at once, in this order. The
// planned blocks and shifts are read from today's plan; these are the times
// the simulation acts at.
var (
	earlyShiftCheck   = at(7, 25)  // staff of the Frühdienst stamp in
	earlyCareStart    = at(7, 30)  // the planned Frühbetreuung starts
	earlyTeamNote     = at(7, 45)  // a colleague of the Frühdienst writes in the Team-Chat
	earlyCareEnd      = at(7, 58)  // the early children leave for their lessons
	morningShiftCheck = at(7, 58)  // staff whose Dienst starts at 08:00 stamp in
	lateMorningCheck  = at(8, 25)  // staff whose Dienst starts at 08:30 stamp in
	sickCallsAt       = at(8, 30)  // parents report sick children and write
	lateShiftCheck    = at(11, 31) // the late shift stamps in and writes
	sessionsStart     = at(12, 0)  // planned blocks and kiosk sessions start together
	middayMovesAt     = at(12, 40) // two children leave their room, released room stays
	earlyPickupsAt    = at(12, 45) // a few first graders go home after lunch
	firstParentReply  = at(12, 46) // the OGS answers the first parent
	afternoonNotesAt  = at(12, 50) // parents write, the OGS answers the morning question
)

// arrivalWave is one group of classes whose lessons end at the same time.
type arrivalWave struct {
	start  wallClock
	grades []int
}

// arrivalWaves stagger the children by the end of their lessons. A child whose
// class names no grade arrives with the last wave.
var arrivalWaves = []arrivalWave{
	{start: at(12, 0), grades: []int{1, 2}},
	{start: at(12, 15), grades: []int{3}},
	{start: at(12, 35), grades: []int{4}},
}

// arrivalsPerMinute is how many children of one wave arrive in the same
// minute.
const arrivalsPerMinute = 6

// adminTokenReuse is how long an admin login is reused after the clock moved.
// The access token lives 15 minutes; the simulation signs in again before.
const adminTokenReuse = 10 * time.Minute

var simulationBerlin = mustLoadBerlin()

func mustLoadBerlin() *time.Location {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(fmt.Sprintf("load Europe/Berlin: %v", err))
	}
	return loc
}

// advanceClock moves the shared clock forward to the given time of today. It
// never moves the clock back: a wave whose time has passed runs at once. After
// a move the admin signs in again, since the access token may have expired.
func (rt *Runtime) advanceClock(ctx context.Context, to wallClock) error {
	if rt.Options.SetClock == nil {
		return nil
	}
	now := rt.Options.now().In(simulationBerlin)
	target := time.Date(now.Year(), now.Month(), now.Day(), int(to)/60, int(to)%60, 0, 0, simulationBerlin)
	if !target.After(now) {
		return nil
	}
	if err := rt.Options.SetClock(ctx, target); err != nil {
		return fmt.Errorf("move the clock to %s: %w", to, err)
	}
	if moved := rt.Options.now(); moved.Before(target.Add(-time.Minute)) {
		return fmt.Errorf("move the clock to %s: the clock still reads %s", to, moved.In(simulationBerlin).Format("15:04"))
	}
	fmt.Printf("  Clock: %s\n", to)
	if rt.Options.now().Sub(rt.adminLoginAt) < adminTokenReuse {
		return nil
	}
	return rt.reloginAdmin()
}

// reloginAdmin signs the admin in on a new client, so an expired access token
// is never reused.
func (rt *Runtime) reloginAdmin() error {
	client, err := buildClient(rt.Options.Client, rt.State.BaseURL, rt.Options.Verbose)
	if err != nil {
		return err
	}
	admin := rt.State.Accounts.Admin[0]
	if err := client.Login(admin.Email, admin.Password, rt.State.Bootstrap.TenantSlug); err != nil {
		return fmt.Errorf("admin login after clock move: %w", err)
	}
	rt.Client = client
	rt.adminLoginAt = rt.Options.now()
	return nil
}

// clockStep moves the clock to a time of the simulated day; the next action
// runs then.
type clockStep struct{ at wallClock }

func (c clockStep) Name() string { return fmt.Sprintf("move the clock to %s", c.at) }

func (c clockStep) Run(ctx context.Context, rt *Runtime) error { return rt.advanceClock(ctx, c.at) }

// classGrade reads the grade from a class name such as "Klasse 3b"; 0 when the
// name holds none.
func classGrade(class string) int {
	for _, r := range class {
		if r >= '1' && r <= '9' {
			return int(r - '0')
		}
	}
	return 0
}

// arrivalWaveIndex returns the wave a class arrives with.
func arrivalWaveIndex(class string) int {
	grade := classGrade(class)
	for i, wave := range arrivalWaves {
		for _, g := range wave.grades {
			if g == grade {
				return i
			}
		}
	}
	return len(arrivalWaves) - 1
}

func fullDayScenario(close bool) Scenario {
	actions := []Action{
		verifyServerAction{},
		loginAdminAction{},
		assignRFIDsAction{},
		loadTodayAction{},
		clockStep{earlyShiftCheck}, staffClockInAction{},
		clockStep{earlyCareStart}, earlyCareAction{},
		clockStep{earlyTeamNote}, teamChatAction{earlyTeamNote},
		clockStep{earlyCareEnd}, earlyCareEndAction{},
		clockStep{morningShiftCheck}, staffClockInAction{},
		clockStep{lateMorningCheck}, staffClockInAction{},
		clockStep{sickCallsAt}, sickCallsAction{}, parentQuestionAction{},
		arrivalsAction{name: "early arrivals", until: lateShiftCheck},
		clockStep{lateShiftCheck}, staffClockInAction{}, teamChatAction{lateShiftCheck},
		clockStep{sessionsStart}, startMiddayAction{},
		seedStaffFeedTombstoneAction{},
		arrivalsAction{name: "record attendance and checkins", until: middayMovesAt, unregisteredOK: true},
		clockStep{middayMovesAt}, teamChatAction{middayMovesAt}, middayActivityAction{},
		independentRoomStaysAction{},
		openRoomBlockAction{},
		kioskOpenRoomStayAction{},
		clockStep{earlyPickupsAt}, earlyPickupsAction{},
		clockStep{firstParentReply}, firstParentReplyAction{},
		clockStep{afternoonNotesAt}, afternoonMessagesAction{},
	}
	if close {
		actions = append(actions, endOfDayAction{})
	}
	actions = append(actions, printSummaryAction{})
	return Scenario{
		Name:    "full-day",
		Actions: actions,
	}
}
