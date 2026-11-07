package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type publicCaseReadInterleave struct {
	compose.IntakeChildren
	beforeSecondChildren func()
	reads                int
}

func (s *publicCaseReadInterleave) ChildrenForRequest(ctx context.Context, id int64, lock bool) ([]*enrollment.RequestChild, error) {
	s.reads++
	if s.reads == 2 {
		s.beforeSecondChildren()
	}
	return s.IntakeChildren.ChildrenForRequest(ctx, id, lock)
}

// A batch may cover immutable terminal threads, but must not freeze an open
// case's reply or the shared request's children and phase across tenant frames.
func TestEnrollmentPublicCasesKeepOpenMessagesAndSubjectLive(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	env := f.env
	ctx := testpkg.Ctx(t)
	submitted, err := f.resource.RequestService.Submit(ctx, validSubmission(t, env.phaseID))
	require.NoError(t, err)
	statuses := []string{enrollment.ChangeRequestStatusApproved, enrollment.ChangeRequestStatusNeedsParentResponse, enrollment.ChangeRequestStatusRejected}
	cases := make([]*enrollment.ChangeRequest, 0, len(statuses))
	for _, status := range statuses {
		row := &enrollment.ChangeRequest{RequestID: submitted.Request.ID, Origin: enrollment.ChangeRequestOriginParent, Status: status, BaseSnapshot: json.RawMessage(`{}`), ProposedSnapshot: json.RawMessage(`{}`), Diff: json.RawMessage(`{}`)}
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChangeRequest, row))
		cases = append([]*enrollment.ChangeRequest{row}, cases...)
	}
	for _, internal := range []bool{false, true} {
		message := &enrollment.ChangeRequestMessage{ChangeRequestID: cases[0].ID, AuthorType: enrollment.ChangeRequestMessageAuthorStaff, Body: "Original message", InternalOnly: internal}
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChangeRequestMessage, message))
	}
	beforePhase := env.phase.Name
	beforeChild := submitted.Children[0].FirstName
	children := &publicCaseReadInterleave{IntakeChildren: env.repos.Enrollment(), beforeSecondChildren: func() {
		message := &enrollment.ChangeRequestMessage{ChangeRequestID: cases[1].ID, AuthorType: enrollment.ChangeRequestMessageAuthorParent, Body: "Committed reply"}
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChangeRequestMessage, message))
		require.NoError(t, testpkg.TenantCommand2(t, ctx, env.db, env.repos.Enrollment().SetChangeRequestStatus, cases[1].ID, enrollment.ChangeRequestStatusPendingReview))
		phase := *env.phase
		phase.Name = "Committed phase name"
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().UpdatePhase, &phase))
		_, updateErr := env.db.NewRaw("UPDATE enrollment.request_children SET first_name = ? WHERE tenant_id = ? AND id = ?", "Committed child name", testpkg.Tenant(t), submitted.Children[0].ID).Exec(ctx)
		require.NoError(t, updateErr)
	}}
	service := newTestChangeRequestService(owners.EnrollmentChangeRequestSources{Requests: env.repos.Enrollment(), Children: children, Guardians: env.repos.Enrollment(), Catalog: env.repos.Enrollment(), CareOfferingRepo: compose.NewCareOfferingRecords(env.reposCare.CarePlan), Settings: f.settings.Settings, OutboxEnqueuer: env.outbox, ParentsURL: "http://parents.localhost:3000", FrontendURL: "http://localhost:3000"})
	rows, err := service.ListPublic(ctx, submitted.Request.StatusToken)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	for i, row := range rows {
		require.Equal(t, cases[i].ID, row.ChangeRequest.ID)
		require.Len(t, row.Children, 1)
	}
	require.Equal(t, beforePhase, rows[0].Phase.Name)
	require.Equal(t, beforeChild, rows[0].Children[0].FirstName)
	for _, row := range rows[1:] {
		require.Equal(t, "Committed phase name", row.Phase.Name)
		require.Equal(t, "Committed child name", row.Children[0].FirstName)
	}
	require.Len(t, rows[0].Messages, 1)
	require.False(t, rows[0].Messages[0].InternalOnly)
	require.Len(t, rows[1].Messages, 1)
	require.Equal(t, "Committed reply", rows[1].Messages[0].Body)
	require.Empty(t, rows[2].Messages)
	// A second call constructs its own set, rather than retaining the first
	// call's terminal messages on the service.
	message := &enrollment.ChangeRequestMessage{ChangeRequestID: cases[2].ID, AuthorType: enrollment.ChangeRequestMessageAuthorStaff, Body: "Later stored message"}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChangeRequestMessage, message))
	rows, err = service.ListPublic(ctx, submitted.Request.StatusToken)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	require.Len(t, rows[2].Messages, 1)
	require.Equal(t, message.Body, rows[2].Messages[0].Body)
}
