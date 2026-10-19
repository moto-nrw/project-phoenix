package presenceprojectionhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentAPIExcludesRetiredContacts(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Contact", "Read", "1a")
	// Linked contacts must not reintroduce retired student response fields.
	guardian := testpkg.CreateTestGuardianProfileNamed(t, tc.db, "Linked", "Contact", "linked@example.test")
	testpkg.CreateTestStudentGuardianLink(t, tc.db, student.ID, guardian.ID, "parent")
	for _, path := range []string{fmt.Sprintf("/%d", student.ID), "/"} {
		t.Run(path, func(t *testing.T) {
			rr := authExec(t, tc, testutil.NewRequest(http.MethodGet, path, nil), testutil.AdminTestClaims(1), []string{"admin:*"})
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			var response struct {
				Data json.RawMessage `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
			var records []map[string]any
			if path == "/" {
				require.NoError(t, json.Unmarshal(response.Data, &records))
			} else {
				var record map[string]any
				require.NoError(t, json.Unmarshal(response.Data, &record))
				records = append(records, record)
			}
			require.NotEmpty(t, records)
			for _, record := range records {
				for _, key := range []string{"guardian_name", "guardian_contact", "guardian_email", "guardian_phone"} {
					require.NotContains(t, record, key)
				}
			}
		})
	}
}
