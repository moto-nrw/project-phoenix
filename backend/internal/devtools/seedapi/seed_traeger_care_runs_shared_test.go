package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// seedSharedTraegerRuns is the demo Träger after earlier demo orders: an
// open October run of other schools with nothing left to bill, and a
// released October run whose invoices went with their erased demo school.
// Like the server, it bills a run only for its schools and refuses the
// DATEV file of a run without an invoice.
type seedSharedTraegerRuns struct {
	mu       sync.Mutex
	runs     []seedCareRunRef
	preview  map[int64]int
	started  [][]int64
	released []int64
	exported []int64
}

func (f *seedSharedTraegerRuns) serve(t *testing.T) func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
	return func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		path := r.Method + " " + r.URL.Path
		runID := seedRunIDOf(r.URL.Path)
		switch {
		case r.Method == http.MethodPut && (strings.HasPrefix(r.URL.Path, "/traeger/billing-terms/") ||
			strings.HasPrefix(r.URL.Path, "/traeger/billing-run-due-dates/") || strings.HasPrefix(r.URL.Path, "/traeger/settings/values/") ||
			r.URL.Path == "/traeger/sepa-creditor/" || strings.HasSuffix(r.URL.Path, "/invoice-date")):
			_, _ = fmt.Fprint(w, `{"data":{}}`)
		case path == "GET /traeger/care-billing-runs/":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"runs": f.runs}}))
		case path == "POST /traeger/care-billing-runs/":
			var start struct {
				BillingMonth calendar.Date `json:"billing_month"`
				SchoolIDs    []int64       `json:"school_ids"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&start))
			f.started = append(f.started, start.SchoolIDs)
			run := seedCareRunRef{ID: int64(10 + len(f.started)), BillingMonth: start.BillingMonth, Status: "calculated", SchoolIDs: start.SchoolIDs}
			f.runs = append([]seedCareRunRef{run}, f.runs...)
			f.preview[run.ID] = 2
			_, _ = fmt.Fprintf(w, `{"data":{"id":%d}}`, run.ID)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/release"):
			f.released = append(f.released, runID)
			for i := range f.runs {
				if f.runs[i].ID == runID {
					f.runs[i].Status = "released"
				}
			}
			_, _ = fmt.Fprint(w, `{"data":{"status":"released"}}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/datev"):
			if f.preview[runID] == 0 {
				w.WriteHeader(http.StatusConflict)
				_, _ = fmt.Fprint(w, `{"status":409,"code":"parent_billing.export_run_empty","detail":"the run has no invoice to export"}`)
				return
			}
			f.exported = append(f.exported, runID)
			_, _ = fmt.Fprint(w, "EXTF;700")
		case r.Method == http.MethodGet && runID != 0:
			_, _ = fmt.Fprintf(w, `{"data":{"id":%d,"status":"calculated","preview_count":%d,"preview_cents":9100,"fingerprint":"f","invoices":[]}}`,
				runID, f.preview[runID])
		default:
			t.Errorf("unexpected request %s", path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// seedRunIDOf is the run ID in a run path, 0 without one.
func seedRunIDOf(path string) int64 {
	var id int64
	for _, part := range strings.Split(path, "/") {
		if _, err := fmt.Sscan(part, &id); err == nil {
			return id
		}
	}
	return 0
}

// A demo school ordered after others bills its own contracts in its own
// runs and exports its own run to DATEV, beside the leftover runs of the
// other demo schools (demo order failure of 10 November 2026:
// parent_billing.export_run_empty on run 3).
func TestSeedCareRunsBillTheSeededSchoolBesideLeftoverRunsOfOtherDemoSchools(t *testing.T) {
	t.Parallel()

	const seeded = int64(65)
	october, november := calendar.NewDate(2026, 10, 1), calendar.NewDate(2026, 11, 1)
	traeger := &seedSharedTraegerRuns{
		runs: []seedCareRunRef{
			{ID: 4, BillingMonth: october, Status: "calculated", SchoolIDs: []int64{53, 59, 60}},
			{ID: 3, BillingMonth: october, Status: "released", SchoolIDs: []int64{53, 59}},
		},
		preview: map[int64]int{4: 0, 3: 0},
	}
	srv := newSeedHTTPTestServer(traeger.serve(t))
	defer srv.Close()
	client, auth := newTestClient(srv.URL, false), AuthRef{Kind: AuthBearer, Token: "traeger"}

	issued, err := seedCareRuns(client, auth, seeded, october, november)
	require.NoError(t, err)

	assert.Equal(t, 2, issued)
	assert.Equal(t, [][]int64{{seeded}, {seeded}}, traeger.started, "October and November runs for the seeded school alone")
	assert.Equal(t, []int64{11}, traeger.released, "the seeded school's October run, not the leftover run 4")
	assert.Equal(t, []int64{11}, traeger.exported, "the DATEV file of the school's own run, not of the empty run 3")
	assert.False(t, slices.ContainsFunc(traeger.runs, func(run seedCareRunRef) bool { return run.ID == 4 && run.Status != "calculated" }))
}

// The first billed month is the seeded school's, not the earliest contract
// of the whole Träger.
func TestSeedFirstActiveMonthReadsTheContractsOfTheSeededSchool(t *testing.T) {
	t.Parallel()

	var query string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{"contracts":[{"effective_from":"2026-12-01"},{"effective_from":"2026-10-15"},{"effective_from":"2026-11-01"}]}}`)
	})
	defer srv.Close()

	client, auth := newTestClient(srv.URL, false), AuthRef{Kind: AuthBearer, Token: "traeger"}
	first, err := seedFirstActiveMonth(client, auth, 65, calendar.NewDate(2026, 11, 1))
	require.NoError(t, err)
	assert.Equal(t, calendar.NewDate(2026, 10, 1), first)
	assert.Equal(t, "status=aktiv&school_id=65", query)

	// A contract that begins after the current month, such as an imported
	// one (#4723), has no month to bill: alone it gives no first month.
	first, err = seedFirstActiveMonth(client, auth, 65, calendar.NewDate(2026, 9, 1))
	require.NoError(t, err)
	assert.True(t, first.IsZero())
}

// While another school's notice run of the year is open, the seeded school
// gets no notices instead of failing its order.
func TestSeedStartNoticeRunSkipsTheOpenRunOfOtherSchools(t *testing.T) {
	t.Parallel()

	var started map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /traeger/care-notice-runs/":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&started))
			w.WriteHeader(http.StatusConflict)
			_, _ = fmt.Fprint(w, `{"status":409,"code":"parent_billing.care_notice_run_open","detail":"open"}`)
		case "GET /traeger/care-notice-runs/":
			_, _ = fmt.Fprint(w, `{"data":{"runs":[{"id":4,"year":2026,"status":"calculated","school_ids":[53,61]}]}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	runID, err := seedStartNoticeRun(newTestClient(srv.URL, false), AuthRef{Kind: AuthBearer, Token: "traeger"}, 65, calendar.NewDate(2026, 12, 1))
	require.NoError(t, err)
	assert.Zero(t, runID)
	assert.Equal(t, []any{float64(65)}, started["school_ids"])
}

// Cost bearer shares go onto contracts of the seeded school only.
func TestSeedCareCostSharesLeaveTheContractsOfOtherSchools(t *testing.T) {
	t.Parallel()

	var shared []float64
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /traeger/care-cost-shares/":
			_, _ = fmt.Fprint(w, `{"data":{"contracts":[`+
				`{"contract_id":1,"school_id":53,"shares":[]},{"contract_id":2,"school_id":53,"shares":[]},`+
				`{"contract_id":3,"school_id":53,"shares":[]},{"contract_id":4,"school_id":53,"shares":[]},`+
				`{"contract_id":11,"school_id":65,"shares":[]},{"contract_id":12,"school_id":65,"shares":[]},`+
				`{"contract_id":13,"school_id":65,"shares":[]},{"contract_id":14,"school_id":65,"shares":[]}],"cost_bearers":[]}}`)
		case "POST /traeger/cost-bearers/":
			_, _ = fmt.Fprint(w, `{"data":{"id":9}}`)
		case "POST /traeger/care-cost-shares/":
			var share map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&share))
			shared = append(shared, share["contract_id"].(float64))
			_, _ = fmt.Fprint(w, `{"data":{}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	require.NoError(t, seedCareCostShares(newTestClient(srv.URL, false), AuthRef{Kind: AuthBearer, Token: "traeger"}, 65, calendar.NewDate(2026, 10, 1)))
	assert.Equal(t, []float64{13, 14}, shared)
}

// A notice run the seed cannot release would hold the year for every demo
// school ordered later (demo notice run 4 of 2026, #6641): the seed discards
// it, whether its preview is empty or the release refuses it as empty.
func TestSeedCareNoticesDiscardTheOwnRunTheyCannotRelease(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		preview int
		release int
	}{
		{name: "empty preview, no release", preview: 0, release: 0},
		{name: "release refused as empty", preview: 3, release: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var released, discarded int
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "GET /traeger/master-data/document-templates/bescheid":
					_, _ = fmt.Fprint(w, `{"data":{"current":{"standard":false}}}`)
				case "POST /traeger/care-notice-runs/":
					_, _ = fmt.Fprint(w, `{"data":{"id":4}}`)
				case "GET /traeger/care-notice-runs/4":
					_, _ = fmt.Fprintf(w, `{"data":{"id":4,"year":2026,"status":"calculated","school_ids":[65],"preview_count":%d,"preview_cents":9100,"fingerprint":"f"}}`, tc.preview)
				case "POST /traeger/care-notice-runs/4/release":
					released++
					w.WriteHeader(http.StatusConflict)
					_, _ = fmt.Fprint(w, `{"status":409,"code":"parent_billing.billing_run_empty","detail":"empty"}`)
				case "POST /traeger/care-notice-runs/4/discard":
					discarded++
					_, _ = fmt.Fprint(w, `{"data":{"id":4,"status":"discarded"}}`)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
			})
			defer srv.Close()

			issued, err := seedCareNotices(newTestClient(srv.URL, false), AuthRef{Kind: AuthBearer, Token: "traeger"}, 65, calendar.NewDate(2026, 10, 1))
			require.NoError(t, err)
			assert.Zero(t, issued)
			assert.Equal(t, tc.release, released)
			assert.Equal(t, 1, discarded, "the run frees the year for later demo schools")
		})
	}
}

// A run without notices but with a booking without a price stays open for
// the office; the order still succeeds.
func TestSeedCareNoticesLeaveARunWithSomethingToSettleOpen(t *testing.T) {
	t.Parallel()

	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /traeger/master-data/document-templates/bescheid":
			_, _ = fmt.Fprint(w, `{"data":{"current":{"standard":false}}}`)
		case "POST /traeger/care-notice-runs/":
			_, _ = fmt.Fprint(w, `{"data":{"id":4}}`)
		case "GET /traeger/care-notice-runs/4":
			_, _ = fmt.Fprint(w, `{"data":{"id":4,"year":2026,"status":"calculated","school_ids":[65],"preview_count":0}}`)
		case "POST /traeger/care-notice-runs/4/discard":
			w.WriteHeader(http.StatusConflict)
			_, _ = fmt.Fprint(w, `{"status":409,"code":"parent_billing.billing_run_not_empty","detail":"not empty"}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	issued, err := seedCareNotices(newTestClient(srv.URL, false), AuthRef{Kind: AuthBearer, Token: "traeger"}, 65, calendar.NewDate(2026, 10, 1))
	require.NoError(t, err)
	assert.Zero(t, issued)
}
