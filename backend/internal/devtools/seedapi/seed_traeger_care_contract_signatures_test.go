package api

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every demo school hangs under one Träger, whose contract list holds a page
// of at most 100 drafts. The seeded school's parents find their contracts
// only when the seed reads the drafts of their school (#6621).
func TestSeedSendContractsToParentsReadsTheDraftsOfTheSeededSchool(t *testing.T) {
	t.Parallel()

	var query string
	var sent map[string][]int64
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /traeger/contracts/":
			query = r.URL.RawQuery
			_, _ = fmt.Fprint(w, `{"data":{"contracts":[{"id":4,"party_email":"Eltern@example.test"},{"id":5,"party_email":"other@example.test"}]}}`)
		case "POST /traeger/contracts/send":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
			_, _ = fmt.Fprint(w, `{"data":{}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()
	rt := &Runtime{
		Client: newTestClient(srv.URL, false), Bootstrap: &bootstrapSeedState{SchoolID: 65},
		Parents: []ParentCredentials{{Email: "eltern@example.test"}},
	}

	require.NoError(t, seedSendContractsToParents(rt, AuthRef{Kind: AuthBearer, Token: "traeger"}))
	assert.Equal(t, "status=entwurf&school_id=65&limit=100", query)
	assert.Equal(t, map[string][]int64{"contract_ids": {4}}, sent)
}

// The paper contract is a draft of the seeded school, and a school that has
// an active contract already gets none, whatever the other schools of the
// Träger hold (#6621).
func TestSeedPaperContractSignsADraftOfTheSeededSchool(t *testing.T) {
	t.Parallel()

	for _, active := range []int{0, 1} {
		t.Run(fmt.Sprintf("active_%d", active), func(t *testing.T) {
			t.Parallel()
			var queries []string
			var signed []string
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "GET /traeger/contracts/":
					queries = append(queries, r.URL.RawQuery)
					if r.URL.Query().Get("status") == "aktiv" {
						_, _ = fmt.Fprintf(w, `{"data":{"contracts":[],"counts":{"aktiv":%d}}}`, active)
						return
					}
					_, _ = fmt.Fprint(w, `{"data":{"contracts":[{"id":7,"party_name":"Erika Muster"},{"id":8,"party_name":"Max Muster"}]}}`)
				case "POST /traeger/contracts/8/paper-signature":
					signed = append(signed, r.URL.Path)
					_, _ = fmt.Fprint(w, `{"data":{}}`)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
			})
			defer srv.Close()

			paper, err := seedPaperContract(newTestClient(srv.URL, false), AuthRef{Kind: AuthBearer, Token: "traeger"}, 65)
			require.NoError(t, err)
			if active > 0 {
				assert.Zero(t, paper)
				assert.Equal(t, []string{"status=aktiv&school_id=65"}, queries)
				assert.Empty(t, signed)
				return
			}
			assert.Equal(t, 1, paper)
			assert.Equal(t, []string{"status=aktiv&school_id=65", "status=entwurf&school_id=65"}, queries)
			assert.Equal(t, []string{"/traeger/contracts/8/paper-signature"}, signed)
		})
	}
}
