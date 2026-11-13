package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A second seed run on the same database (CI seeds the standing demo school
// after the profiles) saves the recipients of the earlier run again instead
// of adding a recipient whose name the Träger already has (#6911, ADR 0221).
func TestSeedExportHandoverSavesTheRecipientsOfAnEarlierRun(t *testing.T) {
	t.Parallel()

	var calls []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && r.URL.Path == "/traeger/export-recipient/":
			_, _ = w.Write([]byte(`{"status":"success","data":{"recipients":[{"id":4,"name":"Kämmerei"}]}}`))
		case r.URL.Path == "/traeger/export-recipient/4":
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":4}}`))
		case r.Method == "POST" && r.URL.Path == "/traeger/export-recipient/":
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":5}}`))
		default:
			_, _ = w.Write([]byte(`{"status":"success","data":{"state":"failed","reason":"address_denied"}}`))
		}
	})
	defer srv.Close()

	require.NoError(t, seedExportHandover(newTestClient(srv.URL, false), AuthRef{Kind: AuthBearer, Token: "traeger-token"}, 9))
	assert.Equal(t, []string{
		"GET /traeger/export-recipient/",
		"PUT /traeger/export-recipient/4",
		"POST /traeger/export-recipient/",
		"PUT /traeger/export-recipient/formats",
		"POST /traeger/finance-handovers/runs/holiday-runs/9/sollstellung",
	}, calls)
}
