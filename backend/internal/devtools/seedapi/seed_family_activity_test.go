package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every family message names the child where the template asks for it, and
// a text without a name stays as it is: fmt.Sprintf turned "Danke, das
// tragen wir ein." into "…%!(EXTRA string=Oscar)" in the staff inbox.
func TestSeedFamilyConversationsWriteCleanTexts(t *testing.T) {
	t.Parallel()

	var parentMessages, staffAnswers []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/parent/auth/login" {
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"parent-token"}}`)
			return
		}
		var body struct {
			Body string `json:"body"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		switch {
		case strings.HasPrefix(r.URL.Path, "/parent/me/messages/children/"):
			parentMessages = append(parentMessages, body.Body)
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"thread_id":"9","messages":[{"id":"5"}]}}`)
		case r.URL.Path == "/api/messages/threads/9":
			staffAnswers = append(staffAnswers, body.Body)
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	rt := familyRuntime(srv.URL, 30)
	rt.FixedSeeder = &FixedSeeder{studentIDByIndex: map[int]int64{}}
	for i := range rt.FamilyAccounts {
		rt.FixedSeeder.studentIDByIndex[i] = rt.FamilyAccounts[i].StudentIDs[0]
	}

	written, err := seedFamilyConversations(t.Context(), rt)
	require.NoError(t, err)
	assert.Equal(t, len(seedFamilyConversationTemplates), written)
	require.Len(t, parentMessages, len(seedFamilyConversationTemplates))
	require.Len(t, staffAnswers, 3, "two conversations stay unanswered")
	for _, text := range append(parentMessages, staffAnswers...) {
		assert.NotContains(t, text, "%!", text)
		assert.NotContains(t, text, "{child}", text)
	}
	assert.Equal(t, "Guten Morgen! "+DemoStudents[1].FirstName+" hat heute einen Zahnarzttermin. Ich hole "+
		DemoStudents[1].FirstName+" um 14:30 Uhr ab.", parentMessages[0])
	assert.Equal(t, "Danke, das tragen wir ein.", staffAnswers[2])
}
