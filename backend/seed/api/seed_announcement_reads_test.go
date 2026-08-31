package api

import (
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
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/parent-announcements/":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"71"}}`)
		case "/parent/auth/login":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"parent-token"}}`)
		case "/api/parent-announcements/71/publish":
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
	assert.Equal(t, []string{
		"/api/parent-announcements/", "/api/parent-announcements/71/publish",
		"/parent/auth/login", "/parent/me/news/71/read",
	}, paths)
}
