package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedParentEngagementStepUsesParentFacingFlows(t *testing.T) {
	t.Parallel()

	var paths []string
	var message map[string]any
	var mealException map[string]any
	var appointments []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/parent/me/children/44/meal-participation" {
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"effective_from":"2026-10-12"}}`)
			return
		}
		if r.URL.Path == "/parent/me/children/44/meal-participation/2026-10-14" {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&mealException))
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
			return
		}
		if r.URL.Path == "/api/students/44" && r.Method == seedHTTPMethodGet {
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"group_id":"7"}}`)
			return
		}
		if r.URL.Path == "/api/calendar/appointments" {
			var appointment map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&appointment))
			appointments = append(appointments, appointment)
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
			return
		}
		if r.URL.Path == "/parent/auth/login" {
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"parent-token"}}`)
			return
		}
		if r.URL.Path == "/parent/me/messages/children/44" && r.Method == seedHTTPMethodPost {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&message))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"thread_id":"91","messages":[{"id":"92"}]}}`)
			return
		}
		if r.URL.Path == "/parent/me/children/44/guardians" {
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"guardian_profile_id":"93"}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
	})
	defer srv.Close()

	rt := &Runtime{
		Client: newTestClient(srv.URL, false),
		Parents: []ParentCredentials{{
			Email: "parent@example.test", Password: "Parent1234%", StudentIDs: []int64{44},
		}},
	}
	rt.Adapter = rt.Client.adapter

	err := (seedParentEngagementStep{}).Run(t.Context(), rt)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"/parent/auth/login",
		"/parent/me/notification-preferences/parent_message",
		"/parent/me/messages/children/44",
		"/api/messages/threads/91",
		"/api/messages/threads/91/unread",
		"/parent/me/messages/children/44",
		"/parent/me/children/44/guardians",
		"/parent/me/children/44/guardians/93/pickup",
		"/api/settings/values/guardians.parent_invite_mode",
		"/parent/me/children/44/related-accounts",
		"/api/settings/values/guardians.parent_invite_mode",
		"/api/students/44",
		"/parent/me/children/44/consents/photo",
		"/parent/me/children/44/master-data/guardian_profile/preferred_contact_method",
		"/parent/me/children/44/master-data/requests",
		"/parent/me/children/44/meal-participation",
		"/parent/me/children/44/meal-participation/2026-10-14",
		"/api/students/44",
		"/api/calendar/appointments",
		"/api/calendar/appointments",
		"/api/calendar/appointments",
		"/api/calendar/appointments",
	}, paths)
	assert.Equal(t, "Können Sie bitte prüfen, ob die neue Abholzeit eingetragen ist?", message["body"])
	assert.Equal(t, map[string]any{"participating": false}, mealException)

	titles := make([]any, 0, len(appointments))
	for _, appointment := range appointments {
		titles = append(titles, appointment["title"])
		assert.Equal(t, false, appointment["send_email"])
	}
	assert.Equal(t, []any{"Elternabend der Gruppe", "Ausflug in den Zoo", "Pädagogischer Tag: OGS geschlossen", "Laternenfest"}, titles)
	assert.Equal(t, []any{
		map[string]any{"type": "parents_by_group", "id": float64(7)},
		map[string]any{"type": "all_staff"},
	}, appointments[0]["targets"])
}

// The lunch week the parents portal opens with mixes days with and without
// lunch, whichever weekday is the first one still open for changes (#3923).
// Regular days are Monday to Thursday.
func TestMealParticipationExceptionMixesTheOpeningWeek(t *testing.T) {
	t.Parallel()

	cases := []struct {
		firstChangeable string
		day             string
		participating   bool
	}{
		{firstChangeable: "2026-10-12", day: "2026-10-14", participating: false}, // Monday
		{firstChangeable: "2026-10-13", day: "2026-10-14", participating: false}, // Tuesday
		{firstChangeable: "2026-10-14", day: "2026-10-14", participating: false}, // Wednesday
		{firstChangeable: "2026-10-15", day: "2026-10-16", participating: true},  // Thursday
		{firstChangeable: "2026-10-16", day: "2026-10-16", participating: true},  // Friday
	}
	for _, tc := range cases {
		firstChangeable, err := parseSeedDate(tc.firstChangeable)
		require.NoError(t, err)
		day, participating := mealParticipationException(firstChangeable)
		assert.Equal(t, tc.day, day.String(), tc.firstChangeable)
		assert.Equal(t, tc.participating, participating, tc.firstChangeable)
	}
}

func TestSeedOutingDateFollowsTheOtherParentRequests(t *testing.T) {
	t.Parallel()

	friday, err := parseSeedDate("2026-10-09")
	require.NoError(t, err)
	// 15 days after a Friday is a Saturday; the outing moves to Monday.
	assert.Equal(t, "2026-10-26", seedOutingDate(friday).String())
	assert.Equal(t, time.Monday, seedOutingDate(friday).Weekday())
}

func TestSeedParentEngagementStepRequiresParentWithStudent(t *testing.T) {
	t.Parallel()

	err := (seedParentEngagementStep{}).Run(t.Context(), &Runtime{})
	require.ErrorContains(t, err, "parent engagement")
}
