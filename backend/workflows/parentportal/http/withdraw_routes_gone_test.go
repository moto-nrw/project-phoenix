package parent_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestWithdrawRoutesGone pins the retirement of guardian withdrawal (#2267,
// story 39): a guardian corrects an open request instead of taking it back, so
// the four withdraw routes must be gone for good. An old parents-portal tab
// that still holds the buttons gets a clean 404/405 instead of silently
// closing a request the family meant to fix.
func TestWithdrawRoutesGone(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	router := newWriteRouter(t, db)
	token := parentToken(t, chain.AccountID)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodDelete, fmt.Sprintf("/me/children/%d/excused-requests/1", chain.StudentID)},
		{http.MethodDelete, fmt.Sprintf("/me/children/%d/pickup-change-requests/1", chain.StudentID)},
		{http.MethodPost, fmt.Sprintf("/me/children/%d/care-schedule/requests/1/withdraw", chain.StudentID)},
		{http.MethodPost, fmt.Sprintf("/me/children/%d/care-offerings/requests/1/withdraw", chain.StudentID)},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rr := doRequest(t, router, tc.method, tc.path, token, nil)
			assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, rr.Code,
				"the withdraw route must not be routed any more, got %d: %s", rr.Code, rr.Body.String())
		})
	}
}
