package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// regularBookingRuntime is a demo school whose first twelve children of the
// master data have the IDs 1001 to 1012, child 1001 belongs to the demo
// parent, and the running phase is 77.
func regularBookingRuntime(url string) *Runtime {
	ids := map[int]int64{}
	for index := range 12 {
		ids[index] = int64(1001 + index)
	}
	for _, index := range careExitDemoStudentIndexes() {
		ids[index] = int64(9000 + index)
	}
	rt := &Runtime{
		Client:      newTestClient(url, false),
		FixedSeeder: &FixedSeeder{studentIDByIndex: ids},
		Parents:     []ParentCredentials{{StudentIDs: []int64{1001}}},
		Values:      map[string]any{enrollmentPhaseBodyKey: map[string]any{"name": "Anmeldung", "audience": "open"}},
	}
	rt.Enrollment.PhaseID = 77
	rt.Enrollment.Offerings = map[string]int64{"ogs-ganztag": 11, "ogs-kurz": 12, "mittagessen": 13}
	return rt
}

// Every child of the master data who eats regularly gets a booking, in
// cohort order. The demo parent's child already has one, a child whose care
// ends soon would lose its care exit, and a child from an application has
// its own booking (#5023).
func TestRegularBookingStudentsAreTheBookableChildrenWhoEat(t *testing.T) {
	t.Parallel()
	rt := regularBookingRuntime("http://127.0.0.1")
	exiting := rt.FixedSeeder.studentIDByIndex[careExitDemoStudentIndexes()[1]]
	for _, studentID := range []int64{1005, 1001, 1003, exiting, 5555} {
		recordRegularLunchDays(rt, studentID, []int{1, 2, 3})
	}

	lunchDays := rt.Values[regularLunchDaysKey].(map[int64][]int)
	assert.Equal(t, []int64{1003, 1005}, regularBookingStudents(rt, lunchDays))
}

// The school registers no child for lunch whom it cannot book, so the meal
// plan never shows a child eating without a booking.
func TestBookableMealChildrenLeaveOutChildrenWithoutBooking(t *testing.T) {
	t.Parallel()
	rt := regularBookingRuntime("http://127.0.0.1")
	exiting := rt.FixedSeeder.studentIDByIndex[careExitDemoStudentIndexes()[0]]
	children := []mealChild{{1002, "Klasse 1a"}, {exiting, "Klasse 4b"}, {5555, "Klasse 1a"}, {1004, "Klasse 1b"}}

	assert.Equal(t, []mealChild{{1002, "Klasse 1a"}, {1004, "Klasse 1b"}}, bookableMealChildren(rt, children))
	assert.Equal(t, children, bookableMealChildren(&Runtime{}, children), "without the fixed seeder every child stays")
}

// regularBookingServer answers the routes of the bookings step and records
// the writes in order. A booking lands on the child of the master data with
// the booked name, whose ID regularBookingRuntime gives it.
type regularBookingServer struct {
	t        *testing.T
	mu       sync.Mutex
	events   []string
	bookings []map[string]any
	// wrongStudent answers every booking with another child.
	wrongStudent bool
}

