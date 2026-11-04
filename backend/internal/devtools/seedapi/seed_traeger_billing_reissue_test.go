package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestSeedBillingReissueCancelsTheSecondInvoiceAndDecidesAProposal(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var calls []string
	var cancellation map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /traeger/auth/login":
			_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"traeger-token","refresh_token":"r"}`))
		case "GET /traeger/billing-runs/41":
			_, _ = w.Write([]byte(`{"data":{"holiday_period_id":5,"documents":[
				{"id":7,"kind":"invoice","children":[{"positions":[{"kind":"base"},{"kind":"correction"}]}]},
				{"id":8,"kind":"invoice","children":[{"positions":[{"kind":"base"},{"kind":"correction"},{"kind":"correction"}]}]}]}}`))
		case "POST /traeger/billing-runs/41/documents/8/cancellation":
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&cancellation))
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"success"}`))
		case "POST /traeger/billing-runs/":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"data":{"id":42}}`))
		case "GET /traeger/billing-runs/42":
			_, _ = w.Write([]byte(`{"data":{"holiday_period_id":5,"documents":[
				{"kind":"invoice","reference_number":"FE-2","children":[{"carried_corrections":[{"position_id":61},{"position_id":62}]}]}]}}`))
		default:
			_, _ = w.Write([]byte(`{"status":"success"}`))
		}
	})
	defer srv.Close()

	runtime := &Runtime{
		Client:    newTestClient(srv.URL, false),
		Bootstrap: &bootstrapSeedState{AdminEmail: "admin@example.test", AdminPassword: testpkg.NewTestPassword()},
		Values:    map[string]any{seedBillingRunKey: int64(41)},
	}
	require.NoError(t, (seedTraegerBillingReissueStep{}).Run(context.Background(), runtime))
	assert.Equal(t, []string{
		"POST /traeger/auth/login",
		"GET /traeger/billing-runs/41",
		"POST /traeger/billing-runs/41/documents/8/cancellation",
		"POST /traeger/billing-runs/",
		"GET /traeger/billing-runs/42",
		"DELETE /traeger/billing-runs/42/carried-corrections/61",
	}, calls, "the first family keeps its invoice; the next run keeps one proposal")
	assert.Equal(t, seedReissueReason, cancellation["reason"])

	runtime.Values = map[string]any{}
	assert.ErrorContains(t, (seedTraegerBillingReissueStep{}).Run(context.Background(), runtime), "released holiday run")
}
