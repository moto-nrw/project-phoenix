package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every notice of the day has an end: today's end today, the weekly one
// before the Herbstferien, so none reads „täglich (unbefristet)“.
func TestStaffNoticesHaveARealisticValidity(t *testing.T) {
	t.Parallel()

	tuesday := seedDate{Time: time.Date(2026, time.September, 22, 0, 0, 0, 0, time.UTC)}
	notices := staffNotices(tuesday)
	require.Len(t, notices, 5)
	audiences := map[string]bool{}
	for _, notice := range notices {
		assert.Equal(t, "2026-09-22", notice["valid_from"], notice["title"])
		assert.NotEmpty(t, notice["valid_until"], notice["title"])
		audiences[notice["audience"].(string)] = true
	}
	assert.Equal(t, map[string]bool{"all": true, "staff": true, "lehrkraft": true}, audiences, "#2208: every portal sees its share")
	assert.Equal(t, "2026-09-22", notices[0]["valid_until"], "the Räumungsübung is today")
	gym := notices[2]
	assert.Equal(t, "Turnhalle belegt", gym["title"])
	assert.Equal(t, []int16{2}, gym["weekdays"], "every Tuesday")
	assert.Contains(t, gym["body"], "dienstags")
	assert.Equal(t, "2026-10-09", gym["valid_until"], "until the Friday before the Herbstferien")
}

// The confirmations come from colleagues: the author's own one is stamped on
// creation and not counted.
func TestSeedStaffNoticesStepHasColleaguesConfirm(t *testing.T) {
	t.Parallel()

	var created []map[string]any
	confirmations := map[string][]string{}
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/login":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"access_token":"token-%s"}}`, body["email"])
		case r.URL.Path == "/api/staff-notices/":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "Bearer admin", r.Header.Get("Authorization"), "the Leitung writes the notices")
			created = append(created, body)
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":"%d"}}`, 70+len(created))
		case strings.HasSuffix(r.URL.Path, "/acknowledge"):
			confirmations[r.URL.Path] = append(confirmations[r.URL.Path], r.Header.Get("Authorization"))
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	for index, staff := range DemoStaff[:12] {
		fs.staffCredentials = append(fs.staffCredentials, StaffCredentials{
			Email: fmt.Sprintf("staff%d@example.test", index), Password: "Staff1234%", Name: staff.FirstName + " " + staff.LastName,
		})
	}
	rt := &Runtime{Client: fs.client, Adapter: fs.client.adapter, FixedSeeder: fs, TenantAuth: AuthRef{Kind: AuthBearer, Token: "admin"}}
	require.NoError(t, (seedStaffNoticesStep{}).Run(t.Context(), rt))

	require.Len(t, created, 5)
	readers := []string{
		"Bearer token-staff0@example.test", "Bearer token-staff1@example.test", "Bearer token-staff3@example.test",
		"Bearer token-staff4@example.test", "Bearer token-staff11@example.test",
	}
	assert.Equal(t, map[string][]string{
		"/api/staff-notices/71/acknowledge": readers,
		"/api/staff-notices/72/acknowledge": readers,
	}, confirmations, "the notices for everybody that ask for it")
	assert.Equal(t, "admin", rt.Client.auth.Token, "the step hands the admin session back")
}
