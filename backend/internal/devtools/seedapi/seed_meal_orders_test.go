package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The order day is the next weekday, so its regular registrations are made
// before the deadline is moved back behind today (#3873).
func TestMealOrderSeedDayIsDueAtAnyTime(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		today, day string
		lead       int
	}{
		{"2026-09-08", "2026-09-09", 2}, // Tuesday → Wednesday
		{"2026-09-11", "2026-09-14", 4}, // Friday → Monday
		{"2026-09-12", "2026-09-14", 3}, // Saturday → Monday
		{"2026-09-13", "2026-09-14", 2}, // Sunday → Monday
	} {
		today, err := parseSeedDate(tc.today)
		require.NoError(t, err)
		day, lead := mealOrderSeedDay(today)
		assert.Equal(t, tc.day, day.String(), tc.today)
		assert.Equal(t, tc.lead, lead, tc.today)
	}
}

// The children the school registers spread over the classes before a class
// gets a second one, and stop when the classes run out.
func TestPickAcrossClassesSpreadsOverClasses(t *testing.T) {
	t.Parallel()
	children := []mealChild{{1, "1a"}, {2, "1a"}, {3, "1a"}, {4, "2b"}, {5, "3c"}, {6, "3c"}}
	assert.Equal(t, []mealChild{{1, "1a"}, {4, "2b"}, {5, "3c"}, {2, "1a"}}, pickAcrossClasses(children, 4))
	assert.Len(t, pickAcrossClasses(children, 10), 6)
}

// Every child the school registers eats on the order day, whatever its
// pattern leaves out.
func TestMealWeekdaysWithContainTheOrderDay(t *testing.T) {
	t.Parallel()
	for weekday := time.Monday; weekday <= time.Friday; weekday++ {
		for index := range mealOrderWeekdays {
			assert.Contains(t, mealWeekdaysWith(index, weekday), int(weekday), "pattern %d on %s", index, weekday)
		}
	}
}

// mealOrderServer answers the meal order and caterer routes the seed
// drives and records each write in order.
type mealOrderServer struct {
	t          *testing.T
	date       string
	events     []string
	failDayFor string
}

func (s *mealOrderServer) handle(w seedHTTPResponseWriter, r *seedHTTPRequest) {
	w.Header().Set("Content-Type", "application/json")
	var body map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	path := r.URL.Path
	switch {
	case path == "/api/meal-plan/participants":
		assert.Equal(s.t, s.date, r.URL.Query().Get("date"))
		assert.Equal(s.t, "all", r.URL.Query().Get("include"))
		_, _ = fmt.Fprint(w, mealOrderListJSON(18, 50))
	case strings.HasSuffix(path, "/schedule"):
		s.events = append(s.events, fmt.Sprintf("schedule %s %v %s", strings.Split(path, "/")[4], body["weekdays"], body["effective_from"]))
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"effective_from":"2026-09-08"}}`)
	case strings.Contains(path, "/days/"):
		s.events = append(s.events, fmt.Sprintf("day %s %v", strings.TrimPrefix(path, "/api/meal-plan/participation/"), body["participating"]))
		if s.failDayFor != "" && strings.Contains(path, s.failDayFor) {
			w.WriteHeader(seedHTTPStatusConflict)
			_, _ = fmt.Fprint(w, `{"status":409,"code":"general.conflict","detail":"boom"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
	case path == "/api/settings/values/operations.meal_registration_lead_days":
		s.events = append(s.events, fmt.Sprintf("lead %v", body["value"]))
		_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
	default:
		s.handleCaterer(w, r, body)
	}
}

