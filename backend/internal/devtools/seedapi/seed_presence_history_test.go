package api

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// presenceHistoryFixture is a school like the demo: 100 children, six rooms,
// a closed weekday and two days of Herbstferien inside the window, care on
// weekdays only.
func presenceHistoryFixture() (presenceHistoryInput, map[calendar.Date]bool) {
	closed := map[calendar.Date]bool{
		calendar.NewDate(2026, time.October, 1):  true,
		calendar.NewDate(2026, time.October, 12): true,
		calendar.NewDate(2026, time.October, 13): true,
	}
	input := presenceHistoryInput{
		today:    calendar.NewDate(2026, time.October, 27),
		deviceID: 7,
		careDays: func(day calendar.Date) bool { return !calendar.IsWeekend(day) && !closed[day] },
	}
	for i := range 100 {
		input.students = append(input.students, presenceHistoryStudent{id: int64(1000 + i)})
	}
	for i := range 6 {
		input.sessions = append(input.sessions, presenceHistorySession{activityID: int64(50 + i), roomID: int64(90 + i), capacity: []int{25, 20, 80, 20, 40, 15}[i]})
	}
	return input, closed
}

// The Statistik of a fresh demo school (#5031) shows a plausible quota: most
// children come, some are sick or excused, few stay away unexplained, and
// the children differ.
func TestPresenceHistoryPlanGivesPlausibleQuotas(t *testing.T) {
	t.Parallel()
	input, _ := presenceHistoryFixture()
	plan := planPresenceHistory(input)

	careDays := 0
	for day := input.today.AddDays(-presenceHistoryDays); day.Before(input.today); day = day.AddDays(1) {
		if input.careDays(day) {
			careDays++
		}
	}
	require.Equal(t, 18, careDays, "29 days minus weekends, the closed day and two Ferien days")
	sick, excused := 0, 0
	for _, status := range plan.statuses {
		switch status.status {
		case "sick":
			sick += len(status.dates)
		case "excused":
			excused += len(status.dates)
		default:
			t.Fatalf("unexpected status %q", status.status)
		}
	}
	total := careDays * len(input.students)
	assert.Equal(t, total, len(plan.days)+sick+excused+plan.unexplained, "every care day of every child is decided once")
	rate := float64(len(plan.days)) / float64(total)
	assert.GreaterOrEqual(t, rate, 0.85)
	assert.LessOrEqual(t, rate, 0.95)
	assert.Positive(t, sick)
	assert.Positive(t, excused)
	assert.Positive(t, plan.unexplained)
	assert.Less(t, plan.unexplained, sick, "unexplained absences stay the exception")

	present := map[int64]int{}
	for _, day := range plan.days {
		present[day.StudentID]++
	}
	distinct := map[int]bool{}
	for _, count := range present {
		distinct[count] = true
	}
	assert.GreaterOrEqual(t, len(distinct), 4, "the children's attendance differs")

	// The children of a day spread over the rooms by capacity: no room holds
	// more children than it has places.
	perRoomDay := map[[2]any]int{}
	for _, day := range plan.days {
		perRoomDay[[2]any{day.Date, day.Visit.RoomID}]++
	}
	capacity := map[int64]int{}
	for _, session := range input.sessions {
		capacity[session.roomID] = session.capacity
	}
	for key, children := range perRoomDay {
		assert.LessOrEqual(t, children, capacity[key[1].(int64)], key)
	}
}

func TestPresenceHistoryPlanDoesNotOverfillRooms(t *testing.T) {
	t.Parallel()

	input, _ := presenceHistoryFixture()
	input.sessions = []presenceHistorySession{{activityID: 50, roomID: 90, capacity: 1}}
	plan := planPresenceHistory(input)

	visitsPerDay := map[calendar.Date]int{}
	withoutVisit := 0
	for _, day := range plan.days {
		if day.Visit == nil {
			withoutVisit++
			continue
		}
		assert.Equal(t, int64(90), day.Visit.RoomID)
		visitsPerDay[day.Date]++
	}
	for date, visits := range visitsPerDay {
		assert.LessOrEqual(t, visits, 1, date)
	}
	assert.Positive(t, withoutVisit, "attendance stays recorded when every room is full")
}

