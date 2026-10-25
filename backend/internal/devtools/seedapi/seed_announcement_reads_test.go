package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedAnnouncementsMarksOperatorAnnouncementSeenByStaff(t *testing.T) {
	t.Parallel()

	var paths []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":%d}}`, len(paths))
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), OperatorAuth: AuthRef{Token: "operator"}, TenantAuth: AuthRef{Token: "staff"}}
	require.NoError(t, (seedAnnouncementsStep{}).Run(t.Context(), rt))
	assert.Equal(t, []string{
		"/operator/announcements", "/operator/announcements", "/operator/announcements",
		"/api/platform/announcements/1/seen",
	}, paths)
}

func TestSeedParentLetterMarksLetterReadByParent(t *testing.T) {
	t.Parallel()

	var paths []string
	createdAnnouncements := 0
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/parent-announcements/":
			createdAnnouncements++
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":"%d"}}`, 70+createdAnnouncements)
		case "/parent/auth/login":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"parent-token"}}`)
		case "/api/parent-announcements/71/publish", "/api/parent-announcements/72/publish":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"published_at":"2026-08-31T00:00:00Z"}}`)
		default:
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	rt := &Runtime{
		Client: client, Adapter: client.adapter, TenantAuth: AuthRef{Token: "staff"},
		Parents: []ParentCredentials{{Email: "parent@example.test", Password: "Parent1234%"}},
	}
	require.NoError(t, seedParentLetter(rt))
	// The attachment upload sits between create and publish on purpose
	// (#2890): a published announcement is immutable, so an upload after the
	// publish call would be refused with 409 and the demo letter would ship
	// without its file.
	// The step ends with the announcement that carries a scheduled reminder
	// (#3162): created and published, nothing else — the reminder is a field
	// of the announcement, not a further call.
	assert.Equal(t, []string{
		"/api/parent-announcements/", "/api/announcement-attachments/71",
		"/api/parent-announcements/71/publish",
		"/parent/auth/login", "/parent/me/news/71/read",
		"/api/parent-announcements/", "/api/parent-announcements/72/publish",
	}, paths)
}

func TestSeedParentPollCreatesSixtyAppointmentOptions(t *testing.T) {
	t.Parallel()

	var poll struct {
		ResponseType string   `json:"response_type"`
		Options      []string `json:"options"`
	}
	var paths []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/parent-announcements/":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&poll))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"73"}}`)
		case "/api/parent-announcements/73/publish":
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		default:
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), TenantAuth: AuthRef{Token: "staff"}}
	require.NoError(t, (seedParentPollStep{}).Run(t.Context(), rt))

	assert.Equal(t, []string{"/api/parent-announcements/", "/api/parent-announcements/73/publish"}, paths)
	assert.Equal(t, "multi_choice", poll.ResponseType)
	require.Len(t, poll.Options, 60)
	assert.Equal(t, "Mittwoch, 14. Oktober, 14:00 Uhr", poll.Options[0])
	assert.Equal(t, "Freitag, 16. Oktober, 18:45 Uhr", poll.Options[len(poll.Options)-1])
	distinct := make(map[string]struct{}, len(poll.Options))
	for _, option := range poll.Options {
		distinct[option] = struct{}{}
	}
	assert.Len(t, distinct, len(poll.Options), "each appointment option must be distinct")
}

func TestSeedParentDeclarationDeclaresThroughTheParentAPI(t *testing.T) {
	t.Parallel()

	var paths []string
	var declared map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/parent-announcements/":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"81"}}`)
		case "/parent/auth/login":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"parent-token"}}`)
		case "/parent/me/news":
			_, _ = fmt.Fprintf(w, `{"status":"success","data":[{"id":"81","title":%q,"declaration":{"version":{"id":"5"},"children":[{"student_id":"9","can_submit":false},{"student_id":"10","can_submit":true}]}}]}`, seedDeclarationTitle)
		case "/parent/me/news/81/declaration":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&declared))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"created":true}}`)
		default:
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	rt := &Runtime{
		Client: client, Adapter: client.adapter, TenantAuth: AuthRef{Token: "staff"},
		Parents: []ParentCredentials{{Email: "parent@example.test", Password: "Parent1234%"}},
	}
	require.NoError(t, (seedParentDeclarationStep{}).Run(t.Context(), rt))
	// The document is attached before the first publication: after it the
	// attachments of an Erklärung are fixed.
	assert.Equal(t, []string{
		"/api/parent-announcements/", "/api/announcement-attachments/81",
		"/api/parent-announcements/81/publish",
		"/parent/auth/login", "/parent/me/news", "/parent/me/news/81/declaration",
	}, paths)
	signature, ok := declared["signature"].(map[string]any)
	require.True(t, ok, "the seeded consent is signed (#4785)")
	delete(declared, "signature")
	assert.Equal(t, map[string]any{"student_id": "10", "action": "agreed", "version_id": "5"}, declared,
		"the parent declares only for a child they may declare for")
	assert.Equal(t, "drawn", signature["method"])
	assert.NotEmpty(t, signature["image_png"])
}

