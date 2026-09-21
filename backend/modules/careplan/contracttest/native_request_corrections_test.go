package contracttest_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type correctionBoundary struct {
	decisionBoundary
	allowed bool
	history []compose.RequestCorrectionEvent
	payload map[string]any
}

func (b *correctionBoundary) CanReview(context.Context, compose.ReviewStudent) (bool, error) {
	b.calls = append(b.calls, "authorize")
	return b.allowed, nil
}
func (b *correctionBoundary) CorrectionHistory(context.Context, *carerequests.Request) ([]compose.RequestCorrectionEvent, error) {
	b.calls = append(b.calls, "history")
	return b.history, nil
}
func (b *correctionBoundary) RecordCorrection(_ context.Context, _ *carerequests.Request, _ int64, payload map[string]any) error {
	b.calls = append(b.calls, "record_correction")
	b.payload = payload
	return b.ledgerErr
}
func (b *correctionBoundary) NotifyCorrection(context.Context, *carerequests.Request, int64, bool, string) error {
	b.calls = append(b.calls, "notify_correction")
	return b.notificationErr
}

func TestNativeCorrectionReauthorizesAndRollsBackExactExceptionDeletion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := careplantest.NewCarePlan(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Correction", "Transaction", "1a")
	parent := testpkg.CreateTestAccount(t, db, "parent")
	staff, reviewer := testpkg.CreateTestStaffWithAccount(t, db, "Correction", "Reviewer")
	exception, err := owner.CreatePickupException(ctx, careplan.PickupException{StudentID: student.ID, ExceptionDate: "2031-02-03", CreatedBy: staff.ID})
	require.NoError(t, err)
	request, err := owner.CreateCareScheduleRequest(ctx, carerequests.Request{StudentID: student.ID, SubmittedBy: parent.ID,
		RequestKind: "pickup_change", Status: "pending", Payload: json.RawMessage(`{"date":"2031-02-03","pickup_time":"14:00"}`)})
	require.NoError(t, err)
	priorReason := "Bestätigt"
	require.NoError(t, owner.DecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision{ID: request.ID, Status: "approved", ReviewedBy: &reviewer.ID, Reason: &priorReason, Applied: true}))
	request, err = owner.FindCareScheduleRequest(ctx, request.ID, false)
	require.NoError(t, err)
	b := &correctionBoundary{decisionBoundary: decisionBoundary{student: compose.ReviewStudent{ID: student.ID}}}
	service, err := compose.NewRequestCorrections(compose.RequestCorrectionDependencies{Records: owner, People: b, Plans: b, Effects: b})
	require.NoError(t, err)
	correct := func(version string) error {
		return testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			return service.Correct(txCtx, request.ID, false, version, "  Korrektur  ", reviewer.ID)
		})
	}
	require.ErrorIs(t, correct("stale"), careplan.ErrParentRequestStale)
	require.Empty(t, b.calls)
	version := careplan.ParentRequestVersion(request.UpdatedAt)
	require.ErrorIs(t, correct(version), carerequests.ErrCareRequestForbidden)
	require.Equal(t, []string{"lock_student", "authorize"}, b.calls)
	b.allowed = true
	require.ErrorIs(t, correct(version), careplan.ErrParentRequestCorrectionUnsupported)
	b.history = []compose.RequestCorrectionEvent{
		{Type: "decided", Payload: map[string]any{"pickup_exception_id": float64(exception.ID)}},
		{Type: "corrected", Payload: map[string]any{"pickup_exception_id": parent.ID}},
	}
	for _, failNotice := range []bool{false, true} {
		failure := errors.New("correction effect unavailable")
		b.ledgerErr, b.notificationErr = failure, nil
		if failNotice {
			b.ledgerErr, b.notificationErr = nil, failure
		}
		require.ErrorIs(t, correct(version), failure)
		_, err = owner.FindPickupException(ctx, exception.ID, false)
		require.NoError(t, err, "deletion rolls back when either effect fails")
		unchanged, readErr := owner.FindCareScheduleRequest(ctx, request.ID, false)
		require.NoError(t, readErr)
		require.Equal(t, "approved", unchanged.Status)
	}
	b.ledgerErr, b.notificationErr, b.calls = nil, nil, nil
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		reason := "Nachträglich geändert"
		exception.Reason = &reason
		require.NoError(t, owner.UpdatePickupException(txCtx, exception))
		edited, readErr := owner.FindPickupException(txCtx, exception.ID, false)
		require.NoError(t, readErr)
		require.True(t, edited.UpdatedAt.After(*request.ReviewedAt))
		return service.Correct(txCtx, request.ID, false, version, "Korrektur", reviewer.ID)
	})
	require.ErrorIs(t, err, careplan.ErrParentRequestCorrectionUnsupported)
	require.Contains(t, err.Error(), "03.02.2031 wurde nach der Entscheidung geändert")
	b.calls = nil
	require.NoError(t, correct(version))
	require.Equal(t, []string{"lock_student", "authorize", "history", "record_correction", "notify_correction"}, b.calls)
	require.Equal(t, map[string]any{"approve": false, "reason": "Korrektur", "from": "approved", "to": "rejected", "prior_reviewer": reviewer.ID, "prior_reason": priorReason}, b.payload)
	_, err = owner.FindPickupException(ctx, exception.ID, false)
	require.ErrorIs(t, err, careplan.ErrStudentScheduleNotFound)
}
