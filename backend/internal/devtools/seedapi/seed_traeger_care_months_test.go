package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The demo queue seeds several schools of one Träger at once (CI "Seed and
// simulate smoke (queue)"): another seed starts the month's run between this
// seed's look and its start, and releases it before this seed does. The seed
// takes that run instead of failing the school.
func TestSeedCareRunTakesTheRunAnotherSeedOfTheTraegerStartedAndReleased(t *testing.T) {
	t.Parallel()

	lists := 0
	var released bool
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /traeger/care-billing-runs/":
			lists++
			if lists == 1 {
				_, _ = fmt.Fprint(w, `{"status":"success","data":{"runs":[]}}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"runs":[{"id":5,"billing_month":"2026-11-01","status":"calculated"}]}}`)
		case "POST /traeger/care-billing-runs/":
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(409)
			_, _ = fmt.Fprint(w, `{"status":409,"code":"parent_billing.care_run_open","detail":"the month already has a billing run that is not released"}`)
		case "PUT /traeger/billing-terms/care", "PUT /traeger/billing-terms/meal", "PUT /traeger/billing-terms/holiday",
			"PUT /traeger/billing-run-due-dates/care/5", "PUT /traeger/sepa-creditor/":
			// The payment terms and due dates the run's seed sets (#4689),
			// and the SEPA creditor (#4695).
			_, _ = fmt.Fprint(w, `{"status":"success","data":{}}`)
		case "GET /traeger/care-billing-runs/5":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":5,"status":"calculated","preview_count":2,"preview_cents":9100}}`)
		case "POST /traeger/care-billing-runs/5/release":
			released = true
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(409)
			_, _ = fmt.Fprint(w, `{"status":409,"code":"parent_billing.billing_run_released","detail":"the billing run is released and does not change"}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	issued, err := seedCareRun(newTestClient(srv.URL, false), AuthRef{Kind: AuthBearer, Token: "traeger"}, calendar.NewDate(2026, 11, 1), true)
	require.NoError(t, err)
	assert.Zero(t, issued, "the other seed issued the invoices")
	assert.Equal(t, 2, lists, "the seed reads the runs again after the conflict")
	assert.True(t, released, "the seed tried to release the run it took")
}

// Any other refusal still fails the school.
func TestSeedCareRunKeepsOtherRefusals(t *testing.T) {
	t.Parallel()

	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /traeger/care-billing-runs/":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"runs":[]}}`)
		case "POST /traeger/care-billing-runs/":
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(422)
			_, _ = fmt.Fprint(w, `{"status":422,"code":"general.invalid_request","detail":"invalid billing month"}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	_, err := seedCareRun(newTestClient(srv.URL, false), AuthRef{Kind: AuthBearer, Token: "traeger"}, calendar.NewDate(2026, 11, 1), true)
	require.ErrorContains(t, err, "start the monthly run of 2026-11-01")
}
