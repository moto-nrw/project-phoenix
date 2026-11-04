package api

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// payrollRulesSeedServer answers the payroll rule routes like the backend:
// created wage types come back in the list.
type payrollRulesSeedServer struct {
	mu        sync.Mutex
	wageTypes []string
	rules     []map[string]any
	mapped    map[string]string
	auths     []string
}

func newPayrollRulesSeedServer(t *testing.T, existing []string) (*payrollRulesSeedServer, string) {
	t.Helper()
	server := &payrollRulesSeedServer{wageTypes: existing, mapped: map[string]string{}}
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		server.mu.Lock()
		defer server.mu.Unlock()
		server.auths = append(server.auths, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch {
		case r.Method == "GET" && r.URL.Path == payrollRulesPath:
			wageTypes := []map[string]string{}
			for i, name := range server.wageTypes {
				wageTypes = append(wageTypes, map[string]string{"id": fmt.Sprint(i + 1), "name": name})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"wage_types":    wageTypes,
				"absence_types": []map[string]any{{"id": "40", "name": "Regenerationstag", "wage_type_id": nil}},
			}})
		case r.Method == "POST" && r.URL.Path == payrollRulesPath+"/wage-types":
			server.wageTypes = append(server.wageTypes, fmt.Sprint(body["name"]))
			_, _ = fmt.Fprintf(w, `{"data":{"id":"%d"}}`, len(server.wageTypes))
		case r.Method == "POST" && r.URL.Path == payrollRulesPath+"/surcharge-rules":
			server.rules = append(server.rules, body)
			_, _ = fmt.Fprint(w, `{"data":{"id":"90"}}`)
		case r.Method == "PUT" && r.URL.Path == payrollRulesPath+"/absence-types/40":
			server.mapped["40"] = fmt.Sprint(body["wage_type_id"])
			_, _ = fmt.Fprint(w, `{"data":null}`)
		default:
			w.WriteHeader(404)
		}
	})
	return server, srv.URL
}

func TestSeedPayrollRulesStep_CreatesWageTypesRulesAndTheMapping(t *testing.T) {
	t.Parallel()

	server, url := newPayrollRulesSeedServer(t, nil)
	rt := &Runtime{Client: newTestClient(url, false), TenantAuth: AuthRef{Kind: AuthBearer, Token: "tenant-token"}}

	require.NoError(t, seedPayrollRulesStep{}.Run(context.Background(), rt))

	assert.Equal(t, []string{"Nachtzuschlag", "Sonntagszuschlag", "Feiertagszuschlag", "Regenerationstag"}, server.wageTypes)
	require.Len(t, server.rules, 3)
	assert.Equal(t, "nacht", server.rules[0]["kind"])
	assert.Equal(t, "1", server.rules[0]["wage_type_id"], "the night rule books onto the night wage type")
	assert.Equal(t, "2", server.rules[1]["wage_type_id"])
	assert.Empty(t, server.mapped)

	require.NoError(t, seedPayrollAbsenceTypeMappingStep{}.Run(context.Background(), rt))
	assert.Equal(t, map[string]string{"40": "4"}, server.mapped, "the Regenerationstag books onto its day wage type")
	for _, auth := range server.auths {
		assert.Equal(t, "Bearer tenant-token", auth)
	}
}

func TestSeedPayrollRulesStep_KeepsTheWageTypesOfASecondRun(t *testing.T) {
	t.Parallel()

	server, url := newPayrollRulesSeedServer(t, []string{"Nachtzuschlag"})
	rt := &Runtime{Client: newTestClient(url, false), TenantAuth: AuthRef{Kind: AuthBearer, Token: "tenant-token"}}

	require.NoError(t, seedPayrollRulesStep{}.Run(context.Background(), rt))

	assert.Equal(t, []string{"Nachtzuschlag"}, server.wageTypes)
	assert.Empty(t, server.rules)
	require.NoError(t, seedPayrollAbsenceTypeMappingStep{}.Run(context.Background(), rt))
	assert.Empty(t, server.mapped)
}

func TestPayrollRuleMappingWaitsForDeferredTimeTrackingHistory(t *testing.T) {
	t.Parallel()

	index := func(steps []Step, match func(Step) bool) int {
		for i, step := range steps {
			if match(step) {
				return i
			}
		}
		return -1
	}
	isType := func(want Step) func(Step) bool {
		return func(got Step) bool { return reflect.TypeOf(got) == reflect.TypeOf(want) }
	}

	complete := fullDemoWorkflow(&Seeder{}).Steps
	assert.Less(t, index(complete, isType(seedTimeTrackingHistoryStep{})), index(complete, isType(seedPayrollAbsenceTypeMappingStep{})))

	deferred := &Seeder{options: SeedOptions{DeferHistory: true}}
	opening := fullDemoWorkflow(deferred).Steps
	assert.NotEqual(t, -1, index(opening, isType(seedPayrollRulesStep{})))
	assert.Equal(t, -1, index(opening, isType(seedPayrollAbsenceTypeMappingStep{})))
	assert.Less(t, index(deferredDemoSteps(deferred), isType(seedTimeTrackingHistoryStep{})), index(deferredDemoSteps(deferred), isType(seedPayrollAbsenceTypeMappingStep{})))
}
