package api

import (
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
		wireID   string
		personID string
	}{
		{name: "numeric person ID", wireID: "11", personID: "11"},
		{name: "quoted person ID", wireID: `"11"`, personID: "11"},
		{name: "large numeric person ID", wireID: "9007199254740993", personID: "9007199254740993"},
		{name: "large quoted person ID", wireID: `"9007199254740993"`, personID: "9007199254740993"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			staff := DemoStaff[0]
			hasExample := false
			created := 0
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/api/staff/1":
					_, _ = fmt.Fprintf(w, `{"data":{"person":{"id":%s,"first_name":%q,"last_name":%q}}}`,
						tc.wireID, staff.FirstName, staff.LastName)
				case r.Method == http.MethodGet && r.URL.Path == "/operator/staff-person-merges/42":
					if hasExample {
						_, _ = fmt.Fprintf(w, `{"data":[{"keeper":{"person":{"id":%q}}}]}`, tc.personID)
						return
					}
					_, _ = fmt.Fprint(w, `{"data":[]}`)
				case r.Method == http.MethodPost && r.URL.Path == "/api/staff/externals":
					created++
					_, _ = fmt.Fprint(w, `{"data":{"id":"2"}}`)
				case r.Method == http.MethodPut && r.URL.Path == "/api/staff/2/stammdaten/kontakt":
					_, _ = fmt.Fprint(w, `{"data":{}}`)
				case r.Method == http.MethodDelete && r.URL.Path == "/api/staff/2":
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
				}},
			}
			step := seedStaffPersonMergeStep{}
			require.NoError(t, step.Run(t.Context(), rt))
			require.NoError(t, step.Run(t.Context(), rt))

			assert.Equal(t, 1, created)
		})
	}
}
