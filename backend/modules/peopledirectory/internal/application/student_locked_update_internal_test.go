package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

const lockedUpdateStudentID int64 = 4711

// lockedRows serves one locked row and records the lock and the write.
type lockedRows struct {
	row      *peopledirectory.StudentState
	lockErr  error
	locked   []int64
	written  []*peopledirectory.StudentState
	writeErr error
}

func (r *lockedRows) FindByIDForUpdate(_ context.Context, id int64) (*peopledirectory.StudentState, error) {
	r.locked = append(r.locked, id)
	return r.row, r.lockErr
}

func (r *lockedRows) Update(_ context.Context, student *peopledirectory.StudentState) error {
	r.written = append(r.written, student)
	return r.writeErr
}

func TestWidenAccompaniedDaysByIDLocksTheRowItWidens(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("the locked row is widened", func(t *testing.T) {
		t.Parallel()
		rows := &lockedRows{row: &peopledirectory.StudentState{}}
		found, err := NewLockedStudentUpdates(rows, nil).WidenAccompaniedDaysByID(ctx, lockedUpdateStudentID, []string{peopledirectory.PickupDayTuesday}, 0)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, []int64{lockedUpdateStudentID}, rows.locked)
		require.Len(t, rows.written, 1)
		assert.Same(t, rows.row, rows.written[0])
		assert.Equal(t, map[string]bool{peopledirectory.PickupDayTuesday: true}, peopledirectory.AccompaniedWeekdays(rows.row.AllowedDepartureModes, rows.row.DepartureDays))
	})

	t.Run("a missing row is not found", func(t *testing.T) {
		t.Parallel()
		for name, rows := range map[string]*lockedRows{
			"no rows": {lockErr: peoplePorts.MissingStudentError("find by id for update")},
			"nil row": {},
		} {
			found, err := NewLockedStudentUpdates(rows, nil).WidenAccompaniedDaysByID(ctx, lockedUpdateStudentID, []string{peopledirectory.PickupDayTuesday}, 0)
			require.NoError(t, err, name)
			assert.False(t, found, name)
			assert.Empty(t, rows.written, name)
		}
	})

	t.Run("another lock error is returned", func(t *testing.T) {
		t.Parallel()
		lockErr := errors.New("lock failed")
		rows := &lockedRows{lockErr: lockErr}
		_, err := NewLockedStudentUpdates(rows, nil).WidenAccompaniedDaysByID(ctx, lockedUpdateStudentID, []string{peopledirectory.PickupDayTuesday}, 0)
		require.ErrorIs(t, err, lockErr)
		assert.Empty(t, rows.written)
	})
}

func TestSetLiveAbsenceFlags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	at := time.Date(2026, time.October, 18, 8, 30, 0, 0, time.UTC)
	earlier := at.Add(-24 * time.Hour)
	stored := func() *peopledirectory.StudentState {
		sick, excused := false, true
		return &peopledirectory.StudentState{Sick: &sick, SickSince: &earlier, Excused: &excused, ExcusedSince: &earlier}
	}

	t.Run("a sick day raises sick at the approval time and clears excused", func(t *testing.T) {
		t.Parallel()
		rows := &lockedRows{row: stored()}
		err := NewLockedStudentUpdates(rows, nil).SetLiveAbsenceFlags(ctx, lockedUpdateStudentID, peopledirectory.LiveAbsenceFlags{Sick: true, ClearExcused: true}, at)
		require.NoError(t, err)
		assert.Equal(t, []int64{lockedUpdateStudentID}, rows.locked)
		require.Len(t, rows.written, 1)
		row := rows.written[0]
		require.NotNil(t, row.Sick)
		assert.True(t, *row.Sick)
		require.NotNil(t, row.SickSince)
		assert.Equal(t, at, *row.SickSince)
		require.NotNil(t, row.Excused)
		assert.False(t, *row.Excused)
		assert.Nil(t, row.ExcusedSince)
	})

	t.Run("another absence clears sick and keeps excused", func(t *testing.T) {
		t.Parallel()
		rows := &lockedRows{row: stored()}
		err := NewLockedStudentUpdates(rows, nil).SetLiveAbsenceFlags(ctx, lockedUpdateStudentID, peopledirectory.LiveAbsenceFlags{}, at)
		require.NoError(t, err)
		row := rows.written[0]
		require.NotNil(t, row.Sick)
		assert.False(t, *row.Sick)
		assert.Nil(t, row.SickSince)
		require.NotNil(t, row.Excused)
		assert.True(t, *row.Excused)
		require.NotNil(t, row.ExcusedSince)
		assert.Equal(t, earlier, *row.ExcusedSince)
	})

	t.Run("the lock and the write errors are returned", func(t *testing.T) {
		t.Parallel()
		lockErr := errors.New("lock failed")
		rows := &lockedRows{lockErr: lockErr}
		require.ErrorIs(t, NewLockedStudentUpdates(rows, nil).SetLiveAbsenceFlags(ctx, lockedUpdateStudentID, peopledirectory.LiveAbsenceFlags{}, at), lockErr)
		assert.Empty(t, rows.written)

		writeErr := errors.New("write failed")
		rows = &lockedRows{row: stored(), writeErr: writeErr}
		require.ErrorIs(t, NewLockedStudentUpdates(rows, nil).SetLiveAbsenceFlags(ctx, lockedUpdateStudentID, peopledirectory.LiveAbsenceFlags{}, at), writeErr)
	})
}
