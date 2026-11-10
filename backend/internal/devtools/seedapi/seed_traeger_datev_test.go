package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestSeedDatevFileHandlesLegacyRunsWithoutHidingOtherFailures(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name      string
		status    int
		body      string
		wantError bool
	}{
		{name: "new file", status: http.StatusOK, body: "EXTF;700"},
		{name: "already exported", status: http.StatusConflict, body: `{"code":"parent_billing.datev_no_new_bookings"}`},
		{name: "concurrent download", status: http.StatusConflict, body: `{"code":"parent_billing.datev_bookings_taken"}`},
		{name: "legacy invoice without debtor", status: http.StatusConflict, body: `{"code":"parent_billing.datev_bookings_invalid","details":{"documents":[{"document_number":"MB-2026-0001","field":"debtor_number","reason":"missing"}]}}`},
		{name: "unrelated conflict", status: http.StatusConflict, body: `{"code":"parent_billing.datev_not_ready"}`, wantError: true},
		{name: "server failure", status: http.StatusInternalServerError, body: `{"code":"general.internal_error"}`, wantError: true},
		{name: "empty file", status: http.StatusOK, wantError: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			downloads := 0
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				assert.Equal(t, "Bearer traeger", r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/traeger/settings/values/"):
					_, _ = fmt.Fprint(w, `{"data":{}}`)
				case r.Method == http.MethodGet && r.URL.Path == "/traeger/care-billing-runs/":
					_, _ = fmt.Fprint(w, `{"data":{"runs":[{"id":5,"billing_month":"2026-11-01","status":"released"}]}}`)
				case r.Method == http.MethodGet && r.URL.Path == "/traeger/finance-exports/runs/care-runs/5/datev":
					downloads++
					w.WriteHeader(scenario.status)
					_, _ = fmt.Fprint(w, scenario.body)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
			})
			defer srv.Close()

			err := seedDatevFile(newTestClient(srv.URL, false), AuthRef{Kind: AuthBearer, Token: "traeger"}, calendar.NewDate(2026, 11, 1))
			if scenario.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, 1, downloads)
		})
	}
}
