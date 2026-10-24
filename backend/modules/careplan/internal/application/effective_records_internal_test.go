package application

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
	records := NewEffectiveRecords[careplan.PickupSchedule, *careplan.PickupSchedule](RecordFuncs[careplan.PickupSchedule]{
		Create: store, Upsert: store,
		Update: func(context.Context, careplan.PickupSchedule) error { writes++; return nil },
	})

	require.EqualError(t, records.Create(context.Background(), &careplan.PickupSchedule{}), "student_id is required")
	require.EqualError(t, records.UpsertSchedule(context.Background(), &careplan.PickupSchedule{}), "student_id is required")
	require.EqualError(t, records.Update(context.Background(), &careplan.PickupSchedule{}), "student_id is required")
	assert.Zero(t, writes)
}
