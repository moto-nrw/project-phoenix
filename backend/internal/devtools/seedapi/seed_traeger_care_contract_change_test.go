package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedCareContractChangeUsesContractChild(t *testing.T) {
	t.Parallel()
	for _, student := range []string{"22", "null", "99"} {
		t.Run(student, func(t *testing.T) {
			var catalogs, requests []string
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.URL.Path == "/traeger/contracts/":
					_, _ = fmt.Fprintf(w, `{"data":{"contracts":[{"id":70,"party_email":"parent@example.test","student_id":%s}]}}`, student)
				case r.URL.Path == "/parent/auth/login":
					_, _ = fmt.Fprint(w, `{"access_token":"parent-token"}`)
				case strings.HasSuffix(r.URL.Path, "/catalog"):
					catalogs = append(catalogs, r.URL.Path)
					_, _ = fmt.Fprint(w, `{"data":{"earliest_effective_from":"2026-12-01","items":[{"id":"80","selected":true,"days_of_week_mode":"parent_choice","selected_days":["mon","tue"]}]}}`)
				case strings.HasSuffix(r.URL.Path, "/requests"):
					requests = append(requests, r.URL.Path)
					_, _ = fmt.Fprint(w, `{"data":{}}`)
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
				}
			})
			defer srv.Close()
			rt := &Runtime{Client: newTestClient(srv.URL, false), Parents: []ParentCredentials{{Email: "parent@example.test", Password: "example-test-password", StudentIDs: []int64{11, 22}}}}
			changed, err := seedCareContractChange(rt, AuthRef{})
			require.NoError(t, err)
			if student == "22" {
				assert.True(t, changed)
				assert.Equal(t, []string{"/parent/me/children/22/care-offerings/catalog"}, catalogs)
				assert.Equal(t, []string{"/parent/me/children/22/care-offerings/requests"}, requests)
			} else {
				assert.False(t, changed)
				assert.Empty(t, catalogs)
				assert.Empty(t, requests)
			}
		})
	}
}
