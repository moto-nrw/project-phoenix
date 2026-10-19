package simulate

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/demoprofile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Parent ticks of the public demo (#3468): other parents of the demo school
// ask for a pickup change and write a message, so the OGS app always has
// open requests. Driven at the tick with an injected clock and a recorded
// parent client.

type demoParentPost struct {
	parent string
	path   string
	body   map[string]any
}

type demoParentRecorder struct {
	logins []string
	posts  []demoParentPost
	// reject answers every post to the path with the status.
	reject map[string]int
	// loginErr fails every parent login; loginAttempts counts them all.
	loginErr      error
	loginAttempts int
}

type demoParentHTTPError struct{ status int }

func (e demoParentHTTPError) Error() string       { return fmt.Sprintf("status %d", e.status) }
func (e demoParentHTTPError) HTTPStatusCode() int { return e.status }

func (r *demoParentRecorder) client() DemoParentClient {
	return &demoParentRecorderClient{recorder: r}
}

type demoParentRecorderClient struct {
	recorder *demoParentRecorder
	email    string
}

func (c *demoParentRecorderClient) LoginParent(email, _ string) error {
	c.recorder.loginAttempts++
	if c.recorder.loginErr != nil {
		return c.recorder.loginErr
	}
	c.email = email
	c.recorder.logins = append(c.recorder.logins, email)
	return nil
}

func (c *demoParentRecorderClient) Post(path string, body any) ([]byte, error) {
	for prefix, status := range c.recorder.reject {
		if strings.HasSuffix(path, prefix) {
			return nil, demoParentHTTPError{status: status}
		}
	}
	values, _ := body.(map[string]any)
	c.recorder.posts = append(c.recorder.posts, demoParentPost{parent: c.email, path: path, body: values})
	return []byte(`{"data":{}}`), nil
}

func newDemoParentTicker(t *testing.T, now *time.Time, parents []DemoParent, recorder *demoParentRecorder) *DemoTicker {
	t.Helper()
	return newDemoParentTickerWith(t, now, parents, recorder, &demoRecordingClient{})
}

func newDemoParentTickerWith(t *testing.T, now *time.Time, parents []DemoParent, recorder *demoParentRecorder, children *demoRecordingClient) *DemoTicker {
	t.Helper()
	state := minimalLiveState("")
	state.Accounts.Betreuer = []AccountCredentials{{StaffID: 17}}
	state.Activities = map[string]int64{"Hausaufgaben": 23}
	ticker, err := NewDemoTicker(DemoTickOptions{
		State: state, Client: children, Now: func() time.Time { return *now },
		Visits:  func(context.Context) ([]DemoVisit, error) { return nil, nil },
		Parents: parents, ParentClient: recorder.client,
	})
	require.NoError(t, err)
	return ticker
}

var demoOtherParents = []DemoParent{
	{Email: "petra.meyer@example.test", Password: "secret", StudentID: 102},
	{Email: "stefan.koch@example.test", Password: "secret", StudentID: 103},
}

func TestDemoParentTickAsksForAPickupChangeRightAwayAndThenWritesAMessage(t *testing.T) {
	t.Parallel()
	// Saturday evening: the pickup change still asks for a school day.
	now := time.Date(2026, 9, 19, 20, 0, 0, 0, time.UTC)
	recorder := &demoParentRecorder{}
	ticker := newDemoParentTicker(t, &now, demoOtherParents, recorder)

	require.NoError(t, ticker.Tick(t.Context()))
	require.Len(t, recorder.posts, 1, "the first tick after the entry fills the open requests")
	pickup := recorder.posts[0]
	assert.Equal(t, "petra.meyer@example.test", pickup.parent)
	assert.Equal(t, "/parent/me/children/102/care-exception", pickup.path)
	date, err := time.Parse("2006-01-02", pickup.body["date"].(string))
	require.NoError(t, err)
	assert.True(t, date.After(now.AddDate(0, 0, 1)), "the change concerns a later day, not today")
	assert.NotEqual(t, time.Saturday, date.Weekday())
	assert.NotEqual(t, time.Sunday, date.Weekday())
	assert.NotEmpty(t, pickup.body["pickup_time"])
	assert.NotEmpty(t, pickup.body["reason"])

	now = now.Add(time.Minute)
	require.NoError(t, ticker.Tick(t.Context()))
	assert.Len(t, recorder.posts, 1, "parents do not flood the school every few seconds")

	now = now.Add(demoParentActionInterval)
	require.NoError(t, ticker.Tick(t.Context()))
	require.Len(t, recorder.posts, 2)
	message := recorder.posts[1]
	assert.Equal(t, "/parent/me/messages/children/102", message.path)
	assert.NotEmpty(t, message.body["body"])
	assert.Equal(t, []string{"petra.meyer@example.test"}, recorder.logins, "one login serves the parent for a while")
}

func TestDemoParentTickTakesTurnsBetweenParents(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	recorder := &demoParentRecorder{}
	ticker := newDemoParentTicker(t, &now, demoOtherParents, recorder)

	for range 8 {
		require.NoError(t, ticker.Tick(t.Context()))
		now = now.Add(demoParentActionInterval)
	}
	parents := make(map[string]int)
	for _, post := range recorder.posts {
		parents[post.parent]++
	}
	assert.Len(t, recorder.posts, 8)
	assert.Positive(t, parents["petra.meyer@example.test"])
	assert.Positive(t, parents["stefan.koch@example.test"])
	for _, post := range recorder.posts {
		student := map[string]string{"petra.meyer@example.test": "/102", "stefan.koch@example.test": "/103"}[post.parent]
		assert.Contains(t, post.path, student, "a parent writes about the own child only")
	}
}

func TestDemoParentTickSkipsARejectedRequestWithoutFailingTheTick(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	recorder := &demoParentRecorder{reject: map[string]int{"/care-exception": 409}}
	ticker := newDemoParentTicker(t, &now, demoOtherParents, recorder)

	require.NoError(t, ticker.Tick(t.Context()), "a request the school refuses is not a broken simulation")
	now = now.Add(demoParentActionInterval)
	require.NoError(t, ticker.Tick(t.Context()))
	require.Len(t, recorder.posts, 1)
	assert.Contains(t, recorder.posts[0].path, "/messages/", "the parents go on with the next action")
}

func TestDemoParentTickNeedsParents(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	recorder := &demoParentRecorder{}
	ticker := newDemoParentTicker(t, &now, nil, recorder)
	require.NoError(t, ticker.Tick(t.Context()))
	assert.Empty(t, recorder.logins)
	assert.Empty(t, recorder.posts)
}

// The visitor's own parent account and every parent who shares its child
// stay out of the simulation: the visitor writes as that parent, and a
// co-parent's messages would appear in the visitor's own parent app.
func TestOtherDemoParentsLeaveTheVisitorsFamilyAlone(t *testing.T) {
	t.Parallel()
	parents := []demoprofile.ParentCredentials{
		{Email: "sabine@example.test", Password: "p", AccountID: 1, StudentIDs: []int64{101}},
		{Email: "klaus@example.test", Password: "p", AccountID: 2, StudentIDs: []int64{101}},
		{Email: "petra@example.test", Password: "p", AccountID: 3, StudentIDs: []int64{102}},
		{Email: "ohne-kind@example.test", Password: "p", AccountID: 4},
	}
	assert.Equal(t, []DemoParent{{Email: "petra@example.test", Password: "p", StudentID: 102}}, OtherDemoParents(parents, 1))
	assert.Len(t, OtherDemoParents(parents, 0), 3, "without a visitor every parent with a child takes part")
}

// A parent who cannot sign in is logged, not a failed tick: the first tick
// opens a new demo school, and the children move on all the same. The next
// try waits for the next interval instead of hammering the login.
func TestDemoParentTickFailureDoesNotStopTheChildren(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	recorder := &demoParentRecorder{loginErr: fmt.Errorf("parent login refused")}
	children := &demoRecordingClient{}
	ticker := newDemoParentTickerWith(t, &now, demoOtherParents, recorder, children)

	require.NoError(t, ticker.Tick(t.Context()))
	assert.Contains(t, children.studentActions, "/api/iot/checkin", "the day is rebuilt all the same")
	assert.Empty(t, recorder.posts)
	require.Equal(t, 1, recorder.loginAttempts)

	now = now.Add(time.Minute)
	require.NoError(t, ticker.Tick(t.Context()))
	assert.Equal(t, 1, recorder.loginAttempts, "no second try within the interval")
	now = now.Add(demoParentActionInterval)
	require.NoError(t, ticker.Tick(t.Context()))
	assert.Equal(t, 2, recorder.loginAttempts, "the next interval tries again")
}
