package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedStaffPersonMergeStepDoesNotDuplicateExistingExample(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		personID int64
	}{
		{name: "person ID", personID: 11},
		{name: "large person ID", personID: 9007199254740993},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			staff := DemoStaff[0]
			hasExample := false
			created := 0
			openInvitation := false
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/operator/staff-person-merges/42":
					if hasExample {
						_, _ = fmt.Fprintf(w, `{"data":[{"keeper":{"person":{"id":%q}}}]}`, fmt.Sprint(tc.personID))
						return
					}
					_, _ = fmt.Fprint(w, `{"data":[]}`)
				case r.Method == http.MethodPost && r.URL.Path == "/api/staff/externals":
					var body map[string]string
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					assert.Equal(t, staffPersonMergeDuplicateEmail(staff, ""), body["email"])
					created++
					openInvitation = true
					_, _ = fmt.Fprint(w, `{"data":{"id":"2"}}`)
				case r.Method == http.MethodGet && r.URL.Path == "/auth/invitations":
					_, _ = fmt.Fprintf(w, `{"data":[{"id":7,"email":%q}]}`, staffPersonMergeDuplicateEmail(staff, ""))
				case r.Method == http.MethodDelete && r.URL.Path == "/auth/invitations/7":
					openInvitation = false
					_, _ = fmt.Fprint(w, `{"data":null}`)
				case r.Method == http.MethodPut && r.URL.Path == "/api/staff/2/stammdaten/kontakt":
					_, _ = fmt.Fprint(w, `{"data":{}}`)
				case r.Method == http.MethodDelete && r.URL.Path == "/api/staff/2":
					assert.False(t, openInvitation, "the invitation is withdrawn before the employment ends")
					hasExample = true
					_, _ = fmt.Fprint(w, `{"data":{}}`)
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
			})
			defer srv.Close()

			rt := &Runtime{
				Client:       newTestClient(srv.URL, false),
				OperatorAuth: AuthRef{Token: "operator"},
				TenantAuth:   AuthRef{Token: "tenant"},
				Bootstrap:    &bootstrapSeedState{SchoolID: 42},
				FixedSeeder: &FixedSeeder{staffIDs: map[string]int64{
					fmt.Sprintf("%s %s", staff.FirstName, staff.LastName): 1,
				}, personIDs: map[string]int64{fmt.Sprintf("%s %s", staff.FirstName, staff.LastName): tc.personID}},
			}
			step := seedStaffPersonMergeStep{}
			require.NoError(t, step.Run(t.Context(), rt))
			require.NoError(t, step.Run(t.Context(), rt))

			assert.Equal(t, 1, created)
			assert.False(t, openInvitation)
		})
	}
}
