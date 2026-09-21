package contracttest_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type editEvents struct{ err error }

func (e *editEvents) RecordGuardianEdit(context.Context, *carerequests.Request, int64) error {
	return e.err
}

func TestNativeRequestEditChecksOwnershipBeforeStatusAndVersion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := careplantest.NewCarePlan(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Edit", "Owner", "1a")
	account := testpkg.CreateTestAccount(t, db, "parent")
	stranger := testpkg.CreateTestAccount(t, db, "stranger")
	row, err := owner.CreateCareScheduleRequest(ctx, carerequests.Request{StudentID: student.ID, SubmittedBy: account.ID,
		RequestKind: "weekly_schedule", Status: "pending", Payload: json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"16:00"}]}`)})
	require.NoError(t, err)
	query, err := compose.NewRequestEdits(owner, nil, &editEvents{}, func() calendar.Date { return "2026-09-20" })
	require.NoError(t, err)
	input := carerequests.EditInput{RequestID: row.ID, StudentID: student.ID, GuardianAccountID: stranger.ID, ExpectedVersion: "stale"}
	edit := func() error {
		return testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			_, editErr := query.EditRequest(txCtx, input)
			return editErr
		})
	}
	require.ErrorIs(t, edit(), careplan.ErrCareScheduleRequestNotFound)
	input.GuardianAccountID = account.ID
	require.ErrorIs(t, edit(), careplan.ErrParentRequestStale)
	require.NoError(t, owner.DecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision{ID: row.ID, Status: "rejected"}))
	input.GuardianAccountID = stranger.ID
	require.ErrorIs(t, edit(), careplan.ErrCareScheduleRequestNotFound)
	input.GuardianAccountID = account.ID
	require.ErrorIs(t, edit(), careplan.ErrCareScheduleRequestNotPending)
}

type editedPickup struct{ dates []calendar.Date }

func (p *editedPickup) PickupTime(_ context.Context, _ int64, date calendar.Date) (*time.Time, error) {
	p.dates = append(p.dates, date)
	clock := time.Date(1, 1, 1, 17, 0, 0, 0, time.UTC)
	return &clock, nil
}

func TestNativePickupEditChecksOldCutoffAndRollsBackLedgerFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := careplantest.NewCarePlan(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Edit", "Pickup", "1a")
	account := testpkg.CreateTestAccount(t, db, "parent")
	const original = `{"date":"2026-09-20","pickup_time":"15:00","previous_pickup_time":"16:00","reason":"Termin"}`
	row, err := owner.CreateCareScheduleRequest(ctx, carerequests.Request{StudentID: student.ID, SubmittedBy: account.ID,
		RequestKind: "pickup_change", Status: "pending", Payload: json.RawMessage(original)})
	require.NoError(t, err)
	events := &editEvents{err: errors.New("ledger unavailable")}
	pickup := &editedPickup{}
	query, err := compose.NewRequestEdits(owner, pickup, events, func() calendar.Date { return "2026-09-20" })
	require.NoError(t, err)
	cutoff, err := careplan.NewSameDayCutoff("11:00", time.Date(2026, 9, 20, 12, 0, 0, 0, calendar.Berlin))
	require.NoError(t, err)
	input := carerequests.EditInput{RequestID: row.ID, StudentID: student.ID, GuardianAccountID: account.ID,
		ExpectedVersion: careplan.ParentRequestVersion(row.UpdatedAt), Date: "2026-09-21", PickupTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC), Reason: "Neuer Termin", Cutoff: cutoff}
	edit := func() error {
		return testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			_, editErr := query.EditRequest(txCtx, input)
			return editErr
		})
	}
	require.ErrorIs(t, edit(), carerequests.ErrPickupChangeCutoffPassed)
	require.Empty(t, pickup.dates)
	input.Cutoff = nil
	require.ErrorIs(t, edit(), events.err)
	unchanged, err := owner.FindCareScheduleRequest(ctx, row.ID, false)
	require.NoError(t, err)
	require.JSONEq(t, original, string(unchanged.Payload))
	events.err = nil
	require.NoError(t, edit())
	updated, err := owner.FindCareScheduleRequest(ctx, row.ID, false)
	require.NoError(t, err)
	require.Equal(t, row.ID, updated.ID)
	require.Equal(t, row.RequestKind, updated.RequestKind)
	require.JSONEq(t, `{"date":"2026-09-21","pickup_time":"14:00","previous_pickup_time":"17:00","reason":"Neuer Termin"}`, string(updated.Payload))
	require.Equal(t, []calendar.Date{input.Date, input.Date}, pickup.dates)
}
