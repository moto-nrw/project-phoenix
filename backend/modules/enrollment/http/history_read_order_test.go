package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	compose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentHTTP "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type historyReadInterleave struct {
	compose.DecisionChildren
	afterFirstPhase func()
	fired           bool
}

func (s *historyReadInterleave) ChildrenForRequest(ctx context.Context, id int64, lock bool) ([]*enrollment.RequestChild, error) {
	if !s.fired {
		s.fired = true
		s.afterFirstPhase()
	}
	return s.DecisionChildren.ChildrenForRequest(ctx, id, lock)
}

// A committed phase edit between the first phase read and the next request's
// phase read is visible at READ COMMITTED. The child port supplies only the
// deterministic scheduling point; both the reads and concurrent edit use SQL.
func TestEnrollmentHistoryKeepsPhaseReadOrder(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	env := f.env
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, env.db, "History", "Interleave", "2a")
	beforeName := env.phase.Name
	for i := range 3 {
		req := makeOwnerRequest(env.phaseID, uniqueToken("history-read-order"), fmt.Sprintf("history-order%d@example.test", i))
		req.SubmittedAt = time.Now().Add(time.Duration(i) * time.Minute)
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertRequest, req))
		child := makeChild(req.ID, "History", "Interleave")
		child.Status = enrollment.ChildStatusApproved
		child.CreatedStudentID = &student.ID
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChild, child))
	}
	children := &historyReadInterleave{DecisionChildren: env.repos.Enrollment(), afterFirstPhase: func() {
		phase := *env.phase
		phase.Name = "Committed phase edit"
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().UpdatePhase, &phase))
	}}
	f.resource.DecisionService = newTestDecisionService(compose.DecisionDependencies{Requests: env.repos.Enrollment(), Children: children, Guardians: env.repos.Enrollment(), Phases: env.repos.Enrollment(), Schemas: env.repos.Enrollment(), Offerings: compose.NewCareOfferingRecords(env.reposCare.CarePlan)}, f.settings.Settings)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/students/%d/requests/", student.ID), nil)
	req.Header.Set("Authorization", "Bearer "+f.token)
	rec := httptest.NewRecorder()
	f.router(t).ServeHTTP(rec, req)
	t.Logf("wire status=%d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Data []enrollmentHTTP.AdminRequestSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data, 3)
	require.Equal(t, beforeName, body.Data[0].PhaseName)
	require.Equal(t, "Committed phase edit", body.Data[1].PhaseName)
	require.Equal(t, "Committed phase edit", body.Data[2].PhaseName)
	require.True(t, children.fired)
}
