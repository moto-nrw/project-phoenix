package students_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type aggListEnvelope struct {
	Data struct {
		Items []struct {
			RequestType string          `json:"request_type"`
			Data        json.RawMessage `json:"data"`
		} `json:"items"`
		NextCursor string `json:"next_cursor"`
	} `json:"data"`
}

// insertDecidedMasterDataRequest creates one decided Stammdaten request with a
// controlled decision instant so the keyset order is deterministic.
func insertDecidedMasterDataRequest(t *testing.T, tc *testContext, studentID, tenantID, accountID int64, status string, decidedAt time.Time) *userModels.StudentDataChangeRequest {
	t.Helper()
	request := &userModels.StudentDataChangeRequest{
		StudentID:   studentID,
		SubmittedBy: accountID,
		Target:      userModels.DataChangeTargetPerson,
		FieldKey:    "first_name",
		NewValue:    json.RawMessage(`"Neu"`),
		Status:      status,
		ReviewedBy:  &accountID,
		ReviewedAt:  &decidedAt,
	}
	request.TenantID = tenantID
	request.CreatedAt = decidedAt.Add(-time.Hour)
	request.UpdatedAt = decidedAt
	_, err := tc.db.NewInsert().Model(request).Exec(t.Context())
	require.NoError(t, err)
	return request
}

// The aggregated list runs against the production router: real middleware
// chain, real services, real keyset SQL. The distinctive child name plus the
// server-side search keeps the assertions hermetic on the shared test DB.
func TestAggregatedChangeRequests_RouterHistoryCursor(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Agg", "Reviewer")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AggListGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Zaggreg", "Atkind", "AL1")
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	base := time.Now().UTC().Add(-time.Hour)
	newest := insertDecidedMasterDataRequest(t, tc, student.ID, student.TenantID, account.ID, userModels.DataChangeStatusApproved, base)
	middle := insertDecidedMasterDataRequest(t, tc, student.ID, student.TenantID, account.ID, userModels.DataChangeStatusRejected, base.Add(-time.Minute))
	oldest := insertDecidedMasterDataRequest(t, tc, student.ID, student.TenantID, account.ID, userModels.DataChangeStatusApproved, base.Add(-2*time.Minute))

	claims := testutil.TeacherTestClaims(int(account.ID))
	perms := []string{"users:read", "users:update"}

	fetch := func(t *testing.T, query string) aggListEnvelope {
		t.Helper()
		rr := authExec(t, tc, testutil.NewRequest("GET", "/change-requests?"+query, nil), claims, perms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var env aggListEnvelope
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
		return env
	}

	requestIDs := func(env aggListEnvelope) []string {
		ids := make([]string, 0, len(env.Data.Items))
		for _, item := range env.Data.Items {
			var payload struct {
				ID string `json:"id"`
			}
			require.NoError(t, json.Unmarshal(item.Data, &payload))
			ids = append(ids, payload.ID)
		}
		return ids
	}
	idOf := func(request *userModels.StudentDataChangeRequest) string {
		return strconv.FormatInt(request.ID, 10)
	}

	pageOne := fetch(t, "view=history&types=master_data&search=Zaggreg&limit=2")
	require.Len(t, pageOne.Data.Items, 2)
	assert.Equal(t, []string{idOf(newest), idOf(middle)}, requestIDs(pageOne))
	require.NotEmpty(t, pageOne.Data.NextCursor)

	pageTwo := fetch(t, "view=history&types=master_data&search=Zaggreg&limit=2&cursor="+url.QueryEscape(pageOne.Data.NextCursor))
	assert.Equal(t, []string{idOf(oldest)}, requestIDs(pageTwo))
	assert.Empty(t, pageTwo.Data.NextCursor)

	// The history status filter runs server-side.
	rejectedOnly := fetch(t, "view=history&types=master_data&search=Zaggreg&status=rejected")
	assert.Equal(t, []string{idOf(middle)}, requestIDs(rejectedOnly))
}

