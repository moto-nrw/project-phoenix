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
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type submissionEffects struct {
	failure error
	stage   string
	calls   []string
}

func (e *submissionEffects) RecordSubmission(context.Context, *carerequests.Request) error {
	e.calls = append(e.calls, "record")
	if e.stage == "record" {
		return e.failure
	}
	return nil
}
func (e *submissionEffects) NotifySubmission(context.Context, *carerequests.Request) error {
	e.calls = append(e.calls, "notify")
	if e.stage == "notify" {
		return e.failure
	}
	return nil
}
func (e *submissionEffects) WakeGuardians(context.Context, *carerequests.Request) {
	e.calls = append(e.calls, "wake")
}

func TestNativeRequestSubmissionRollsBackEffectsAndRejectsDuplicates(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := careplantest.NewCarePlan(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Request", "Submission", "1a")
	account := testpkg.CreateTestAccount(t, db, "parent")
	effects := &submissionEffects{failure: errors.New("submission effect failed")}
	service, err := compose.NewRequestSubmissions(owner, nil, effects, func() calendar.Date { return "2026-09-20" })
	require.NoError(t, err)
	payload := json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"16:00","ignored":true}]}`)
	create := func() error {
		return testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			_, createErr := service.CreateRequest(txCtx, student.ID, account.ID, payload)
			return createErr
		})
	}
	for _, stage := range []string{"record", "notify"} {
		effects.stage, effects.calls = stage, nil
		require.ErrorIs(t, create(), effects.failure)
		rows, readErr := owner.ListCareScheduleRequests(ctx, careplan.CareScheduleRequestFilter{StudentID: student.ID})
		require.NoError(t, readErr)
		require.Empty(t, rows, "failed effects must roll back the request")
		require.NotContains(t, effects.calls, "wake")
		if stage == "record" {
			require.NotContains(t, effects.calls, "notify")
		}
	}
	effects.stage, effects.calls = "", nil
	require.NoError(t, create())
	require.Equal(t, []string{"record", "notify", "wake"}, effects.calls)
	rows, err := owner.ListCareScheduleRequests(ctx, careplan.CareScheduleRequestFilter{StudentID: student.ID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.JSONEq(t, `{"weekdays":[{"weekday":1,"pickup":"16:00"}]}`, string(rows[0].Payload))
	effects.calls = nil
	require.ErrorIs(t, create(), carerequests.ErrAlreadyPending)
	require.Empty(t, effects.calls)
}
