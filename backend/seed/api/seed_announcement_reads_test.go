package api

import (
	"encoding/json"
	"fmt"
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
	assert.Equal(t, "Dienstag, 14. Oktober, 14:00 Uhr", poll.Options[0])
	assert.Equal(t, "Donnerstag, 16. Oktober, 18:45 Uhr", poll.Options[len(poll.Options)-1])
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
	assert.Equal(t, map[string]any{"student_id": "10", "action": "agreed", "version_id": "5"}, declared,
		"the parent declares only for a child they may declare for")
}