func TestAggregatedChangeRequests_RouterOpenSearchAndPermissions(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Agg", "OpenReviewer")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AggOpenGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Zoffen", "Listkind", "AL2")
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	request := &userModels.StudentDataChangeRequest{
		StudentID:   student.ID,
		SubmittedBy: account.ID,
		Target:      userModels.DataChangeTargetPerson,
		FieldKey:    "first_name",
		NewValue:    json.RawMessage(`"Neu"`),
		Status:      userModels.DataChangeStatusPending,
	}
	request.TenantID = student.TenantID
	_, err := tc.db.NewInsert().Model(request).Exec(t.Context())
	require.NoError(t, err)

	claims := testutil.TeacherTestClaims(int(account.ID))

	// A users:update reviewer finds the pending Stammdaten request by name.
	rr := authExec(t, tc, testutil.NewRequest("GET", "/change-requests?search=Zoffen", nil), claims, []string{"users:read", "users:update"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var env aggListEnvelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	require.Len(t, env.Data.Items, 1)
	assert.Equal(t, "master_data", env.Data.Items[0].RequestType)

	// An absence-only caller may open the route but is narrowed to the
	// excused queue — the users:update request stays invisible (#2232).
	rr = authExec(t, tc, testutil.NewRequest("GET", "/change-requests?search=Zoffen", nil), claims, []string{"users:read", "users:absence"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	env = aggListEnvelope{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	assert.Empty(t, env.Data.Items)

	// Without either review permission the route itself refuses.
	rr = authExec(t, tc, testutil.NewRequest("GET", "/change-requests", nil), claims, []string{"users:read"})
	assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
}

// Kinderkartei-Reiter „Änderungsprotokoll“ (#2437): the same aggregation,
// filtered to one child. Two children of the same group each carry a decided
// request, so the test proves the filter selects and — more importantly —
// excludes.
func TestAggregatedChangeRequests_RouterStudentFilter(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Agg", "ProtocolReviewer")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AggProtocolGroup")
	child := testpkg.CreateTestStudent(t, tc.db, "Zprotokoll", "Eigenkind", "AP1")
	other := testpkg.CreateTestStudent(t, tc.db, "Zprotokoll", "Fremdkind", "AP2")
	testpkg.AssignStudentToGroup(t, tc.db, child.ID, group.ID)
	testpkg.AssignStudentToGroup(t, tc.db, other.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	base := time.Now().UTC().Add(-time.Hour)
	own := insertDecidedMasterDataRequest(t, tc, child.ID, child.TenantID, account.ID, userModels.DataChangeStatusApproved, base)
	foreign := insertDecidedMasterDataRequest(t, tc, other.ID, other.TenantID, account.ID, userModels.DataChangeStatusApproved, base.Add(-time.Minute))

	claims := testutil.TeacherTestClaims(int(account.ID))
	perms := []string{"users:read", "users:update"}

	fetch := func(t *testing.T, query string) aggListEnvelope {
		t.Helper()
		rr := authExec(t, tc, testutil.NewRequest("GET", "/change-requests?"+query, nil), claims, perms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var env aggListEnvelope
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
		return env
	}
	requestIDs := func(env aggListEnvelope) []string {
		ids := make([]string, 0, len(env.Data.Items))
		for _, item := range env.Data.Items {
			var payload struct {
				ID string `json:"id"`
			}
			require.NoError(t, json.Unmarshal(item.Data, &payload))
			ids = append(ids, payload.ID)
		}
		return ids
	}

	// Both children share the search name; only the filtered child remains.
	both := requestIDs(fetch(t, "view=history&types=master_data&search=Zprotokoll"))
	assert.Contains(t, both, strconv.FormatInt(own.ID, 10))
	assert.Contains(t, both, strconv.FormatInt(foreign.ID, 10))

	filtered := requestIDs(fetch(t, "view=history&student_id="+strconv.FormatInt(child.ID, 10)))
	assert.Equal(t, []string{strconv.FormatInt(own.ID, 10)}, filtered)

	// A malformed child id is a client bug, not an unfiltered list.
	for _, bad := range []string{"student_id=0", "student_id=-1", "student_id=abc"} {
		rr := authExec(t, tc, testutil.NewRequest("GET", "/change-requests?view=history&"+bad, nil), claims, perms)
		assert.Equal(t, http.StatusBadRequest, rr.Code, bad)
	}
}