// No seeded day falls on a weekend, holiday or Ferien day, none reaches today
// or beyond the visit retention, and every visit lies inside its attendance
// and session.
func TestPresenceHistoryPlanStaysOnPastCareDays(t *testing.T) {
	t.Parallel()
	input, closed := presenceHistoryFixture()
	plan := planPresenceHistory(input)
	oldest := input.today.AddDays(-presenceHistoryDays)
	check := func(day calendar.Date) {
		assert.False(t, calendar.IsWeekend(day), day)
		assert.False(t, closed[day], day)
		assert.True(t, day.Before(input.today), day)
		assert.False(t, day.Before(oldest), day)
	}
	for _, day := range plan.days {
		check(day.Date)
		assert.Equal(t, day.Date, calendar.DateFromTime(day.CheckIn))
		assert.True(t, day.CheckOut.After(day.CheckIn))
		require.NotNil(t, day.Visit)
		assert.False(t, day.Visit.Entry.Before(day.CheckIn) || day.Visit.Exit.After(day.CheckOut), "visit inside attendance")
		assert.False(t, day.Visit.Entry.Before(day.Visit.SessionStart) || day.Visit.Exit.After(day.Visit.SessionEnd), "visit inside session")
		assert.Equal(t, input.deviceID, day.DeviceID)
	}
	for _, status := range plan.statuses {
		for _, day := range status.dates {
			check(day)
		}
	}
}

// The same day seeds the same history, so seeder tests and screenshots stay
// stable.
func TestPresenceHistoryPlanIsDeterministic(t *testing.T) {
	t.Parallel()
	input, _ := presenceHistoryFixture()
	assert.Equal(t, planPresenceHistory(input), planPresenceHistory(input))
}

// A child whose care ended gets no presence and no absence after its last
// care day: the Statistik no longer counts those days for it.
func TestPresenceHistoryPlanStopsAtTheEndOfCare(t *testing.T) {
	t.Parallel()
	input, _ := presenceHistoryFixture()
	lastDay := calendar.NewDate(2026, time.October, 9)
	input.students[0].lastDay = &lastDay
	plan := planPresenceHistory(input)
	seeded := 0
	for _, day := range plan.days {
		if day.StudentID == input.students[0].id {
			assert.False(t, day.Date.After(lastDay), day.Date)
			seeded++
		}
	}
	assert.Positive(t, seeded, "the days before the end stay")
	for _, status := range plan.statuses {
		if status.studentID != input.students[0].id {
			continue
		}
		for _, day := range status.dates {
			assert.False(t, day.After(lastDay), day)
		}
	}
}

func TestPresenceHistoryStatusDaysRollBackWhenHistoryWriteFails(t *testing.T) {
	t.Parallel()

	var deleted []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case seedHTTPMethodPost + " /api/students/7/status-days":
			_, _ = fmt.Fprint(w, `{"status":"success","data":[{"id":101,"date":"2026-10-01","status":"sick"},{"id":102,"date":"2026-10-02","status":"sick"}]}`)
		case seedHTTPMethodPost + " /api/students/8/status-days":
			_, _ = fmt.Fprint(w, `{"status":"success","data":[{"id":201,"date":"2026-10-03","status":"excused"}]}`)
		case seedHTTPMethodDelete + " /api/students/7/status-days/101", seedHTTPMethodDelete + " /api/students/7/status-days/102", seedHTTPMethodDelete + " /api/students/8/status-days/201":
			deleted = append(deleted, r.URL.Path)
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"deleted":true}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), TenantAuth: AuthRef{Kind: AuthBearer, Token: "tenant"}}
	statuses := []presenceHistoryStatus{
		{studentID: 7, status: "sick", dates: []calendar.Date{calendar.NewDate(2026, time.October, 1), calendar.NewDate(2026, time.October, 2)}},
		{studentID: 8, status: "excused", dates: []calendar.Date{calendar.NewDate(2026, time.October, 3)}},
	}

	historyErr := errors.New("presence writer unavailable")
	err := seedPresenceHistoryPlan(context.Background(), rt, 1, func(context.Context, int64, []PastPresenceDay) (int, error) {
		return 0, historyErr
	}, presenceHistoryPlan{statuses: statuses})
	require.ErrorIs(t, err, historyErr)
	assert.Equal(t, []string{
		"/api/students/7/status-days/101",
		"/api/students/7/status-days/102",
		"/api/students/8/status-days/201",
	}, deleted)
}

