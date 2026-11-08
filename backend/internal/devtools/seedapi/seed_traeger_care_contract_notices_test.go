package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedCareContractNoticesFillsMissingCasesOnce(t *testing.T) {
	t.Parallel()
	for _, setup := range []struct{ ordinaryExists, draftsOnly bool }{{}, {ordinaryExists: true}, {draftsOnly: true}} {
		ordinaryExists := setup.ordinaryExists
		t.Run(fmt.Sprintf("ordinary_exists_%t_drafts_%t", ordinaryExists, setup.draftsOnly), func(t *testing.T) {
			notices := map[string]map[string]any{}
			if ordinaryExists {
				notices["3"] = map[string]any{"termination_kind": "ordentlich"}
			}
			var writes []string
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				w.Header().Set("Content-Type", "application/json")
				respond := func(data any) { _ = json.NewEncoder(w).Encode(map[string]any{"data": data}) }
				if r.Method == "GET" && r.URL.Path == "/traeger/contracts/" {
					contracts := []map[string]any{}
					status := r.URL.Query().Get("status")
					if (setup.draftsOnly && status == "aktiv") || (!setup.draftsOnly && status == "entwurf") {
						respond(map[string]any{"contracts": contracts})
						return
					}
					for id := 1; id <= 6; id++ {
						_, terminated := notices[fmt.Sprint(id)]
						if terminated != (r.URL.Query().Get("status") == "gekuendigt") {
							continue
						}
						row := map[string]any{"id": id}
						if id == 1 {
							row["child_first_name"], row["child_last_name"] = "Lina", "Amini"
						}
						if id == 2 {
							row["child_first_name"], row["child_last_name"] = "Tilda", "Berger"
						}
						contracts = append(contracts, row)
					}
					respond(map[string]any{"contracts": contracts})
					return
				}
				if strings.HasSuffix(r.URL.Path, "/paper-signature") {
					assert.Equal(t, "POST", r.Method)
					assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")
					writes = append(writes, r.Method+" "+r.URL.Path)
					respond(map[string]any{})
					return
				}
				prefix := "/traeger/contract-notices/"
				if !strings.HasPrefix(r.URL.Path, prefix) {
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(404)
					return
				}
				parts := strings.Split(strings.TrimPrefix(r.URL.Path, prefix), "/")
				id := parts[0]
				if r.Method == "GET" {
					respond(map[string]any{"notice": notices[id]})
					return
				}
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				writes = append(writes, r.Method+" "+r.URL.Path)
				if len(parts) == 1 {
					assert.Equal(t, "kuendigung", body["kind"])
					assert.NotEmpty(t, body["received_on"])
					if body["termination_kind"] == "ausserordentlich" {
						assert.NotEmpty(t, body["reason"])
					}
					notices[id] = map[string]any{"termination_kind": body["termination_kind"]}
					respond(map[string]any{"notice": notices[id], "reinterpreted_ends_on": "2026-12-31"})
					return
				}
				switch parts[1] {
				case "ends-on":
					assert.Equal(t, "PUT", r.Method)
					assert.NotEmpty(t, body["ends_on"])
					notices[id]["end_decision"] = "festgelegt"
				case "reason-rejection":
					assert.Equal(t, "POST", r.Method)
					assert.Equal(t, "2026-12-31", body["expected_ends_on"], "use the server's preview, not a second date calculation")
					notices[id]["end_decision"] = "umgedeutet"
				default:
					t.Errorf("unexpected decision %s", parts[1])
					w.WriteHeader(404)
				}
				respond(map[string]any{})
			})
			defer srv.Close()
			client := newTestClient(srv.URL, false)
			created, err := seedCareContractNotice(client, AuthRef{})
			require.NoError(t, err)
			wantCreated := 4
			if ordinaryExists {
				wantCreated--
			}
			assert.Equal(t, wantCreated, created)
			require.Len(t, notices, 4)
			assert.Equal(t, "ordentlich", notices["3"]["termination_kind"])
			assert.Equal(t, "ausserordentlich", notices["4"]["termination_kind"])
			assert.Empty(t, notices["4"]["end_decision"], "one extraordinary termination remains open")
			assert.Equal(t, "festgelegt", notices["5"]["end_decision"])
			assert.Equal(t, "umgedeutet", notices["6"]["end_decision"])
			writeCount := len(writes)
			created, err = seedCareContractNotice(client, AuthRef{})
			require.NoError(t, err)
			assert.Zero(t, created)
			assert.Len(t, writes, writeCount, "a repeat sends no declaration or decision again")
		})
	}
}
