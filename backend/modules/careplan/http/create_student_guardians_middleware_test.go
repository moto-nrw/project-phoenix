package careplanhttp_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/stretchr/testify/require"
)

// TestCreateStudent_GuardianRelationshipTypeAccepted confirms every allowed
// relationship type still creates the student+guardian, so tightening the
// validation didn't reject valid input. Runs through the real middleware.
func TestCreateStudent_GuardianRelationshipTypeAccepted(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	for _, relType := range []string{"parent", "guardian", "relative", "other"} {
		t.Run(relType, func(t *testing.T) {
			body := map[string]interface{}{
				"first_name":   "RelOk",
				"last_name":    "Accept_" + relType,
				"school_class": "2b",
				"guardians": []map[string]interface{}{
					{
						"first_name":         "Valid",
						"last_name":          "Guardian",
						"relationship_type":  relType,
						"emergency_priority": 1,
					},
				},
			}

			req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
			rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
			require.Equal(t, http.StatusCreated, rr.Code,
				"relationship_type %q must be accepted. Body: %s", relType, rr.Body.String())

			var resp createStudentResponse
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		})
	}
}
