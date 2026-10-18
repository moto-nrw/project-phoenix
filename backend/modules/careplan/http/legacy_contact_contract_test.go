package careplanhttp_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

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
