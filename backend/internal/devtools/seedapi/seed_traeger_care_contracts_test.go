package api

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedConclusionSubmitsConditionsBeforeApproval(t *testing.T) {
	t.Parallel()
	for _, refuseSubmission := range []bool{false, true} {
		t.Run(fmt.Sprintf("submission_refused_%t", refuseSubmission), func(t *testing.T) {
			var calls []string
			const text = "Vertragsbedingungen für die Betreuung\n\nGespeicherte Bedingungen"
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				calls = append(calls, r.Method+" "+r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				var body map[string]any
				if r.Method != "GET" {
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				}
				switch r.Method + " " + r.URL.Path {
				case "GET /traeger/contract-templates/":
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
						"school_id": 42, "offering_id": 71, "acceptance_text": text,
					}}})
				case "POST /api/enrollment/schema/":
					assert.Empty(t, body["fields"], "the contract form asks no custom questions")
					blocks := body["legal_blocks"].([]any)
					require.Len(t, blocks, 1)
					block := blocks[0].(map[string]any)
					assert.Equal(t, "terms", block["kind"])
					assert.Equal(t, text, block["text"])
					assert.Equal(t, true, block["required"])
					assert.Equal(t, true, block["enabled"])
					assert.NotContains(t, block, "text_sha256")
					_, _ = fmt.Fprint(w, `{"data":{"id":"81"}}`)
				case "PUT /api/enrollment/phases/61":
					if len(calls) == 4 {
						assert.Equal(t, "81", body["form_schema_id"])
					} else {
						assert.Equal(t, "60", body["form_schema_id"], "restore the original form even after a failed submit")
					}
					_, _ = fmt.Fprint(w, `{"data":{}}`)
				case "POST /api/enrollment/demo/submit":
					assert.NotContains(t, body, "custom_data")
					children := body["children"].([]any)
					require.Len(t, children, 1)
					assert.NotContains(t, children[0].(map[string]any), "custom_data")
					assert.Equal(t, true, body["consent_flags"].(map[string]any)["care_contract"])
					assert.NotContains(t, body, "legal_blocks_snapshot")
					if refuseSubmission {
						w.WriteHeader(400)
						_, _ = fmt.Fprint(w, `{"detail":"submission refused"}`)
						return
					}
					_, _ = fmt.Fprint(w, `{"data":{"request_id":"91","status_url":"/status/token"}}`)
				case "GET /api/enrollment/admin/requests/91":
					_, _ = fmt.Fprint(w, `{"data":{"children":[{"id":"92"}]}}`)
				case "POST /api/enrollment/admin/requests/91/children/92/decide":
					assert.Equal(t, "approved", body["status"])
					_, _ = fmt.Fprint(w, `{"data":{}}`)
				case "PUT /operator/schools/42/settings/values/care_contract.conclusion_mode":
					assert.Equal(t, "bei_annahme", body["value"])
					_, _ = fmt.Fprint(w, `{"data":{}}`)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			})
			defer srv.Close()
			rt := &Runtime{
				Client:      newTestClient(srv.URL, false),
				Bootstrap:   &bootstrapSeedState{SchoolID: 42, TenantSlug: "demo"},
				FixedSeeder: &FixedSeeder{accountScope: "demo"},
				Enrollment:  SeedEnrollmentState{PhaseID: 61, Offerings: map[string]int64{"ogs-ganztag": 71}},
				Values: map[string]any{
					enrollmentPhaseBodyKey: map[string]any{"form_schema_id": "60"},
					seedTraegerSessionKey:  seedTraegerSessionRef{},
				},
			}
			err := seedConclusionAtAcceptance(rt, 42)
			if refuseSubmission {
				require.ErrorContains(t, err, "submission refused")
				assert.Empty(t, rt.Enrollment.Requests)
			} else {
				require.NoError(t, err)
				require.Len(t, rt.Enrollment.Requests, 1)
				assert.Equal(t, "approved", rt.Enrollment.Requests[0].Status)
			}
			assert.Equal(t, "PUT /api/enrollment/phases/61", calls[len(calls)-1])
			assert.Contains(t, calls, "POST /api/enrollment/demo/submit")
			want := []string{
				"PUT /operator/schools/42/settings/values/care_contract.conclusion_mode",
				"GET /traeger/contract-templates/",
				"POST /api/enrollment/schema/",
				"PUT /api/enrollment/phases/61",
				"POST /api/enrollment/demo/submit",
			}
			if !refuseSubmission {
				want = append(want, "GET /api/enrollment/admin/requests/91", "POST /api/enrollment/admin/requests/91/children/92/decide")
			}
			want = append(want, "PUT /api/enrollment/phases/61")
			assert.Equal(t, want, calls)
		})
	}
}

func TestSeedCareContractTemplateNamesTheContractPartner(t *testing.T) {
	t.Parallel()
	assert.NotEmpty(t, seedCareContractTemplate["contract_partner"])
}
