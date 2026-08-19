package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// listStudentsPerms mirrors the permission the list route requires.
var listStudentsPerms = []string{"admin:*"}

// fillWideStudentFields writes the personal/administrative columns a real
// production row carries. Without them omitempty hides most of the full
// projection and a payload comparison would measure nothing.
//
// bus_days / pickup_days / departure_days / allowed_departure_modes are
// `scanonly` on the model, so they are written with raw SQL here exactly like
// the repository reads them.
func fillWideStudentFields(t *testing.T, db *bun.DB, studentID, personID int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.NewRaw(`UPDATE users.students SET
			guardian_name = 'Erika Mustermann',
			guardian_contact = '0221 1234567',
			guardian_email = 'erika.mustermann@example.com',
			guardian_phone = '+49 170 1234567',
			address_street = 'Musterstrasse 12a',
			address_city = 'Koeln',
			address_postal_code = '50667',
			extra_info = 'Geschwisterkind in Klasse 2b, kommt meist mit dem Rad',
			health_info = 'Laktoseintoleranz, Notfallmedikament im Gruppenraum',
			supervisor_notes = 'Braucht manchmal etwas mehr Zeit bei Uebergaengen',
			pickup_status = 'Wird abgeholt',
			bus_days = '{"mon":true,"tue":true,"wed":true,"thu":true,"fri":true}'::jsonb,
			pickup_days = '{"mon":true,"tue":true,"wed":true,"thu":true,"fri":true}'::jsonb,
			departure_days = '{"mon":"pickup","tue":"pickup","wed":"pickup","thu":"pickup","fri":"pickup"}'::jsonb,
			allowed_departure_modes = '{"mon":["pickup"],"tue":["pickup"],"wed":["pickup"],"thu":["pickup"],"fri":["pickup"]}'::jsonb,
			agb_accepted_at = now(),
			data_processing_accepted_at = now(),
			email_contact_accepted_at = now()
		WHERE id = ?`, studentID).Exec(ctx)
	require.NoError(t, err, "failed to fill wide student fields")

	_, err = db.NewRaw(`UPDATE users.persons SET birthday = DATE '2019-02-02' WHERE id = ?`, personID).Exec(ctx)
	require.NoError(t, err, "failed to set person birthday")
}

// listStudentIDs decodes the id of every row in a paginated students response.
func listStudentIDs(t *testing.T, body []byte) []int64 {
	t.Helper()

	var envelope struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))

	ids := make([]int64, 0, len(envelope.Data))
	for _, row := range envelope.Data {
		ids = append(ids, row.ID)
	}
	return ids
}

// TestListStudents_SlimProjection is the payload guard for #2097: the
// Kindersuche list must never carry the wide personal fields its cards,
// filters and grouping modes do not read. The child detail page fetches the
// full record separately.
func TestListStudents_SlimProjection(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	// Pin the clock to a fixed Monday: departure_modes exist for mon-fri only
	// (departureDayKey has no weekend mapping), so a real-today run fails on
	// every Saturday/Sunday CI run.
	tc.resource.Now = func() time.Time { return time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC) }

	student := testpkg.CreateTestStudent(t, tc.db, "Slim", "Kind", "SL1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	fillWideStudentFields(t, tc.db, student.ID, student.PersonID)

	req := testutil.NewRequest("GET", "/?view=slim&include_pickup_times=true&include_arrival_times=true", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), listStudentsPerms)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	body := rr.Body.String()
	for _, forbidden := range []string{
		"person_id", "tag_id", "birthday",
		"guardian_name", "guardian_contact", "guardian_email", "guardian_phone",
		"address_street", "address_city", "address_postal_code",
		"extra_info", "health_info", "supervisor_notes",
		"bus_days", "pickup_days", "departure_days", "allowed_departure_modes",
		"pickup_status",
		"departure_companion_note",
		"day_planning_reason",
		"photo_consent_given_at", "photo_consent_given_by",
		"agb_accepted_at", "data_processing_accepted_at", "email_contact_accepted_at",
		"created_at", "updated_at",
		`"bus":`,
	} {
		assert.NotContains(t, body, forbidden,
			"slim student list must not ship unrendered field %q (#2097)", forbidden)
	}

	// Everything the Kindersuche cards, filters and badges do read.
	for _, required := range []string{
		"id", "first_name", "last_name", "school_class",
		"current_location", "sick", "excused", "class_trip", "departure_modes", "has_full_access",
	} {
		assert.Contains(t, body, required, "slim student list must include %q", required)
	}
	assert.Contains(t, body, `"departure_modes":["pickup"]`)
}

