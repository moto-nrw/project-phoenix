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
					assert.Equal(t, float64(81), body["form_schema_id"], "the submission names the form version it showed")
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

// The Träger drafts, signs and bills the contracts of the seeded school's
// accepted applications. A demo order (DeferHistory) adds its applications
// and bookings only after the school opens, so its contract and billing
// steps follow them there, as in a full seed (#6621).
func TestTraegerCareStepsFollowTheApplicationsOfTheSchool(t *testing.T) {
	t.Parallel()
	for _, options := range []SeedOptions{{}, {DeferHistory: true}} {
		seeder := &Seeder{options: options}
		steps := fullDemoWorkflow(seeder).Steps
		if options.DeferHistory {
			steps = append(steps, deferredDemoSteps(seeder)...)
		}
		at := map[string][]int{}
		for index, step := range steps {
			if serial, ok := step.(traegerSerialStep); ok {
				step = serial.Step
			}
			switch step.(type) {
			case parentRequestsSeedStep:
				at["requests"] = append(at["requests"], index)
			case seedRegularBookingsStep:
				at["bookings"] = append(at["bookings"], index)
			case seedTraegerCareContractsStep:
				at["contracts"] = append(at["contracts"], index)
			case seedTraegerCareContractSignaturesStep:
				at["signatures"] = append(at["signatures"], index)
			case seedTraegerCareMonthsStep:
				at["months"] = append(at["months"], index)
			}
		}
		for _, key := range []string{"requests", "bookings", "contracts", "signatures", "months"} {
			require.Len(t, at[key], 1, "defer %t: %s runs once", options.DeferHistory, key)
		}
		assert.Greater(t, at["contracts"][0], at["requests"][0], "defer %t: the applications precede their contracts", options.DeferHistory)
		assert.Greater(t, at["contracts"][0], at["bookings"][0], "defer %t: the booked care precedes the contracts", options.DeferHistory)
		assert.Greater(t, at["signatures"][0], at["contracts"][0], "defer %t", options.DeferHistory)
		assert.Greater(t, at["months"][0], at["signatures"][0], "defer %t: the months bill concluded contracts", options.DeferHistory)
	}
}