func TestSeedParentPollAnswersForOtherFamiliesOnly(t *testing.T) {
	t.Parallel()

	// The first parent keeps the poll open: the parent portal screenshot shows
	// the open answer form (#3759). The second parent shares the first
	// parent's child and answers only for the child that is theirs alone.
	pollFeed := func(studentIDs ...string) string {
		children := make([]string, 0, len(studentIDs))
		for _, id := range studentIDs {
			children = append(children, fmt.Sprintf(`{"student_id":%q}`, id))
		}
		return `{"status":"success","data":[{"id":"73","title":"Termine für den Elternsprechtag","published_at":"2026-10-20T10:00:00Z",` +
			`"options":[{"id":"o1"},{"id":"o2"},{"id":"o3"},{"id":"o4"},{"id":"o5"},{"id":"o6"},{"id":"o7"},{"id":"o8"}],` +
			`"children":[` + strings.Join(children, ",") + `]}]}`
	}
	feeds := map[string]string{
		"Bearer token-second": pollFeed("1", "2"),
		"Bearer token-third":  pollFeed("3"),
	}
	tokens := map[string]string{
		"second@example.test": "token-second",
		"third@example.test":  "token-third",
	}
	type answer struct {
		auth string
		body map[string]any
	}
	var answers []answer
	var logins []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/parent-announcements/":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"73"}}`)
		case "/api/parent-announcements/73/publish":
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		case "/parent/auth/login":
			var login struct {
				Email string `json:"email"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&login))
			logins = append(logins, login.Email)
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"access_token":%q}}`, tokens[login.Email])
		case "/parent/me/news":
			_, _ = fmt.Fprint(w, feeds[r.Header.Get("Authorization")])
		case "/parent/me/news/73/respond":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			answers = append(answers, answer{auth: r.Header.Get("Authorization"), body: body})
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"answered":true}}`)
		default:
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	rt := &Runtime{
		Client: client, Adapter: client.adapter, TenantAuth: AuthRef{Token: "staff"},
		Parents: []ParentCredentials{
			{Email: "first@example.test", Password: "Parent1234%", StudentIDs: []int64{1}},
			{Email: "second@example.test", Password: "Parent1234%"},
			{Email: "third@example.test", Password: "Parent1234%"},
		},
	}
	require.NoError(t, (seedParentPollStep{}).Run(t.Context(), rt))

	assert.Equal(t, []string{"second@example.test", "third@example.test"}, logins,
		"the first parent never signs in to answer")
	require.Len(t, answers, 2, "one answer per child outside the first family")
	assert.Equal(t, "Bearer token-second", answers[0].auth)
	assert.Equal(t, "2", answers[0].body["student_id"])
	assert.Equal(t, "Bearer token-third", answers[1].auth)
	assert.Equal(t, "3", answers[1].body["student_id"])
	for _, a := range answers {
		assert.Equal(t, "2026-10-20T10:00:00Z", a.body["published_at"])
		options, ok := a.body["option_ids"].([]any)
		require.True(t, ok)
		assert.NotEmpty(t, options)
	}
	assert.NotEqual(t, answers[0].body["option_ids"], answers[1].body["option_ids"],
		"families pick different appointments, so the results show a spread")
}

func TestSeedParentPollRefusesWithoutTheFirstFamilysChildren(t *testing.T) {
	t.Parallel()

	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/parent-announcements/":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"73"}}`)
		case "/api/parent-announcements/73/publish":
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		default:
			t.Errorf("no parent may answer without the first family known: %s", r.URL.Path)
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	rt := &Runtime{
		Client: client, Adapter: client.adapter, TenantAuth: AuthRef{Token: "staff"},
		Parents: []ParentCredentials{
			{Email: "first@example.test", Password: "Parent1234%"},
			{Email: "second@example.test", Password: "Parent1234%"},
		},
	}
	require.ErrorContains(t, (seedParentPollStep{}).Run(t.Context(), rt), "first demo parent has no children")
}