// TestListStudents_FullViewKeepsWideProjection pins the default: only an
// explicit view=slim changes the wire shape, so the other nine consumers of
// GET /api/students (room detail, companion picker, roster search, …) keep the
// payload they read today.
func TestListStudents_FullViewKeepsWideProjection(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Wide", "Kind", "WD1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
	fillWideStudentFields(t, tc.db, student.ID, student.PersonID)

	for _, view := range []string{"", "?view=full"} {
		path := "/" + view
		req := testutil.NewRequest("GET", path, nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), listStudentsPerms)
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

		body := rr.Body.String()
		for _, required := range []string{
			"health_info", "supervisor_notes", "extra_info",
			"guardian_email", "address_street", "pickup_days",
			"allowed_departure_modes", "created_at",
		} {
			assert.Contains(t, body, required,
				"GET /students%s must keep the full projection", path)
		}
	}
}

// TestListStudents_InvalidView rejects an unknown view instead of silently
// serving the full payload — a typo in a caller must surface immediately.
func TestListStudents_InvalidView(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	req := testutil.NewRequest("GET", "/?view=compact", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), listStudentsPerms)
	assert.Equal(t, http.StatusBadRequest, rr.Code, "body: %s", rr.Body.String())
}

// TestListStudents_SlimViewSameRows proves the projection is a WIRE change
// only: both views return the same children in the same order, so filtering,
// day planning and pagination are untouched.
func TestListStudents_SlimViewSameRows(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	var studentIDs []int64
	for i := range 5 {
		student := testpkg.CreateTestStudent(t, tc.db, "SameRows", fmt.Sprintf("Kind%d", i), "SR1")
		fillWideStudentFields(t, tc.db, student.ID, student.PersonID)
		studentIDs = append(studentIDs, student.ID)
	}
	defer testpkg.CleanupActivityFixtures(t, tc.db, studentIDs...)

	query := "search=SameRows&include_pickup_times=true&include_arrival_times=true&page_size=100"

	fullRR := authExec(t, tc, testutil.NewRequest("GET", "/?"+query, nil), testutil.AdminTestClaims(1), listStudentsPerms)
	require.Equal(t, http.StatusOK, fullRR.Code, "body: %s", fullRR.Body.String())

	slimRR := authExec(t, tc, testutil.NewRequest("GET", "/?"+query+"&view=slim", nil), testutil.AdminTestClaims(1), listStudentsPerms)
	require.Equal(t, http.StatusOK, slimRR.Code, "body: %s", slimRR.Body.String())

	fullIDs := listStudentIDs(t, fullRR.Body.Bytes())
	require.Len(t, fullIDs, len(studentIDs))
	assert.Equal(t, fullIDs, listStudentIDs(t, slimRR.Body.Bytes()),
		"view=slim must not change which children are returned or their order")
}

// TestListStudents_SlimPayloadBudget bounds the wire size of the Kindersuche
// list. Production measured 51.9 KB on average and 198.4 KB at peak for this
// endpoint (benchmark 2026-07-30, #2063) because the page requests up to 1000
// rows of the full projection in one trip.
func TestListStudents_SlimPayloadBudget(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	var studentIDs []int64
	const listSize = 30
	for i := range listSize {
		student := testpkg.CreateTestStudent(t, tc.db, "Budget", fmt.Sprintf("Produktionskind%02d", i), "BG1")
		fillWideStudentFields(t, tc.db, student.ID, student.PersonID)
		studentIDs = append(studentIDs, student.ID)
	}
	defer testpkg.CleanupActivityFixtures(t, tc.db, studentIDs...)

	query := "search=Produktionskind&include_pickup_times=true&include_arrival_times=true&page_size=100"

	measure := func(view string) int {
		path := "/?" + query
		if view != "" {
			path += "&view=" + view
		}
		rr := authExec(t, tc, testutil.NewRequest("GET", path, nil), testutil.AdminTestClaims(1), listStudentsPerms)
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
		require.Len(t, listStudentIDs(t, rr.Body.Bytes()), listSize)
		return rr.Body.Len()
	}

	fullSize := measure("")
	slimSize := measure("slim")
	t.Logf("payload budget: %d students → full %d bytes (%.0f B/child), slim %d bytes (%.0f B/child), −%.0f%%",
		listSize, fullSize, float64(fullSize)/listSize, slimSize, float64(slimSize)/listSize,
		100*(1-float64(slimSize)/float64(fullSize)))

	// Budget: ≤ 350 bytes per child plus 1 KB envelope. A production list of
	// 1000 children must stay well under the ~198 KB observed on the full
	// projection. Raise only with a written justification.
	const maxBytes = 1024 + listSize*350
	assert.LessOrEqual(t, slimSize, maxBytes, "slim student list exceeded its payload budget (#2097)")

	// The projection must actually earn its keep: at least a third off the
	// full payload for the same rows.
	assert.LessOrEqual(t, slimSize, fullSize*2/3,
		"view=slim must cut at least a third of the full-projection payload")
}
