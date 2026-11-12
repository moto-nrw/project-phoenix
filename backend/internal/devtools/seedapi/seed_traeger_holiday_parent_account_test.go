package api

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// holidayParentAccountServer answers the routes of the parent account at
// the holiday registration and records what the seed sent.
type holidayParentAccountServer struct {
	t           *testing.T
	mu          sync.Mutex
	events      []string
	bodies      []map[string]any
	seedHeaders []string
}

func (s *holidayParentAccountServer) handle(w seedHTTPResponseWriter, r *seedHTTPRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var body map[string]any
	if r.Body != nil && r.Method != seedHTTPMethodGet {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	s.events = append(s.events, r.Method+" "+r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/parent/auth/login":
		_, _ = w.Write([]byte(`{"status":"success","data":{"access_token":"parent-token"}}`))
	case "/parent/holiday-registrations/link-key/family":
		_, _ = w.Write([]byte(`{"status":"success","data":{"guardian":{},"children":[],"account_proof":"sealed-proof"}}`))
	case "/parent/me/holiday-registration-entries":
		_, _ = w.Write([]byte(`{"status":"success","data":{"registrations":[{"id":1},{"id":2}]}}`))
	case "/traeger/holiday-registrations/link-key":
		s.bodies = append(s.bodies, body)
		s.seedHeaders = append(s.seedHeaders, r.Header.Get(seedTokenHeader))
		w.WriteHeader(seedHTTPStatusCreated)
		if body["parent_account_requested"] == true && r.Header.Get(seedTokenHeader) == "true" {
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":10,"parent_account_setup_url":"http://parents.localhost:3000/ferienkonto/setup-token"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"id":9}}`))
	default:
		_, _ = w.Write([]byte(`{"status":"success","data":{}}`))
	}
}

func holidayParentAccountRuntime(url string) *Runtime {
	client := newTestClient(url, false)
	return &Runtime{
		Client: client, Adapter: client.adapter,
		Parents: []ParentCredentials{{Email: "eltern@example.org", Password: "example-test-password", Name: "Sabine Ferienkind"}},
		Values: map[string]any{seedHolidayLegalPublishedKey: []byte(`{"data":{"public_token":"link-key","weeks":[` +
			`{"id":51,"time_models":[{"id":61},{"id":62}]},{"id":52,"time_models":[{"id":63}]}]}}`)},
	}
}

// The seeded parent registers with the account's proof and asks for nothing;
// the guest family asks for an account with the seed header, redeems the
// setup link of the answer and signs in at the Träger, and the local seed
// state keeps that login. No mailbox is read.
func TestSeedHolidayParentAccountStepSeedsBothWaysToTheAccount(t *testing.T) {
	t.Parallel()
	server := &holidayParentAccountServer{t: t}
	srv := newSeedHTTPTestServer(server.handle)
	defer srv.Close()
	seeder := &Seeder{}

	require.NoError(t, seedHolidayParentAccountStep{seeder: seeder}.Run(context.Background(), holidayParentAccountRuntime(srv.URL)))

	require.Len(t, server.bodies, 2)
	assert.Equal(t, "sealed-proof", server.bodies[0]["parent_account_proof"], "the seeded parent registers with the proof")
	assert.Nil(t, server.bodies[0]["parent_account_requested"])
	assert.Equal(t, seedHolidayProofFamilyEmail, server.bodies[0]["guardian"].(map[string]any)["email"], "an address of its own, outside the seeded parent's daily limit")
	assert.Equal(t, true, server.bodies[1]["parent_account_requested"], "the guest family asks for an account")
	assert.Nil(t, server.bodies[1]["parent_account_proof"])
	assert.Equal(t, []string{"", "true"}, server.seedHeaders, "only the submit whose setup link the seed redeems sends the seed header")
	assert.Contains(t, server.events, "GET /auth/holiday-parent-setups/setup-token")
	assert.Contains(t, server.events, "POST /auth/holiday-parent-setups/setup-token")
	require.NotNil(t, seeder.holidayParentAccount)
	assert.Equal(t, seedHolidayGuestFamilyEmail, seeder.holidayParentAccount.Email)
	assert.Equal(t, seedHolidayParentAccountKey, seeder.holidayParentAccount.Key)
}
