package enrollmenthttp_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"

	enrollment "github.com/moto-ogs/project-phoenix/modules/enrollment"
	compose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestEnrollmentPublicChangesRouteGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	env := f.env
	ctx := testpkg.Ctx(t)
	input := validSubmission(t, env.phaseID)
	submitted, err := f.resource.RequestService.Submit(ctx, input)
	require.NoError(t, err)
	require.NotEmpty(t, submitted.Request.StatusToken)
	f.resource.ChangeRequestService = newTestChangeRequestService(owners.EnrollmentChangeRequestSources{Requests: env.repos.Enrollment(), Children: env.repos.Enrollment(), Guardians: env.repos.Enrollment(), Catalog: env.repos.Enrollment(), CareOfferingRepo: compose.NewCareOfferingRecords(env.reposCare.CarePlan), Settings: f.settings.Settings, OutboxEnqueuer: env.outbox, ParentsURL: "http://parents.localhost:3000", FrontendURL: "http://localhost:3000"})
	router := f.router(t)
	counter := testpkg.CaptureQueries(t, env.db)
	ids := []string{}
	add := func(n int) {
		for range n {
			i := len(ids)
			at := time.Now().Add(time.Duration(i) * time.Minute)
			row := &enrollment.ChangeRequest{RequestID: submitted.Request.ID, Origin: enrollment.ChangeRequestOriginParent, Status: enrollment.ChangeRequestStatusRejected, BaseSnapshot: json.RawMessage(`{"guardian_first_name":"Anna"}`), ProposedSnapshot: json.RawMessage(`{"guardian_first_name":"Anne"}`), Diff: json.RawMessage(`{"changed":["guardian_first_name"]}`), CreatedAt: at, UpdatedAt: at, ReviewedAt: &at, ReviewedByAccountID: &env.creatorID}
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChangeRequest, row))
			message := &enrollment.ChangeRequestMessage{ChangeRequestID: row.ID, AuthorType: enrollment.ChangeRequestMessageAuthorParent, Body: fmt.Sprintf("Public message %d", i)}
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChangeRequestMessage, message))
			ids = append(ids, fmt.Sprint(row.ID))
		}
	}
	run := func(n int) testpkg.QueryMeasurement {
		raw, q := growthReadOnlyGET(t, f, router, counter, "", "/requests/"+submitted.Request.StatusToken+"/change-requests")
		var rows []struct {
			ID        string `json:"id"`
			RequestID string `json:"request_id"`
			Messages  []struct {
				Body string `json:"body"`
			} `json:"messages"`
		}
		require.NoError(t, json.Unmarshal(raw, &rows))
		require.Len(t, rows, n)
		for i, row := range rows {
			require.Equal(t, ids[n-1-i], row.ID)
			require.Equal(t, fmt.Sprint(submitted.Request.ID), row.RequestID)
			require.Len(t, row.Messages, 1)
			require.Equal(t, fmt.Sprintf("Public message %d", n-1-i), row.Messages[0].Body)
		}
		return q
	}
	add(3)
	small := run(3)
	add(5)
	large := run(8)
	t.Logf("public changes N3=%d N8=%d", len(small.Statements), len(large.Statements))
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.public_changes", large)
}
