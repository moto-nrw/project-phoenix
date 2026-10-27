package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The conversations alternate between both people, each opening the thread
// before answering, and two answers to Thomas Schmidt, the admin of the
// product screenshots, stay unread.
func TestSeedStaffMessagingStepLeavesTwoThreadsUnreadForTheAdmin(t *testing.T) {
	t.Parallel()

	type event struct{ method, thread, who string }
	var events []event
	threadOf := map[string]string{}
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		who := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer token-")
		switch r.URL.Path {
		case "/auth/login":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"access_token":"token-%s"}}`, body["email"])
		case "/api/staff-messages/recipients":
			rows := make([]string, 0, len(DemoStaff))
			for index, staff := range DemoStaff[:14] {
				rows = append(rows, fmt.Sprintf(`{"account_id":"%d","name":"%s %s"}`, 100+index, staff.FirstName, staff.LastName))
			}
			_, _ = fmt.Fprintf(w, `{"status":"success","data":[%s]}`, strings.Join(rows, ","))
		case "/api/staff-messages/threads/open":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			id := fmt.Sprintf("t%d", len(threadOf)+1)
			threadOf[id] = who + ">" + body["account_id"].(string)
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"thread_id":"%s"}}`, id)
		default:
			events = append(events, event{r.Method, strings.TrimPrefix(r.URL.Path, "/api/staff-messages/threads/"), who})
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		}
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	for index, staff := range DemoStaff[:14] {
		fs.staffCredentials = append(fs.staffCredentials, StaffCredentials{
			Email: fmt.Sprintf("s%d", index), Password: "Staff1234%", Name: staff.FirstName + " " + staff.LastName,
		})
	}
	rt := &Runtime{Client: fs.client, Adapter: fs.client.adapter, FixedSeeder: fs, TenantAuth: AuthRef{Kind: AuthBearer, Token: "admin"}}
	require.NoError(t, (seedStaffMessagingStep{}).Run(t.Context(), rt))

	require.Len(t, threadOf, len(staffChatConversations))
	assert.Equal(t, "s0>101", threadOf["t1"], "the first staff member writes to Thomas Schmidt")
	// A thread is unread for Thomas when someone else wrote last and he has
	// not opened it since.
	unreadForThomas := map[string]bool{}
	sent := 0
	for _, e := range events {
		thomas := strings.HasPrefix(threadOf[e.thread], "s1>") || strings.HasSuffix(threadOf[e.thread], ">101")
		switch {
		case e.method == seedHTTPMethodPost:
			sent++
			unreadForThomas[e.thread] = thomas && e.who != "s1"
		case e.who == "s1":
			unreadForThomas[e.thread] = false
		}
	}
	total := 0
	for _, conversation := range staffChatConversations {
		total += len(conversation.messages)
	}
	assert.Equal(t, total, sent)
	unread := 0
	for _, isUnread := range unreadForThomas {
		if isUnread {
			unread++
		}
	}
	assert.Equal(t, 2, unread)
	assert.Equal(t, "admin", rt.Client.auth.Token, "the step hands the admin session back")
}
