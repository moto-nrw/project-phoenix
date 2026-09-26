package students_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type staticParentRequestReviewAccess string

func (access staticParentRequestReviewAccess) AccessLevel(context.Context, []string) (string, error) {
	return string(access), nil
}

func TestChangeRequestAccessReportsGroupLeaderCapabilityFromPolicy(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Access", "Reviewer")
	tc.resource.RequestReviewAccess = staticParentRequestReviewAccess("group_leader")

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := authExec(
		t,
		tc,
		testutil.NewRequest("GET", "/change-requests/access", nil),
		claims,
		[]string{"users:read", "users:update"},
	)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var envelope struct {
		Data struct {
			ReviewAccess string `json:"review_access"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
	assert.Equal(t, "group_leader", envelope.Data.ReviewAccess)
}

func TestChangeRequestAccessReportsNoneFromPolicy(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Access", "No Group")
	tc.resource.RequestReviewAccess = staticParentRequestReviewAccess("none")

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := authExec(
		t,
		tc,
		testutil.NewRequest("GET", "/change-requests/access", nil),
		claims,
		[]string{"users:read", "users:update"},
	)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var envelope struct {
		Data struct {
			ReviewAccess string `json:"review_access"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
	assert.Equal(t, "none", envelope.Data.ReviewAccess)
}