func (s *mealOrderServer) handleCaterer(w seedHTTPResponseWriter, r *seedHTTPRequest, body map[string]any) {
	switch r.URL.Path {
	case "/api/meal-caterer/invitations":
		s.events = append(s.events, fmt.Sprintf("invite %s %s", body["email"], body["caterer_name"]))
		w.WriteHeader(seedHTTPStatusCreated)
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"3","status":"open"}}`)
	case "/caterer/invitations/7/tok123/accept":
		assert.Empty(s.t, r.Header.Get("Authorization"), "the caterer accepts without a session")
		assert.Equal(s.t, body["password"], body["confirm_password"])
		s.events = append(s.events, "accept")
		w.WriteHeader(seedHTTPStatusNoContent)
	case "/caterer/auth/login":
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"caterer-token"}}`)
	case "/caterer/schools/7/orders":
		assert.Equal(s.t, s.date, r.URL.Query().Get("from"))
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"days":[{"date":"2026-09-08","status":"ordered","order_id":"55"}]}}`)
	case "/caterer/schools/7/orders/55/confirm":
		assert.Equal(s.t, "Bearer caterer-token", r.Header.Get("Authorization"))
		s.events = append(s.events, "confirm")
		w.WriteHeader(seedHTTPStatusNoContent)
	default:
		s.t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		w.WriteHeader(seedHTTPStatusNotFound)
	}
}

// mealOrderListJSON is a day list with participants 1..n in three classes
// and others 101..100+m; child 1 is the first parent's child.
func mealOrderListJSON(participants, others int) string {
	child := func(id int) string {
		return fmt.Sprintf(`{"student_id":"%d","first_name":"K","last_name":"%d","school_class":"%da"}`, id, id, id%3+1)
	}
	var eating, rest []string
	for id := 1; id <= participants; id++ {
		eating = append(eating, child(id))
	}
	for id := 101; id <= 100+others; id++ {
		rest = append(rest, child(id))
	}
	return `{"status":"success","data":{"date":"2026-09-08","participants":[` + strings.Join(eating, ",") + `],"others":[` + strings.Join(rest, ",") + `]}}`
}

func mealOrderRuntime(url string) *Runtime {
	return &Runtime{Client: newTestClient(url, false), Parents: []ParentCredentials{{StudentIDs: []int64{1}}}, Values: map[string]any{}}
}

// The school tops the day up to mealOrderChildren with regular registrations
// before the deadline moves, fixes the day with one cancellation and two
// late registrations, restores the deadline, and the caterer it invited
// accepts the mailed link and confirms that order (#3873, ADR 0077).
func TestSeedMealOrdersFixesABusyDayAndConnectsTheCaterer(t *testing.T) {
	t.Parallel()
	server := &mealOrderServer{t: t, date: "2026-09-09"}
	srv := newSeedHTTPTestServer(server.handle)
	defer srv.Close()
	var mailed []string
	var mailedSince time.Time
	readMailLink := func(_ context.Context, recipient, marker string, notBefore time.Time) (string, error) {
		mailed, mailedSince = append(mailed, recipient, marker), notBefore
		return "http://caterer.localhost:3000/einladung/7/tok123", nil
	}
	today, err := parseSeedDate("2026-09-08")
	require.NoError(t, err)
	invitedAt := time.Date(2026, time.September, 8, 7, 21, 0, 0, time.UTC)
	rt := mealOrderRuntime(srv.URL)
	rt.Values[seedCatererInvitedAt] = invitedAt

	require.NoError(t, seedMealOrders(context.Background(), rt, readMailLink, today))

	// The families' 18 children plus the school's registrations.
	registered := mealOrderChildren - 18
	require.Len(t, server.events, registered+1+1+2+1+2)
	for _, event := range server.events[:registered] {
		assert.True(t, strings.HasPrefix(event, "schedule "), event)
		assert.True(t, strings.HasSuffix(event, " 2026-09-09"), event)
	}
	// One child of each class first: 101 is in 3a, 102 in 1a, 103 in 2a.
	assert.True(t, strings.HasPrefix(server.events[0], "schedule 101 "), server.events[0])
	assert.True(t, strings.HasPrefix(server.events[1], "schedule 102 "), server.events[1])
	assert.Equal(t, []string{
		"lead 2",
		"day 2/days/2026-09-09 false", // never the first parent's child 1
		fmt.Sprintf("day %d/days/2026-09-09 true", 100+registered+1),
		fmt.Sprintf("day %d/days/2026-09-09 true", 100+registered+2),
		"lead 0",
		"accept",
		"confirm",
	}, server.events[registered:])
	assert.Equal(t, []string{"kontakt@frischekueche-rheinland.example", "/einladung/"}, mailed)
	assert.Equal(t, invitedAt, mailedSince, "only a mail after the invitation counts")
}

// A failed late change still restores the school's deadline.
func TestSeedMealOrdersRestoresTheDeadlineAfterAFailure(t *testing.T) {
	t.Parallel()
	server := &mealOrderServer{t: t, date: "2026-09-09", failDayFor: fmt.Sprintf("/%d/", 100+mealOrderChildren-18+1)}
	srv := newSeedHTTPTestServer(server.handle)
	defer srv.Close()
	today, err := parseSeedDate("2026-09-08")
	require.NoError(t, err)

	require.Error(t, seedMealOrders(context.Background(), mealOrderRuntime(srv.URL), nil, today))
	assert.Equal(t, "lead 0", server.events[len(server.events)-1])
	assert.Contains(t, server.events, "lead 2")
}

// The school invites its caterer once meal registration is on, and only
// with a mailbox to accept the invitation from: in the demo sidecar the
// mail would go to an address nobody reads (ADR 0077).
func TestSeedMealCatererInvitationStepNeedsAMailbox(t *testing.T) {
	t.Parallel()
	server := &mealOrderServer{t: t}
	srv := newSeedHTTPTestServer(server.handle)
	defer srv.Close()
	readMailLink := func(context.Context, string, string, time.Time) (string, error) { return "", nil }

	withoutMailbox := mealOrderRuntime(srv.URL)
	require.NoError(t, seedMealCatererInvitationStep{seeder: &Seeder{}}.Run(context.Background(), withoutMailbox))
	assert.Empty(t, server.events)
	assert.NotContains(t, withoutMailbox.Values, seedCatererInvitedAt)

	withMailbox := mealOrderRuntime(srv.URL)
	step := seedMealCatererInvitationStep{seeder: &Seeder{options: SeedOptions{ReadMailLink: readMailLink}}}
	require.NoError(t, step.Run(context.Background(), withMailbox))
	assert.Equal(t, []string{"invite kontakt@frischekueche-rheinland.example Frischeküche Rheinland GmbH"}, server.events)
	assert.IsType(t, time.Time{}, withMailbox.Values[seedCatererInvitedAt])
}

// Without a mailbox the order week is still fixed, and no caterer accepts.
func TestSeedMealOrdersWithoutMailboxConnectsNoCaterer(t *testing.T) {
	t.Parallel()
	// Sunday: the next order day is Monday.
	server := &mealOrderServer{t: t, date: "2026-09-14"}
	srv := newSeedHTTPTestServer(server.handle)
	defer srv.Close()
	today, err := parseSeedDate("2026-09-13")
	require.NoError(t, err)

	require.NoError(t, seedMealOrders(context.Background(), mealOrderRuntime(srv.URL), nil, today))
	assert.Contains(t, server.events, "lead 2")
	assert.Equal(t, "lead 0", server.events[len(server.events)-1])
}

// A caterer mail still queued behind another seed's mails leaves the
// invitation open; the order week is seeded all the same.
func TestSeedMealOrdersLeavesTheInvitationOpenWithoutItsMail(t *testing.T) {
	t.Parallel()
	server := &mealOrderServer{t: t, date: "2026-09-09"}
	srv := newSeedHTTPTestServer(server.handle)
	defer srv.Close()
	readMailLink := func(context.Context, string, string, time.Time) (string, error) {
		return "", errors.New("timed out waiting for mail with a link")
	}
	today, err := parseSeedDate("2026-09-08")
	require.NoError(t, err)
	rt := mealOrderRuntime(srv.URL)
	rt.Values[seedCatererInvitedAt] = time.Date(2026, time.September, 8, 7, 21, 0, 0, time.UTC)

	require.NoError(t, seedMealOrders(context.Background(), rt, readMailLink, today))
	assert.Equal(t, "lead 0", server.events[len(server.events)-1], "no caterer write follows")
	assert.NotContains(t, server.events, "accept")
}
