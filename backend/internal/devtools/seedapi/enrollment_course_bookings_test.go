package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The approved online enrollments fill the courses believably: the
// Fußball-AG about three quarters, the Theater-AG to its last place, so the
// parents app keeps a full course, and the Ferienbetreuung partly. Every
// booked child is an approved enrollment of the seed.
func TestSeedEnrollmentCourseBookingsFillTheOfferings(t *testing.T) {
	t.Parallel()

	capacity := map[string]int{}
	for _, offering := range demoCareOfferings() {
		if offering.capacity != nil {
			capacity[offering.key] = *offering.capacity
		}
	}
	// Sophie Sharma's enrollment selects the holiday care itself.
	booked := map[string]int{"ferienbetreuung": 1}
	for child, keys := range seedEnrollmentCourseBookings {
		require.NotEqual(t, "Sophie Sharma", child)
		for _, key := range keys {
			booked[key]++
		}
	}
	assert.Equal(t, map[string]int{"ferienbetreuung": 20, "ag-fussball": 12, "ag-theater": 8}, capacity)
	assert.Equal(t, 9, booked["ag-fussball"], "nine of twelve")
	assert.Equal(t, capacity["ag-theater"], booked["ag-theater"], "the Theater-AG is full")
	assert.Equal(t, 8, booked["ferienbetreuung"], "eight of twenty")
}

func TestWithSeedCourseBookingsAddsTheCoursesOnce(t *testing.T) {
	t.Parallel()

	offerings := map[string]int64{"ogs-kurz": 1, "mittagessen": 2, "ag-fussball": 5, "ag-theater": 6, "ferienbetreuung": 4}
	selected := []int64{1, 2}
	assert.Equal(t, []int64{1, 2, 5, 6, 4}, withSeedCourseBookings(offerings, "Clara", "Neumann", selected))
	assert.Equal(t, []int64{1, 2}, selected, "the selection stays untouched")
	assert.Equal(t, []int64{1, 6, 2}, withSeedCourseBookings(offerings, "Zeynep", "Çelik", []int64{1, 6, 2}),
		"a course the form already selects is not added twice")
	assert.Equal(t, []int64{1, 2}, withSeedCourseBookings(offerings, "Luka", "Berisha", selected),
		"the open request books no course")
}

// Luka Berisha's open request answers the form's questions about the child,
// which the decision page shows under "Angaben der Kinder" (product
// screenshot anmeldung-entscheiden).
func TestEnrollmentSubmissionAnswersTheQuestionsAboutTheChild(t *testing.T) {
	t.Parallel()

	offerings := map[string]int64{"ogs-ganztag": 3, "ogs-kurz": 1, "mittagessen": 2, "ag-fussball": 5}
	body := parentEnrollmentSeedStep{}.enrollmentSubmissionWithDays(1, offerings, "Luka", "Berisha", "2016-08-21", 4,
		"Arta", "Berisha", "arta.berisha@example.test", "submitted-4-days", []int64{3, 1, 2}, nil)
	children, ok := body["children"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, children, 1)
	assert.Equal(t, map[string]any{
		"allergies":           "Laktoseintoleranz. Bitte keine Milchprodukte beim Mittagessen.",
		"swimming_permission": true,
	}, children[0]["custom_data"])
	assert.Equal(t, []int64{3, 1, 2}, children[0]["offering_ids"])

	assert.Equal(t, map[string]any{"allergies": "keine", "swimming_permission": false}, seedEnrollmentChildAnswers("Emma", "Klein"))
	assert.Equal(t, map[string]any{"allergies": "keine", "swimming_permission": true}, seedEnrollmentChildAnswers("Oskar", "Wolf"))
}

// The sick note of the third family is for a day the child stays at home,
// and its text fits a note written in the morning: "seit heute Mittag" stood
// next to a child the kiosk showed present since noon.
func TestSeedParentPortalActionsReportSicknessFromTheMorning(t *testing.T) {
	t.Parallel()

	var sickNotes []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == seedHTTPMethodPost && strings.HasSuffix(r.URL.Path, "/sick-note") {
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			sickNotes = append(sickNotes, body)
		}
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"77"}}`)
	})
	defer srv.Close()

	parents := []ParentCredentials{
		{Email: "sabine@example.test", StudentIDs: []int64{1}, GuardianID: 10},
		{Email: "klaus@example.test", StudentIDs: []int64{1}, GuardianID: 11},
		{Email: "shirin@example.test", StudentIDs: []int64{4, 3}},
		{Email: "olga@example.test", StudentIDs: []int64{17}},
		{Email: "ingrid@example.test", StudentIDs: []int64{30}},
		{Email: "elfriede@example.test", StudentIDs: []int64{52}, Name: "Elfriede Franke"},
	}
	auths := map[string]AuthRef{}
	for _, parent := range parents {
		auths[parent.Email] = AuthRef{Token: parent.Email}
	}
	rt := &Runtime{Client: newTestClient(srv.URL, false), Parents: parents}
	actions, err := parentEnrollmentSeedStep{}.seedParentPortalActions(rt, auths, parents, todaySeedDate())
	require.NoError(t, err)
	require.Len(t, actions, 4)

	require.Len(t, sickNotes, 2, "a sick note and an excused absence")
	sick := sickNotes[0]
	assert.Equal(t, "Fieber seit heute Morgen. Wir bleiben zur Sicherheit zu Hause und gehen zum Kinderarzt.", sick["reason"])
	assert.NotContains(t, sick["reason"], "Mittag")
	assert.Len(t, sick["dates"], 1)
	assert.Equal(t, int64(3), actions[0].StudentID, "Aida Amini is reported sick; her brother Baran stays present")
}
