// Unit tests for the shift_moved Änderungsprotokoll entry (#1884): MoveShift
// appends one shift-anchored event per changing move and nothing on a no-op.
package planning

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shiftMoveEventRepo captures the recorded shift_moved entries.
type shiftMoveEventRepo struct {
	created   []workforce.ShiftMovedEvent
	createErr error
}

func (m *shiftMoveEventRepo) record(_ context.Context, event workforce.ShiftMovedEvent) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, event)
	return nil
}

func TestMoveShift_LogsShiftMovedEvent(t *testing.T) {
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()
	events := &shiftMoveEventRepo{}
	svc.(*staffShiftService).shiftMoveAudit = events.record

	existing := validShift(7)
	existing.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*workforce.ShiftRecord, error) {
		return existing, nil
	}
	repo.updateFunc = func(_ context.Context, _ *workforce.ShiftRecord) error { return nil }

	actor := int64(99)
	moved, err := svc.MoveShift(context.Background(), MoveShiftInput{
		ShiftID:        existing.ID,
		SourceStaffID:  7,
		TargetStaffID:  8,
		Date:           calendar.Date(existing.Date),
		StartTime:      existing.StartTime,
		EndTime:        existing.EndTime,
		BreakMinutes:   existing.BreakMinutes,
		ActorStaffID:   7,
		ActorAccountID: &actor,
	})
	require.NoError(t, err)

	require.Len(t, events.created, 1, "exactly one protocol entry per move")
	ev := events.created[0]
	assert.Equal(t, moved.ID, ev.StaffShiftID, "anchored on the shift row")
	assert.Equal(t, moved.Date, ev.OccurrenceDate, "the occurrence is the new slot's day")
	assert.False(t, ev.OccurrenceDate.IsZero())
	assert.Equal(t, int64(7), ev.SubjectStaffID)
	require.NotNil(t, ev.RelatedStaffID)
	assert.Equal(t, int64(8), *ev.RelatedStaffID)
	require.NotNil(t, ev.ActorAccountID)
	assert.Equal(t, actor, *ev.ActorAccountID)
	assert.Contains(t, string(ev.OldValue), `"staff_id":7`)
	assert.Contains(t, string(ev.NewValue), `"staff_id":8`)
}

func TestMoveShift_NoOpMoveLogsNothing(t *testing.T) {
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()
	events := &shiftMoveEventRepo{}
	svc.(*staffShiftService).shiftMoveAudit = events.record

	existing := validShift(7)
	existing.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*workforce.ShiftRecord, error) {
		return existing, nil
	}

	_, err := svc.MoveShift(context.Background(), MoveShiftInput{
		ShiftID:       existing.ID,
		SourceStaffID: 7,
		TargetStaffID: 7,
		Date:          calendar.Date(existing.Date),
		StartTime:     existing.StartTime,
		EndTime:       existing.EndTime,
		BreakMinutes:  existing.BreakMinutes,
		ActorStaffID:  7,
	})
	require.NoError(t, err)
	assert.Empty(t, events.created, "an unchanged move is not a state change")
}

func TestMoveShift_EventWriteFailureAbortsMove(t *testing.T) {
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()
	events := &shiftMoveEventRepo{createErr: errors.New("audit down")}
	svc.(*staffShiftService).shiftMoveAudit = events.record

	existing := validShift(7)
	existing.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*workforce.ShiftRecord, error) {
		return existing, nil
	}
	repo.updateFunc = func(_ context.Context, _ *workforce.ShiftRecord) error { return nil }

	_, err := svc.MoveShift(context.Background(), MoveShiftInput{
		ShiftID:       existing.ID,
		SourceStaffID: 7,
		TargetStaffID: 8,
		Date:          calendar.Date(existing.Date),
		StartTime:     existing.StartTime,
		EndTime:       existing.EndTime,
		BreakMinutes:  existing.BreakMinutes,
		ActorStaffID:  7,
	})
	require.Error(t, err, "fail closed: the protocol is the compliance artifact")
	assert.Contains(t, err.Error(), "log shift move event")
}
