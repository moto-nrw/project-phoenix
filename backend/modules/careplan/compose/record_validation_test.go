package compose

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// The record adapters are the last stop before storage for writes that bypass
// the effective-time core (bulk upserts, excusals, approvals), so they reject
// invalid rows themselves.
func TestEffectiveRecordsRejectInvalidRowsBeforeStorage(t *testing.T) {
	t.Parallel()
	writes := 0
	store := func(_ context.Context, row careplan.PickupSchedule) (careplan.PickupSchedule, error) {
		writes++
		return row, nil
	}
	records := effectiveRecords[careplan.PickupSchedule, *careplan.PickupSchedule]{
		create: store, upsert: store,
		update: func(context.Context, careplan.PickupSchedule) error { writes++; return nil },
	}

	require.EqualError(t, records.Create(context.Background(), &careplan.PickupSchedule{}), "student_id is required")
	require.EqualError(t, records.UpsertSchedule(context.Background(), &careplan.PickupSchedule{}), "student_id is required")
	require.EqualError(t, records.Update(context.Background(), &careplan.PickupSchedule{}), "student_id is required")
	assert.Zero(t, writes)
}

type excusalStoreSpy struct {
	PickupExcusalRecords
	writes int
}

func (s *excusalStoreSpy) CreatePickupException(_ context.Context, row careplan.PickupException) (careplan.PickupException, error) {
	s.writes++
	return row, nil
}

func (s *excusalStoreSpy) UpdatePickupException(context.Context, careplan.PickupException) error {
	s.writes++
	return nil
}

func TestPickupExcusalRecordsRejectInvalidRowsBeforeStorage(t *testing.T) {
	t.Parallel()
	spy := &excusalStoreSpy{}
	records := pickupExcusalRecords{spy}

	require.EqualError(t, records.Create(context.Background(), &careplan.PickupException{}), "student_id is required")
	require.EqualError(t, records.Update(context.Background(), &careplan.PickupException{}), "student_id is required")
	assert.Zero(t, spy.writes)
}

func TestPickupApprovalExceptionsRejectInvalidRowsBeforeStorage(t *testing.T) {
	t.Parallel()
	spy := &excusalStoreSpy{}
	approvals := pickupApprovalExceptions{spy}

	_, err := approvals.Create(context.Background(), careplan.PickupException{})
	require.EqualError(t, err, "student_id is required")
	require.EqualError(t, approvals.Update(context.Background(), careplan.PickupException{}), "student_id is required")
	assert.Zero(t, spy.writes)
}