func TestPresenceHistoryStatusDaysRollBackEarlierWritesWhenLaterReportFails(t *testing.T) {
	t.Parallel()

	var deleted []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case seedHTTPMethodPost + " /api/students/7/status-days":
			_, _ = fmt.Fprint(w, `{"status":"success","data":[{"id":101,"date":"2026-10-01","status":"sick"}]}`)
		case seedHTTPMethodPost + " /api/students/8/status-days":
			w.WriteHeader(seedHTTPStatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"status":"error","message":"injected failure"}`)
		case seedHTTPMethodDelete + " /api/students/7/status-days/101":
			deleted = append(deleted, r.URL.Path)
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"deleted":true}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), TenantAuth: AuthRef{Kind: AuthBearer, Token: "tenant"}}
	writeCalled := false
	err := seedPresenceHistoryPlan(context.Background(), rt, 1, func(context.Context, int64, []PastPresenceDay) (int, error) {
		writeCalled = true
		return 0, nil
	}, presenceHistoryPlan{statuses: []presenceHistoryStatus{
		{studentID: 7, status: "sick", dates: []calendar.Date{calendar.NewDate(2026, time.October, 1)}},
		{studentID: 8, status: "excused", dates: []calendar.Date{calendar.NewDate(2026, time.October, 2)}},
	}})
	require.ErrorContains(t, err, "seed past excused days")
	assert.False(t, writeCalled)
	assert.Equal(t, []string{"/api/students/7/status-days/101"}, deleted)
}

func TestCreatedPresenceHistoryStatusDaysKeepsIDsFromAnIncompleteResponse(t *testing.T) {
	t.Parallel()

	rows, err := createdPresenceHistoryStatusDays([]byte(`{"status":"success","data":[{"id":101,"date":"2026-10-01","status":"sick"}]}`), presenceHistoryStatus{
		studentID: 7, status: "sick", dates: []calendar.Date{calendar.NewDate(2026, time.October, 1), calendar.NewDate(2026, time.October, 2)},
	})
	require.ErrorContains(t, err, "response missed 1 status days")
	assert.Equal(t, []presenceHistoryStatusDay{{studentID: 7, id: 101}}, rows)
}

func TestPresenceHistoryStatusDaysRecoverIDsFromAMalformedResponse(t *testing.T) {
	t.Parallel()

	var deleted []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case seedHTTPMethodPost + " /api/students/7/status-days":
			// The write committed both days, but the response is malformed.
			_, _ = fmt.Fprint(w, `{"status":"success","data":[`)
		case seedHTTPMethodGet + " /api/students/7/status-days":
			assert.Equal(t, "2026-10-01", r.URL.Query().Get("from"))
			assert.Equal(t, "2026-10-02", r.URL.Query().Get("to"))
			_, _ = fmt.Fprint(w, `{"status":"success","data":[{"id":101,"date":"2026-10-01","status":"sick"},{"id":102,"date":"2026-10-02","status":"sick"}]}`)
		case seedHTTPMethodDelete + " /api/students/7/status-days/101", seedHTTPMethodDelete + " /api/students/7/status-days/102":
			deleted = append(deleted, r.URL.Path)
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"deleted":true}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), TenantAuth: AuthRef{Kind: AuthBearer, Token: "tenant"}}
	writeCalled := false
	err := seedPresenceHistoryPlan(context.Background(), rt, 1, func(context.Context, int64, []PastPresenceDay) (int, error) {
		writeCalled = true
		return 0, nil
	}, presenceHistoryPlan{statuses: []presenceHistoryStatus{
		{studentID: 7, status: "sick", dates: []calendar.Date{calendar.NewDate(2026, time.October, 1), calendar.NewDate(2026, time.October, 2)}},
	}})
	require.ErrorContains(t, err, "read created past sick days")
	assert.False(t, writeCalled)
	assert.Equal(t, []string{"/api/students/7/status-days/101", "/api/students/7/status-days/102"}, deleted)
}
