package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// externalCaregiverSeedServer fakes the staff, invitation and acceptance
// routes the external caregiver step drives, and records what it was sent.
type externalCaregiverSeedServer struct {
	t          *testing.T
	created    []map[string]string
	seedHeader []string
	accepted   map[string]string // token -> password
	revoked    []string
	open       map[int64]string // invitation id -> email
}

func (s *externalCaregiverSeedServer) handle(w seedHTTPResponseWriter, r *seedHTTPRequest) {
	t := s.t
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/staff/":
		data := make([]map[string]any, 0, len(s.created))
		for index, external := range s.created {
			data = append(data, map[string]any{
				"id":                    fmt.Sprint(100 + index),
				"is_external":           true,
				"external_organization": external["organization"],
				"person":                map[string]string{"first_name": external["first_name"], "last_name": external["last_name"]},
			})
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": data}))
	case r.Method == http.MethodPost && r.URL.Path == "/api/staff/externals":
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		s.created = append(s.created, body)
		s.seedHeader = append(s.seedHeader, r.Header.Get(seedTokenHeader))
		id := int64(100 + len(s.created) - 1)
		s.open[id] = body["email"]
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":"%d","is_external":true,"invitation_token":"token-%d"}}`, id, id)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/auth/invitations/token-"):
		token := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/auth/invitations/"), "/accept")
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, body["password"], body["confirm_password"])
		s.accepted[token] = body["password"]
		var id int64
		_, err := fmt.Sscanf(token, "token-%d", &id)
		require.NoError(t, err)
		delete(s.open, id) // acceptance consumes the invitation
		_, _ = fmt.Fprint(w, `{"status":"success","data":{}}`)
	case r.Method == http.MethodGet && r.URL.Path == "/auth/invitations":
		data := make([]map[string]any, 0, len(s.open))
		for id, email := range s.open {
			data = append(data, map[string]any{"id": id, "email": email})
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": data}))
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/auth/invitations/"):
		var id int64
		_, err := fmt.Sscanf(strings.TrimPrefix(r.URL.Path, "/auth/invitations/"), "%d", &id)
		require.NoError(t, err)
		s.revoked = append(s.revoked, s.open[id])
		delete(s.open, id)
		_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
	default:
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}
}

func TestSeedExternalCaregiversStepInvitesExternalsWithAndWithoutAccount(t *testing.T) {
	t.Parallel()

	fake := &externalCaregiverSeedServer{t: t, accepted: map[string]string{}, open: map[int64]string{}}
	srv := newSeedHTTPTestServer(fake.handle)
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), FixedSeeder: &FixedSeeder{staffPassword: "Shared1234%"}}
	step := seedExternalCaregiversStep{}
	require.NoError(t, step.Run(t.Context(), rt))
	require.NoError(t, step.Run(t.Context(), rt))

	emails := make([]string, 0, len(fake.created))
	for _, body := range fake.created {
		emails = append(emails, body["email"])
	}
	assert.Equal(t, []string{
		"jonas.becker@example.test", "leonie.wagner@example.test",
		"selin.aydin@example.test", "dennis.kaya@example.test",
	}, emails, "every external gets an ASCII address; a second run creates nobody again")
	assert.Equal(t, []string{"true", "true", "true", "true"}, fake.seedHeader)
	assert.Equal(t, map[string]string{"token-100": "Shared1234%", "token-101": "Shared1234%"}, fake.accepted,
		"Jonas Becker and Leonie Wagner accept with the shared staff password")
	assert.Equal(t, []string{"dennis.kaya@example.test"}, fake.revoked, "Dennis Kaya ends without account and invitation")
	assert.Equal(t, map[int64]string{102: "selin.aydin@example.test"}, fake.open, "Selin Aydın keeps her invitation open")

	ids := seededExternalCaregiverIDs(rt)
	assert.Equal(t, int64(100), ids["Jonas Becker"])
	assert.Equal(t, int64(103), ids["Dennis Kaya"])
}

func TestSeedExternalCaregiversStepUsesTheSchoolDomainOfAScopedSeed(t *testing.T) {
	t.Parallel()

	fake := &externalCaregiverSeedServer{t: t, accepted: map[string]string{}, open: map[int64]string{}}
	srv := newSeedHTTPTestServer(fake.handle)
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), FixedSeeder: &FixedSeeder{accountScope: "ogs-nord"}}
	require.NoError(t, seedExternalCaregiversStep{}.Run(t.Context(), rt))

	require.NotEmpty(t, fake.created)
	assert.Equal(t, "jonas.becker@demo-ogs-nord.moto-ogs.de", fake.created[0]["email"])
	assert.Equal(t, defaultSeedExternalPassword, fake.accepted["token-100"])
}
