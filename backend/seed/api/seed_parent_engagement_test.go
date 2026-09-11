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
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
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
		"/parent/me/children/44/meal-participation/" + nextWeekday(todaySeedDate().AddDays(1).UTCMidnight(), time.Monday).Format(seedDateLayout),
	}, paths)
	assert.Equal(t, "Können Sie bitte prüfen, ob die neue Abholzeit eingetragen ist?", message["body"])
}

func TestSeedParentEngagementStepRequiresParentWithStudent(t *testing.T) {
	t.Parallel()

	err := (seedParentEngagementStep{}).Run(t.Context(), &Runtime{})
	require.ErrorContains(t, err, "parent engagement")
}
