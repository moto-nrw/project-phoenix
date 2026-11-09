package enrollmenthttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// History and public case lists must neither mark requests read nor mutate
// records or consume sequences, including through a database-side effect.
func growthReadOnlyGET(t *testing.T, f *enrollmentGrowthFixture, router http.Handler, counter *testpkg.QueryCounter, token, path string) (json.RawMessage, testpkg.QueryMeasurement) {
	t.Helper()
	before := enrollmentReadState(t, f)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	counter.Reset()
	testpkg.MeasuredHTTPRequest(counter, testpkg.MountedQueryBudgetRouter("/api/enrollment", router), rec, req)
	queries := counter.Queries()
	require.Equal(t, before, enrollmentReadState(t, f), "read graph changed database state")
	for _, query := range queries {
		upper := strings.ToUpper(strings.TrimSpace(query))
		require.False(t, strings.HasPrefix(upper, "INSERT ") || strings.HasPrefix(upper, "UPDATE ") || strings.HasPrefix(upper, "DELETE ") || strings.Contains(upper, "NEXTVAL("), query)
	}
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// A private, opt-in complete wire capture supports the implementation's
	// baseline/candidate replay; ordinary runs need no artifact directory.
	if dir := os.Getenv("PHX_ENROLLMENT_READ_CAPTURE"); dir != "" {
		var body struct {
			Data []json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		name := "student-history"
		if strings.Contains(path, "change-requests") {
			name = "public-changes"
		}
		raw, err := json.MarshalIndent(struct {
			Status  int
			Headers http.Header
			Body    json.RawMessage
		}{rec.Code, rec.Header(), rec.Body.Bytes()}, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-N%d.json", name, len(body.Data))), raw, 0o600))
	}
	var body struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEmpty(t, body.Data)
	return body.Data, counter.Measurement(queries)
}

func enrollmentReadState(t *testing.T, f *enrollmentGrowthFixture) map[string]string {
	t.Helper()
	ctx := testpkg.Ctx(t)
	state := map[string]string{}
	for _, table := range []string{"enrollment.requests", "enrollment.request_children", "enrollment.request_guardians", "enrollment.change_requests", "enrollment.change_request_messages", "enrollment.request_reads", "enrollment.care_offering_bookings"} {
		var data string
		require.NoError(t, f.env.db.NewRaw("SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text), '[]'::jsonb)::text FROM "+table+" AS r WHERE tenant_id = ?", testpkg.Tenant(t)).Scan(ctx, &data))
		state[table] = data
	}
	var sequences string
	require.NoError(t, f.env.db.NewRaw("SELECT COALESCE(jsonb_agg(to_jsonb(s) ORDER BY schemaname, sequencename),'[]'::jsonb)::text FROM pg_sequences AS s WHERE schemaname NOT LIKE 'pg_%'").Scan(ctx, &sequences))
	state["sequences"] = sequences
	return state
}
