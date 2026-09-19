package students_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestStudentAPIExcludesRetiredContacts(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Contact", "Read", "1a")
	// Arrange a pre-cutover row independently of the current student write API.
	_, err := tc.db.NewUpdate().Table("users.students").
		Set("guardian_name = ?", "Legacy Contact").Set("guardian_contact = ?", "legacy@example.test").
		Set("guardian_email = ?", "legacy@example.test").Set("guardian_phone = ?", "0123456789").
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("id = ?", student.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
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

func TestStudentAPIRejectsRetiredContactFields(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Contact", "Contract", "1a")
	for _, field := range []string{"guardian_name", "guardian_contact", "guardian_email", "guardian_phone"} {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			for _, value := range []any{"legacy@example.test", "", nil} {
				t.Run(fmt.Sprintf("%s/%s/%v", method, field, value), func(t *testing.T) {
					body := map[string]any{"first_name": "Contact", "last_name": "Contract", "school_class": "1a", field: value}
					path := "/"
					if method == http.MethodPut {
						path = fmt.Sprintf("/%d", student.ID)
					}
					req := testutil.NewAuthenticatedRequest(t, method, path, body)
					rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
					require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
				})
			}
		}
	}
}
