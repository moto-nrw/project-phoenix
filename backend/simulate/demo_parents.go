package simulate

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/demoprofile"
)

// Parent ticks of the public demo (#3468). Other parents of the demo school
// ask for a pickup change and write a message, taking turns, so „Offene
// Anfragen" in the OGS app is never empty.
const (
	// demoParentActionInterval spaces the parents' actions: a visitor sees
	// new requests arrive without the school drowning in them.
	demoParentActionInterval = 3 * time.Minute
	// demoParentTurn is how long one parent acts before the next one takes
	// over. One login serves the whole turn, like the admin's login.
	demoParentTurn = 10 * time.Minute
)

// DemoParent is a parent of a demo school other than the visitor, with the
// child the parent writes about.
type DemoParent struct {
	Email     string
	Password  string
	StudentID int64
}

// DemoParentClient is the parents portal side of the simulation. It holds the
// login of one parent.
type DemoParentClient interface {
	LoginParent(email, password string) error
	Post(path string, body any) ([]byte, error)
}

// OtherDemoParents returns the seeded parents the simulation acts as: every
// parent with a child except the visitor's own parent account and the parents
// who share its child. The visitor writes as that parent, and a co-parent's
// message would appear in the visitor's own parent app.
func OtherDemoParents(parents []demoprofile.ParentCredentials, visitorAccountID int64) []DemoParent {
	var visitorChildren []int64
	for _, parent := range parents {
		if visitorAccountID != 0 && parent.AccountID == visitorAccountID {
			visitorChildren = append(visitorChildren, parent.StudentIDs...)
		}
	}
	var others []DemoParent
	for _, parent := range parents {
		if len(parent.StudentIDs) == 0 || (visitorAccountID != 0 && parent.AccountID == visitorAccountID) {
			continue
		}
		if slices.ContainsFunc(parent.StudentIDs, func(id int64) bool { return slices.Contains(visitorChildren, id) }) {
			continue
		}
		others = append(others, DemoParent{Email: parent.Email, Password: parent.Password, StudentID: parent.StudentIDs[0]})
	}
	return others
}

// demoParentState is where the parents' turns stand.
type demoParentState struct {
	client     DemoParentClient
	turn       int
	turnFrom   time.Time
	loggedIn   bool
	nextAction time.Time
	actions    int
}

var (
	demoPickupTimes   = []string{"14:30", "15:00", "13:45", "15:30"}
	demoPickupReasons = []string{
		"Wir haben nachmittags einen Arzttermin.",
		"Die Oma holt heute früher ab.",
		"Wir fahren direkt zum Schwimmkurs.",
		"Geburtstagsfeier bei einem Freund.",
	}
	demoParentMessages = []string{
		"Hat mein Kind heute gut gegessen?",
		"Die Sportsachen liegen noch bei Ihnen. Kann mein Kind sie morgen mitnehmen?",
		"Vielen Dank für den schönen Ausflug gestern!",
		"Mein Kind war gestern sehr müde. Ist etwas vorgefallen?",
	}
)

// parentTick lets the parent of the current turn act when the interval has
// passed. A request the school refuses (a feature switched off, a day that
// already has a change) is skipped; the next action follows as usual.
func (d *DemoTicker) parentTick(now time.Time) error {
	parents := d.options.Parents
	if len(parents) == 0 || d.options.ParentClient == nil || now.Before(d.parents.nextAction) {
		return nil
	}
	state := &d.parents
	if state.client == nil {
		state.client = d.options.ParentClient()
		state.turn = -1
	}
	if state.turn < 0 || !now.Before(state.turnFrom.Add(demoParentTurn)) {
		state.turn = (state.turn + 1) % len(parents)
		state.turnFrom, state.loggedIn = now, false
	}
	parent := parents[state.turn]
	if !state.loggedIn {
		if err := state.client.LoginParent(parent.Email, parent.Password); err != nil {
			return fmt.Errorf("demo parent login: %w", err)
		}
		state.loggedIn = true
	}
	path, body, err := demoParentAction(parent, state.actions, now)
	if err != nil {
		return err
	}
	if _, err := state.client.Post(path, body); err != nil && !isRefusedByTheSchool(err) {
		state.loggedIn = false
		return fmt.Errorf("demo parent action: %w", err)
	}
	state.actions++
	state.nextAction = now.Add(demoParentActionInterval)
	return nil
}

// demoParentAction alternates a pickup change and a message.
func demoParentAction(parent DemoParent, action int, now time.Time) (string, map[string]any, error) {
	pick := action / 2
	if action%2 == 1 {
		return fmt.Sprintf("/parent/me/messages/children/%d", parent.StudentID), map[string]any{
			"body": demoParentMessages[pick%len(demoParentMessages)],
		}, nil
	}
	date, err := demoPickupDate(now, pick)
	if err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("/parent/me/children/%d/care-exception", parent.StudentID), map[string]any{
		"date":        date,
		"pickup_time": demoPickupTimes[pick%len(demoPickupTimes)],
		"reason":      demoPickupReasons[pick%len(demoPickupReasons)],
	}, nil
}

// demoPickupDate is a school day in Berlin from the day after tomorrow on,
// as YYYY-MM-DD; later requests move further ahead, so they rarely hit a
// day already changed.
func demoPickupDate(now time.Time, pick int) (string, error) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return "", fmt.Errorf("load Berlin time zone: %w", err)
	}
	local := now.In(berlin)
	// Noon keeps the day stable across a change to or from summer time.
	day := time.Date(local.Year(), local.Month(), local.Day(), 12, 0, 0, 0, berlin).AddDate(0, 0, 2+pick%10)
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDate(0, 0, 1)
	}
	return day.Format("2006-01-02"), nil
}

func isRefusedByTheSchool(err error) bool {
	var coded interface{ HTTPStatusCode() int }
	if !errors.As(err, &coded) {
		return false
	}
	status := coded.HTTPStatusCode()
	return status >= 400 && status < 500 && status != 401
}
