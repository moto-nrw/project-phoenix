package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// seedCareTraeger is a Träger as the public demo keeps it: the placeholder
// rules of an earlier demo school, and contracts of that school and of the
// school now seeding. Like the server, it refuses an income level that does
// not apply at the school of the contract.
type seedCareTraeger struct {
	mu        sync.Mutex
	schools   []int64
	rules     []seedCareRule
	contracts map[int64]int64 // contract → school
	order     []int64
	evidences map[int64][]map[string]any
	posted    []seedCareRule
}

func (f *seedCareTraeger) serve(t *testing.T) func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
	return func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		write := func(status int, body any) {
			w.WriteHeader(status)
			require.NoError(t, json.NewEncoder(w).Encode(body))
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /traeger/care-rules/":
			schools := make([]map[string]int64, len(f.schools))
			for i, id := range f.schools {
				schools[i] = map[string]int64{"id": id}
			}
			write(http.StatusOK, map[string]any{"data": map[string]any{"rules": f.rules, "schools": schools}})
		case "POST /traeger/care-rules/":
			var rule seedCareRule
			require.NoError(t, json.NewDecoder(r.Body).Decode(&rule))
			rule.ID = int64(len(f.rules) + 1)
			f.rules = append(f.rules, rule)
			f.posted = append(f.posted, rule)
			write(http.StatusCreated, map[string]any{"data": map[string]any{}})
		case "GET /traeger/care-evidences/":
			contracts := []map[string]any{}
			for _, id := range f.order {
				contracts = append(contracts, map[string]any{"contract_id": id, "school_id": f.contracts[id], "evidences": f.evidences[id]})
			}
			levels := []seedCareRule{}
			for _, rule := range f.rules {
				if rule.Kind == "income_level" {
					levels = append(levels, rule)
				}
			}
			write(http.StatusOK, map[string]any{"data": map[string]any{"contracts": contracts, "levels": levels}})
		case "POST /traeger/care-evidences/":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			contract := int64(body["contract_id"].(float64))
			if ruleID, ok := body["rule_id"].(float64); ok {
				for _, rule := range f.rules {
					if rule.ID == int64(ruleID) && !slices.Contains(rule.SchoolIDs, f.contracts[contract]) {
						write(http.StatusBadRequest, map[string]any{"status": 400, "code": "parent_billing.care_evidence_invalid",
							"detail": "rule_id: does not apply at the school of the contract."})
						return
					}
				}
			}
			f.evidences[contract] = append(f.evidences[contract], body)
			write(http.StatusCreated, map[string]any{"data": map[string]any{}})
		case "GET /traeger/care-cost-shares/":
			write(http.StatusOK, map[string]any{"data": map[string]any{"contracts": []any{}}})
		case "GET /traeger/family-children/":
			write(http.StatusOK, map[string]any{"data": map[string]any{"families": []any{}}})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// A demo school ordered after the Träger's first seed falls under none of
// its rules. Its seed must still pass, keep the rules for its own school and
// write evidences only onto its own contracts (demo order failure of
// 10 November 2026).
func TestSeedCareReductionsCoverALaterDemoSchoolOfTheTraeger(t *testing.T) {
	t.Parallel()

	const earlier, seeded = int64(10), int64(20)
	var rules []seedCareRule
	for _, kind := range []string{"income_level", "income_level", "sibling", "single_parent", "cap"} {
		rules = append(rules, seedCareRule{ID: int64(len(rules) + 1), Kind: kind, SchoolIDs: []int64{earlier}})
	}
	traeger := &seedCareTraeger{
		schools:   []int64{earlier, seeded},
		rules:     rules,
		contracts: map[int64]int64{101: seeded, 102: seeded, 201: earlier},
		// The evidence page sorts by school name: the new school may come first.
		order:     []int64{101, 102, 201},
		evidences: map[int64][]map[string]any{},
	}
	srv := newSeedHTTPTestServer(traeger.serve(t))
	defer srv.Close()

	_, evidences, err := seedCareReductions(newTestClient(srv.URL, false), AuthRef{Kind: AuthBearer, Token: "traeger"}, seeded, calendar.NewDate(2026, 11, 1))
	require.NoError(t, err)

	assert.Equal(t, 2, evidences)
	require.Len(t, traeger.posted, len(seedCareRules))
	for _, rule := range traeger.posted {
		assert.Equal(t, []int64{seeded}, rule.SchoolIDs, "rule %s", rule.Kind)
	}
	assert.Equal(t, "income_level", traeger.evidences[101][0]["kind"])
	assert.Equal(t, "single_parent", traeger.evidences[102][0]["kind"])
	assert.Empty(t, traeger.evidences[201], "the contract of another demo school stays untouched")
}

// A Träger without rules gets them for every school, as on a dev machine.
func TestSeedCareReductionsKeepTheRulesForEverySchoolOfANewTraeger(t *testing.T) {
	t.Parallel()

	traeger := &seedCareTraeger{
		schools:   []int64{10, 20},
		contracts: map[int64]int64{101: 10},
		order:     []int64{101},
		evidences: map[int64][]map[string]any{},
	}
	srv := newSeedHTTPTestServer(traeger.serve(t))
	defer srv.Close()

	_, evidences, err := seedCareReductions(newTestClient(srv.URL, false), AuthRef{Kind: AuthBearer, Token: "traeger"}, 10, calendar.NewDate(2026, 11, 1))
	require.NoError(t, err)

	assert.Equal(t, 1, evidences)
	require.Len(t, traeger.posted, len(seedCareRules))
	for _, rule := range traeger.posted {
		assert.Equal(t, []int64{10, 20}, rule.SchoolIDs, "rule %s", rule.Kind)
	}
	assert.Equal(t, "income_level", traeger.evidences[101][0]["kind"])
}
