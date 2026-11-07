package enrollmenthttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmenthttp "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestEnrollmentAdminChangesPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env := setupReviewListTest(t)
	counter := testpkg.CaptureQueries(t, env.db)
	var ids []string
	add := func(n int) {
		for range n {
			row := env.insertChangeRequest(t, enrollment.ChangeRequestStatusRejected, time.Now().Add(time.Duration(len(ids))*time.Minute), 0, true)
			ids = append(ids, fmt.Sprint(row.ID))
		}
	}
	run := func(n int) [][]string {
		counter.Reset()
		rec := env.getPath(t, "/admin/change-requests/?status=rejected", env.token)
		listQ := counter.Queries()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var list struct {
			Data []enrollmenthttp.ChangeRequestResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
		require.Len(t, list.Data, n)
		for i, row := range list.Data {
			require.Equal(t, ids[n-1-i], row.ID)
			require.Equal(t, fmt.Sprint(env.requestID), row.RequestID)
			require.Equal(t, enrollment.ChangeRequestStatusRejected, row.Status)
		}
		counter.Reset()
		page := env.fetch(t, "view=history&limit=100")
		reviewQ := counter.Queries()
		require.Equal(t, reverseFinalIDs(ids), idsOf(page))
		for _, row := range page.Data.Items {
			require.Equal(t, "enrollment", row.RequestType)
			require.ElementsMatch(t, []string{"Quirina Zoffenbach", "Xander Zoffenbach"}, row.Data.ChildNames)
			require.Len(t, row.Data.Children, 2)
		}
		return [][]string{listQ, reviewQ}
	}
	add(3)
	small := run(3)
	add(5)
	large := run(8)
	for i := range large {
		t.Logf("admin changes route %d N3=%d N8=%d", i, len(small[i]), len(large[i]))
	}
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.admin_changes", large[0])
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.review_changes", large[1])
}

func reverseFinalIDs(ids []string) []string {
	out := make([]string, len(ids))
	for i := range ids {
		out[i] = ids[len(ids)-1-i]
	}
	return out
}

func TestEnrollmentAdminChangeDetailPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env := setupReviewListTest(t)
	row := env.insertChangeRequest(t, enrollment.ChangeRequestStatusRejected, time.Now(), 0, true)
	counter := testpkg.CaptureQueries(t, env.db)
	n := 0
	add := func(count int) {
		for range count {
			message := &enrollment.ChangeRequestMessage{ChangeRequestID: row.ID, AuthorType: enrollment.ChangeRequestMessageAuthorParent, Body: fmt.Sprintf("Detail message %02d", n)}
			message.TenantID = env.tenantID
			_, err := env.db.NewInsert().Model(message).ModelTableExpr("enrollment.change_request_messages").Column("tenant_id", "change_request_id", "author_type", "body").Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			n++
		}
	}
	run := func() []string {
		counter.Reset()
		rec := env.getPath(t, fmt.Sprintf("/admin/change-requests/%d/", row.ID), env.token)
		queries := counter.Queries()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body struct {
			Data enrollmenthttp.ChangeRequestResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, fmt.Sprint(row.ID), body.Data.ID)
		require.Equal(t, fmt.Sprint(env.requestID), body.Data.RequestID)
		require.NotNil(t, body.Data.Request)
		require.Len(t, body.Data.Request.Children, 2)
		require.Len(t, body.Data.Messages, n)
		for i, message := range body.Data.Messages {
			require.Equal(t, fmt.Sprintf("Detail message %02d", i), message.Body)
			require.Equal(t, enrollment.ChangeRequestMessageAuthorParent, message.AuthorType)
			require.False(t, message.InternalOnly)
		}
		return queries
	}
	add(3)
	small := run()
	add(5)
	large := run()
	require.Equal(t, len(small), len(large))
	t.Logf("admin change detail messages N3=%d N8=%d", len(small), len(large))
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.admin_change_detail", large)
}