func (s *regularBookingServer) handle(w seedHTTPResponseWriter, r *seedHTTPRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	var body map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	switch {
	case r.URL.Path == "/api/enrollment/phases/77":
		s.events = append(s.events, fmt.Sprintf("audience %v", body["audience"]))
		_, _ = fmt.Fprint(w, `{"status":"success","data":{}}`)
	case strings.HasPrefix(r.URL.Path, "/api/guardians/students/"):
		studentID := strings.Split(r.URL.Path, "/")[4]
		_, _ = fmt.Fprintf(w, `{"status":"success","data":[`+
			`{"guardian":{"first_name":"Oma","last_name":"X","email":"oma@example.test"},"relationship_id":1,"relationship_type":"relative","is_primary":false},`+
			`{"guardian":{"first_name":"Eva","last_name":"Muster","email":"eva.%s@example.test"},"relationship_id":%s,"relationship_type":"parent","is_primary":true}]}`, studentID, studentID)
	case r.URL.Path == "/api/enrollment/phases/77/manual-approved-enrollments":
		s.bookings = append(s.bookings, body)
		child := body["children"].([]any)[0].(map[string]any)
		studentID := ""
		for index, demo := range DemoStudents[:12] {
			if demo.FirstName == child["first_name"] && demo.LastName == child["last_name"] {
				studentID = fmt.Sprint(1001 + index)
			}
		}
		if s.wrongStudent {
			studentID = "4711"
		}
		s.events = append(s.events, "book "+studentID)
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"request_id":"9","child_id":"9","student_id":%q,"status":"approved"}}`, studentID)
	case strings.HasPrefix(r.URL.Path, "/api/guardians/relationships/"):
		s.events = append(s.events, fmt.Sprintf("relationship %s %v", strings.Split(r.URL.Path, "/")[4], body["relationship_type"]))
		_, _ = fmt.Fprint(w, `{"status":"success","data":{}}`)
	default:
		s.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(seedHTTPStatusNotFound)
	}
}

// The school books care and lunch on exactly the child's lunch days, while
// the phase admits only existing children, names the child's primary
// guardian, keeps the guardian's relationship type and opens the phase to
// everyone again (#5023).
func TestSeedRegularBookingsBooksCareAndLunchOnTheLunchDays(t *testing.T) {
	t.Parallel()
	server := &regularBookingServer{t: t}
	srv := newSeedHTTPTestServer(server.handle)
	defer srv.Close()
	rt := regularBookingRuntime(srv.URL)
	recordRegularLunchDays(rt, 1002, []int{1, 2, 3, 4, 5})
	recordRegularLunchDays(rt, 1003, []int{1, 2, 4})

	require.NoError(t, seedRegularBookingsStep{}.Run(context.Background(), rt))

	require.Len(t, server.events, 6)
	assert.Equal(t, "audience existing_students", server.events[0])
	assert.Equal(t, "audience open", server.events[5])
	// The two children are booked in parallel; each booking comes before the
	// restored relationship of its own child.
	for _, studentID := range []string{"1002", "1003"} {
		book := slices.Index(server.events, "book "+studentID)
		relationship := slices.Index(server.events, "relationship "+studentID+" parent")
		assert.Positive(t, book, "child %s is booked", studentID)
		assert.Greater(t, relationship, book, "child %s keeps its relationship after the booking", studentID)
	}
	require.Len(t, server.bookings, 2)
	leonAt := slices.IndexFunc(server.bookings, func(booking map[string]any) bool {
		return booking["guardian_email"] == "eva.1003@example.test"
	})
	require.GreaterOrEqual(t, leonAt, 0)
	leon := server.bookings[leonAt]
	assert.Equal(t, "eva.1003@example.test", leon["guardian_email"])
	assert.Equal(t, map[string]any{"data_processing": true}, leon["consent_flags"], "the photo consent stays as it is")
	assert.Equal(t, true, leon["external_consent_confirmed"])
	assert.Equal(t, false, leon["send_notification"])
	child := leon["children"].([]any)[0].(map[string]any)
	assert.Equal(t, "Koch", child["last_name"])
	assert.EqualValues(t, 1, child["target_grade_level"], "the child keeps its class")
	assert.Equal(t, []any{float64(11), float64(13)}, child["offering_ids"])
	assert.Equal(t, []any{
		map[string]any{"offering_id": float64(11), "selected_days": []any{"mon", "tue", "thu"}},
		map[string]any{"offering_id": float64(13), "selected_days": []any{"mon", "tue", "thu"}},
	}, child["offering_days"])
}

// Every third booked child takes the Kurzbetreuung.
func TestSeedRegularBookingsMixesGanztagAndKurzbetreuung(t *testing.T) {
	t.Parallel()
	server := &regularBookingServer{t: t}
	srv := newSeedHTTPTestServer(server.handle)
	defer srv.Close()
	rt := regularBookingRuntime(srv.URL)
	for studentID := int64(1002); studentID <= 1007; studentID++ {
		recordRegularLunchDays(rt, studentID, []int{1, 3, 5})
	}
	require.NoError(t, seedRegularBookingsStep{}.Run(context.Background(), rt))

	// The bookings run in parallel, so the care follows the child's place
	// in cohort order, not the order the server sees them in.
	careOf := map[string]any{}
	for _, booking := range server.bookings {
		careOf[booking["guardian_email"].(string)] = booking["children"].([]any)[0].(map[string]any)["offering_ids"].([]any)[0]
	}
	assert.Equal(t, map[string]any{
		"eva.1002@example.test": float64(11), "eva.1003@example.test": float64(11), "eva.1004@example.test": float64(12),
		"eva.1005@example.test": float64(11), "eva.1006@example.test": float64(11), "eva.1007@example.test": float64(12),
	}, careOf)
	assert.Equal(t, "audience open", server.events[len(server.events)-1])
}

// A booking that lands on another child would be a duplicate child; the
// step stops and still opens the phase to everyone again.
func TestSeedRegularBookingsRefusesABookingOnAnotherChild(t *testing.T) {
	t.Parallel()
	server := &regularBookingServer{t: t, wrongStudent: true}
	srv := newSeedHTTPTestServer(server.handle)
	defer srv.Close()
	rt := regularBookingRuntime(srv.URL)
	recordRegularLunchDays(rt, 1002, []int{1})

	err := seedRegularBookingsStep{}.Run(context.Background(), rt)

	require.ErrorContains(t, err, `landed on student "4711"`)
	assert.Equal(t, []string{"audience existing_students", "book 4711", "audience open"}, server.events)
}

// The bookings follow the meal orders and the phase, and precede the child
// quota, with and without the deferred history.
func TestRegularBookingsRunAfterTheMealOrdersAndThePhase(t *testing.T) {
	t.Parallel()
	position := func(steps []Step) map[string]int {
		at := map[string]int{}
		for index, step := range steps {
			switch step.(type) {
			case seedMealOrdersStep:
				at["meals"] = index
			case parentRequestsSeedStep:
				at["phase"] = index
			case seedFamilyAppAccountsStep:
				at["families"] = index
			case seedRegularBookingsStep:
				at["bookings"] = index
			case seedChildQuotaStep:
				at["quota"] = index
			}
		}
		return at
	}
	full := position(fullDemoWorkflow(&Seeder{}).Steps)
	require.Contains(t, full, "bookings")
	assert.Greater(t, full["bookings"], full["meals"])
	assert.Greater(t, full["bookings"], full["phase"])
	assert.Greater(t, full["bookings"], full["families"])
	assert.Less(t, full["bookings"], full["quota"])

	deferredSeeder := &Seeder{options: SeedOptions{DeferHistory: true}}
	opening := position(fullDemoWorkflow(deferredSeeder).Steps)
	assert.NotContains(t, opening, "bookings", "the phase does not exist before the deferred steps")
	assert.Contains(t, opening, "meals")
	deferred := position(deferredDemoSteps(deferredSeeder))
	require.Contains(t, deferred, "bookings")
	assert.Greater(t, deferred["bookings"], deferred["phase"])
	assert.Greater(t, deferred["bookings"], deferred["families"])
	assert.Less(t, deferred["bookings"], deferred["quota"])
}

// Every child the school registers for lunch is noted with its lunch days,
// so the bookings that follow cover each of them: with the families'
// eighteen children the meal plan's sixty eaters on the order day all have
// a lunch booking (#5023).
func TestMealOrdersNoteTheLunchDaysTheBookingsFollow(t *testing.T) {
	t.Parallel()
	server := &mealOrderServer{t: t, date: "2026-09-08"}
	srv := newSeedHTTPTestServer(server.handle)
	defer srv.Close()
	rt := mealOrderRuntime(srv.URL)
	ids := map[int]int64{}
	for index := range 50 {
		ids[index] = int64(101 + index)
	}
	rt.FixedSeeder = &FixedSeeder{studentIDByIndex: ids}
	today, err := parseSeedDate("2026-09-07")
	require.NoError(t, err)
	initialDay, err := readMealDay(rt, today.AddDays(1))
	require.NoError(t, err)
	for _, child := range initialDay.participants {
		recordRegularLunchDays(rt, child.id, []int{1, 2, 3, 4, 5}) // the families' registrations
	}

	require.NoError(t, seedMealOrders(context.Background(), rt, nil, today))

	lunchDays := rt.Values[regularLunchDaysKey].(map[int64][]int)
	booked := regularBookingStudents(rt, lunchDays)
	assert.Len(t, booked, mealOrderChildren-18)
	for _, studentID := range booked {
		assert.Contains(t, lunchDays[studentID], 2, "child %d eats on the order day, a Tuesday", studentID)
	}
}

// The approved applications that booked lunch eat on their booked days from
// the day after the fixed order day; a child who eats already, a pending
// application and a child without a lunch booking stay as they are (#5023).
func TestApprovedApplicationsWithLunchEatOnTheirBookedDays(t *testing.T) {
	t.Parallel()
	var schedules []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/enrollment/admin/requests/1":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"children":[`+
				`{"status":"approved","created_student_id":"201","offerings":[{"offering_id":"11"},{"offering_id":"13","selected_days":["fri","mon"]}]},`+
				`{"status":"approved","created_student_id":"202","offerings":[{"offering_id":"13","selected_days":["tue"]}]},`+
				`{"status":"approved","created_student_id":"203","offerings":[{"offering_id":"12","selected_days":["wed"]}]}]}}`)
		case strings.HasPrefix(r.URL.Path, "/api/meal-plan/participation/"):
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			schedules = append(schedules, fmt.Sprintf("%s %v %v", strings.Split(r.URL.Path, "/")[4], body["weekdays"], body["effective_from"]))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()
	rt := regularBookingRuntime(srv.URL)
	rt.Enrollment.Requests = []SeedEnrollmentRequest{{RequestID: 1, Status: "approved"}, {RequestID: 2, Status: "waitlisted"}}
	recordRegularLunchDays(rt, 202, []int{2})

	registered, err := registerBookedApplicationLunch(rt, rt.TenantAuth, rt.Values[regularLunchDaysKey].(map[int64][]int))

	require.NoError(t, err)
	assert.Equal(t, 1, registered)
	assert.Equal(t, []string{"201 [1 5] " + bookedApplicationLunchFrom(todaySeedDate()).String()}, schedules)
	assert.Equal(t, []int{1, 5}, rt.Values[regularLunchDaysKey].(map[int64][]int)[201])
}

// The applications eat from the day after the order day, which stays fixed.
func TestBookedApplicationLunchStartsAfterTheOrderDay(t *testing.T) {
	t.Parallel()
	friday, err := parseSeedDate("2026-10-23")
	require.NoError(t, err)
	assert.Equal(t, "2026-10-27", bookedApplicationLunchFrom(friday).String(), "order day Monday, lunch from Tuesday")
}
