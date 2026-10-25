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
	var childMessages []map[string]any
	var staffReplies []map[string]any
	var mealException, contact, pickupUpdate, masterDataRequest map[string]any
	var appointments []map[string]any
	var groupThreads []map[string]any
	var pickupStatements []map[string]any
	var pickupSignatures []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		decode := func() map[string]any {
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			return body
		}
		switch {
		case r.URL.Path == "/parent/me/children/44/meal-participation":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"effective_from":"2026-10-12"}}`)
		case r.URL.Path == "/parent/me/children/44/meal-participation/2026-10-14":
			mealException = decode()
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		case r.URL.Path == "/api/students/44" && r.Method == seedHTTPMethodGet:
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"group_id":"7"}}`)
		case r.URL.Path == "/api/groups/7":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"name":"Sternengruppe"}}`)
		case r.URL.Path == "/api/messages/group-threads" && r.Method == seedHTTPMethodPost:
			groupThreads = append(groupThreads, decode())
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"thread":{"id":"%d"}}}`, 60+len(groupThreads))
		case r.URL.Path == "/parent/me/messages/children/44" && r.Method == seedHTTPMethodPost:
			childMessages = append(childMessages, decode())
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"thread_id":"91","messages":[{"id":"90"},{"id":"%d"}]}}`, 91+len(childMessages))
		case r.URL.Path == "/api/messages/threads/91":
			staffReplies = append(staffReplies, decode())
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		case r.URL.Path == "/api/calendar/appointments":
			appointments = append(appointments, decode())
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		case r.URL.Path == "/parent/auth/login":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"parent-token"}}`)
		case r.URL.Path == "/parent/me/children/44/guardians/93/pickup":
			pickupUpdate = decode()
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		case r.URL.Path == "/parent/me/children/44/pickup-signature-statement":
			pickupStatements = append(pickupStatements, decode())
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"statement":"Erklärung %d"}}`, len(pickupStatements))
		// The signed pickup permission, the one-day collector and the way home
		// alone (#4780).
		case r.URL.Path == "/parent/me/children/44/pickup-permission-requests":
			_, _ = fmt.Fprint(w, `{"status":"success","data":[{"id":"95","guardian_profile_id":"93","status":"pending"}]}`)
		case r.URL.Path == "/api/students/change-requests":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"items":[`+
				`{"request_type":"care_schedule","expected_version":"v95","data":{"id":"95","impact_token":""}},`+
				`{"request_type":"care_schedule","expected_version":"v96","data":{"id":"96","impact_token":""}}]}}`)
		case r.URL.Path == "/parent/me/children/44/care-exception":
			request := decode()
			pickupSignatures = append(pickupSignatures, request["signature"].(map[string]any))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"96"}}`)
		case r.URL.Path == "/parent/me/children/44/guardians":
			contact = decode()
			pickupSignatures = append(pickupSignatures, contact["pickup_signature"].(map[string]any))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"guardian_profile_id":"93"}}`)
		case r.URL.Path == "/parent/me/children/44/master-data" && r.Method == seedHTTPMethodGet:
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"allowed_departure_modes":{"mon":["pickup"],"tue":["pickup"],"wed":["pickup"],"thu":["pickup"],"fri":["pickup"]}}}`)
		case r.URL.Path == "/parent/me/children/44/master-data/requests":
			masterDataRequest = decode()
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		default:
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		}
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
		"/parent/me/notification-preferences/parent_announcement",
		"/parent/me/notification-preferences/parent_appointment",
		"/parent/me/notification-preferences/parent_request_decided",
		"/parent/me/notification-preferences/parent_care_cancelled",
		"/api/students/44",
		"/api/groups/7",
		"/api/messages/group-threads",
		"/api/messages/group-threads/61/messages",
		"/api/messages/group-threads",
		"/api/messages/group-threads/62/close",
		"/parent/me/messages/group-threads/61",
		"/parent/me/messages/children/44",
		"/parent/me/children/44/pickup-signature-statement",
		"/parent/me/children/44/guardians",
		"/parent/me/children/44/guardians/93/pickup",
		"/parent/me/children/44/pickup-permission-requests",
		"/api/students/change-requests",
		"/api/students/care-schedule-change-requests/95/decide",
		"/parent/me/children/44/pickup-signature-statement",
		"/parent/me/children/44/care-exception",
		"/api/students/change-requests",
		"/api/students/care-schedule-change-requests/96/decide",
		"/parent/me/children/44/pickup-signature-statement",
		"/parent/me/children/44/care-exception",
		"/api/settings/values/guardians.parent_invite_mode",
		"/parent/me/children/44/related-accounts",
		"/api/settings/values/guardians.parent_invite_mode",
		"/api/students/44",
		"/parent/me/children/44/consents/photo",
		"/parent/me/children/44/master-data/guardian_profile/preferred_contact_method",
		"/parent/me/children/44/master-data",
		"/parent/me/children/44/master-data/requests",
		"/parent/me/children/44/meal-participation",
		"/parent/me/children/44/meal-participation/2026-10-14",
		// The conversation comes after every step that leaves a note in the
		// chat, so it ends with the OGS's answer.
		"/parent/me/messages/children/44", "/api/messages/threads/91",
		"/parent/me/messages/children/44", "/api/messages/threads/91",
		"/parent/me/messages/children/44", "/api/messages/threads/91",
		"/api/messages/threads/91/unread",
		"/parent/me/messages/children/44",
		"/api/students/44",
		"/api/calendar/appointments", "/api/calendar/appointments", "/api/calendar/appointments", "/api/calendar/appointments",
		"/api/calendar/appointments", "/api/calendar/appointments", "/api/calendar/appointments",
		"/api/calendar/appointments", "/api/calendar/appointments", "/api/calendar/appointments",
		"/api/calendar/appointments", "/api/calendar/appointments",
	}, paths)

	require.Len(t, childMessages, 4)
	assert.Equal(t, "61", childMessages[0]["group_thread_id"], "the parent answers the open group thread")
	conversation := childMessages[1:]
	require.Len(t, staffReplies, len(conversation), "the OGS answers every message of the conversation")
	for i, message := range conversation {
		assert.Equal(t, seedParentConversationTurns[i].parent, message["body"])
		assert.Nil(t, message["group_thread_id"])
		assert.Equal(t, seedParentConversationTurns[i].staff, staffReplies[i]["body"])
		assert.Equal(t, fmt.Sprint(93+i), staffReplies[i]["handled_up_to_message_id"], "the answer covers the newest parent message")
	}
	assert.Equal(t, map[string]any{"participating": false}, mealException)
	require.Len(t, groupThreads, 2)
	assert.Equal(t, "group", groupThreads[0]["target_type"])
	assert.Equal(t, "7", groupThreads[0]["target_id"])
	assert.Equal(t, "Gruppe Sternengruppe", groupThreads[0]["target_label"])
	assert.Equal(t, "Laternen basteln", groupThreads[0]["title"])

	assert.Equal(t, "gisela.hahn@email.de", contact["email"])
	assert.Equal(t, map[string]any{
		"can_pickup": true, "is_emergency_contact": true, "pickup_notes": "Nur nach telefonischer Rücksprache",
	}, pickupUpdate, "Gisela may pick the child up, after a phone call")
	assert.Equal(t, []any{map[string]any{"phone_number": "+49 172 5550778", "phone_type": "mobile", "is_primary": true}}, contact["phones"],
		"a mobile number without a second label")
	assert.Equal(t, []any{map[string]any{
		"target": "departure", "field_key": "allowed_departure_modes",
		"value": map[string]any{"mon": []any{"pickup"}, "tue": []any{"pickup"}, "wed": []any{"pickup"}, "thu": []any{"pickup"}, "fri": []any{"alone"}},
	}}, masterDataRequest["changes"])

	titles := make([]any, 0, len(appointments))
	for _, appointment := range appointments {
		titles = append(titles, appointment["title"])
		assert.Equal(t, false, appointment["send_email"])
	}
	assert.Equal(t, []any{
		"Elternabend der Gruppe", "Ausflug in den Zoo", "Erntedank-Frühstück", "Herbstwanderung",
		"Elternsprechtag", "Elternsprechtag", "Elternsprechtag", "Der Schulfotograf kommt", "Elterncafé in der OGS",
		"Anmeldeschluss für die Ferienbetreuung", "Pädagogischer Tag: OGS geschlossen", "Laternenfest",
	}, titles)
	assert.Equal(t, []any{
		map[string]any{"type": "parents_by_group", "id": float64(7)},
		map[string]any{"type": "all_staff"},
	}, appointments[0]["targets"])
	require.Len(t, pickupStatements, 3)
	assert.Equal(t, "pickup_permission", pickupStatements[0]["kind"])
	assert.Equal(t, "Gisela Hahn", pickupStatements[0]["person_name"])
	assert.Equal(t, []any{float64(2), float64(4)}, pickupStatements[0]["weekdays"])
	assert.Equal(t, "day_pickup", pickupStatements[1]["kind"])
	assert.Equal(t, "Gisela Hahn", pickupStatements[1]["person_name"])
	assert.Equal(t, "day_pickup", pickupStatements[2]["kind"])
	assert.Equal(t, "alone", pickupStatements[2]["departure_mode"])
	require.Len(t, pickupSignatures, 3)
	for i, signature := range pickupSignatures {
		assert.Equal(t, fmt.Sprintf("Erklärung %d", i+1), signature["statement"])
	}
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

// The families' calendar holds the Elternsprechtag on the three days its
// poll offers, two school events of this month that already took place,
// and the Laternenfest of the Elternbrief.
func TestSeedSchoolAppointmentsShareTheDatesOfPollAndLetter(t *testing.T) {
	t.Parallel()

	today := mustParseSeedDate(t, "2026-10-20") // Tuesday, like the CI run
	byTitle := map[string][]string{}
	for _, appointment := range seedSchoolAppointments(today) {
		title, _ := appointment["title"].(string)
		day, _ := appointment["start_date"].(string)
		byTitle[title] = append(byTitle[title], day)
	}
	assert.Equal(t, []string{"2026-10-27", "2026-10-28", "2026-10-29"}, byTitle["Elternsprechtag"])
	for _, day := range seedParentPollDays(today) {
		assert.Contains(t, byTitle["Elternsprechtag"], day.String())
	}
	assert.Equal(t, []string{"2026-10-07"}, byTitle["Erntedank-Frühstück"])
	assert.Equal(t, []string{"2026-10-12"}, byTitle["Herbstwanderung"])
	assert.Equal(t, []string{seedLanternFestivalDate(today).String()}, byTitle["Laternenfest"])

	sunday := mustParseSeedDate(t, "2026-10-25")
	for _, appointment := range seedPastSchoolAppointments(sunday) {
		day := mustParseSeedDate(t, appointment["start_date"].(string))
		assert.Negative(t, day.Compare(sunday), "%s lies in the past", appointment["title"])
		assert.NotContains(t, []time.Weekday{time.Saturday, time.Sunday}, day.Weekday(), "%s is a school day", appointment["title"])
	}
}
