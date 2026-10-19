package application

import (
	"context"
	"errors"

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

const auditedUpdateActor int64 = 4711

// auditedUpdateJournal records the row write and the history in call order.
// trimIn names the step that reports a trimmed companion link to the
// recording scope it runs in.
type auditedUpdateJournal struct {
	steps     []string
	actor     int64
	updateErr error
	recordErr error
	trimIn    string
}

func (j *auditedUpdateJournal) Update(ctx context.Context, _ *peopledirectory.StudentState) error {
	j.steps = append(j.steps, "update")
	if j.trimIn == "update" {
		peopledirectory.RecordCompanionChange(ctx)
	}
	return j.updateErr
}

func (j *auditedUpdateJournal) RecordChangesForActor(ctx context.Context, _, _ *peopledirectory.StudentState, editedBy int64) error {
	j.steps, j.actor = append(j.steps, "audit"), editedBy
	if j.trimIn == "audit" {
		peopledirectory.RecordCompanionChange(ctx)
	}
	return j.recordErr
}

func TestUpdateStudentAuditedWritesThenRecordsInOneScope(t *testing.T) {
	t.Parallel()
	for _, step := range []string{"update", "audit"} {
		journal := &auditedUpdateJournal{trimIn: step}
		trimmed, err := NewAuditedStudentUpdates(journal, journal).UpdateStudentAudited(context.Background(), &peopledirectory.StudentState{}, &peopledirectory.StudentState{}, auditedUpdateActor)

		require.NoError(t, err)
		assert.True(t, trimmed, "the %s step runs in the recording scope the result reads", step)
		assert.Equal(t, []string{"update", "audit"}, journal.steps)
		assert.Equal(t, auditedUpdateActor, journal.actor)
	}
	journal := &auditedUpdateJournal{}
	trimmed, err := NewAuditedStudentUpdates(journal, journal).UpdateStudentAudited(context.Background(), &peopledirectory.StudentState{}, &peopledirectory.StudentState{}, auditedUpdateActor)
	require.NoError(t, err)
	assert.False(t, trimmed, "an untouched link reads as unchanged")
}

func TestUpdateStudentAuditedNamesTheFailedStep(t *testing.T) {
	t.Parallel()
	recordErr := errors.New("trail unavailable")
	journal := &auditedUpdateJournal{recordErr: recordErr}
	_, err := NewAuditedStudentUpdates(journal, journal).UpdateStudentAudited(context.Background(), &peopledirectory.StudentState{}, &peopledirectory.StudentState{}, auditedUpdateActor)
	var auditErr *peopledirectory.StudentAuditError
	require.ErrorAs(t, err, &auditErr)
	assert.Same(t, recordErr, auditErr.Err)
	assert.EqualError(t, err, "trail unavailable")

	updateErr := errors.New("row refused")
	journal = &auditedUpdateJournal{updateErr: updateErr}
	_, err = NewAuditedStudentUpdates(journal, journal).UpdateStudentAudited(context.Background(), &peopledirectory.StudentState{}, &peopledirectory.StudentState{}, auditedUpdateActor)
	assert.Same(t, updateErr, err)
	assert.Equal(t, []string{"update"}, journal.steps, "a refused row records no history")
}

func TestUpdateStudentAuditedWithoutRecorderOnlyWrites(t *testing.T) {
	t.Parallel()
	journal := &auditedUpdateJournal{}
	_, err := NewAuditedStudentUpdates(journal, nil).UpdateStudentAudited(context.Background(), &peopledirectory.StudentState{}, &peopledirectory.StudentState{}, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"update"}, journal.steps)
}

func TestWidenAccompaniedDays(t *testing.T) {
	t.Parallel()
	companion := func() *peopledirectory.StudentState {
		return &peopledirectory.StudentState{AllowedDepartureModes: peopledirectory.AllowedDepartureModes{
			peopledirectory.PickupDayMonday: {peopledirectory.DepartureAccompanied},
		}}
	}

	t.Run("already accompanied days are not written", func(t *testing.T) {
		journal := &auditedUpdateJournal{}
		require.NoError(t, NewAuditedStudentUpdates(journal, journal).WidenAccompaniedDays(context.Background(), companion(), []string{peopledirectory.PickupDayMonday}, 0))
		assert.Empty(t, journal.steps)
	})

	t.Run("an unattributed widening is refused while a recorder is bound", func(t *testing.T) {
		journal := &auditedUpdateJournal{}
		err := NewAuditedStudentUpdates(journal, journal).WidenAccompaniedDays(context.Background(), companion(), []string{peopledirectory.PickupDayTuesday}, 0)
		require.EqualError(t, err, "users: companion departure audit requires an actor account")
		assert.Empty(t, journal.steps)
	})

	t.Run("the missing days are added and marked, then recorded", func(t *testing.T) {
		journal := &auditedUpdateJournal{}
		row := companion()
		err := NewAuditedStudentUpdates(journal, journal).WidenAccompaniedDays(context.Background(), row, []string{peopledirectory.PickupDayMonday, peopledirectory.PickupDayTuesday}, auditedUpdateActor)
		require.NoError(t, err)
		assert.Equal(t, []string{"update", "audit"}, journal.steps)
		assert.Contains(t, row.AllowedDepartureModes[peopledirectory.PickupDayTuesday], peopledirectory.DepartureAccompanied)
		assert.Equal(t, map[string]bool{peopledirectory.PickupDayMonday: true, peopledirectory.PickupDayTuesday: true}, row.DepartureCompanionDays)
	})

	t.Run("a failed history keeps the companion wording", func(t *testing.T) {
		recordErr := errors.New("trail unavailable")
		journal := &auditedUpdateJournal{recordErr: recordErr}
		err := NewAuditedStudentUpdates(journal, journal).WidenAccompaniedDays(context.Background(), companion(), []string{peopledirectory.PickupDayTuesday}, auditedUpdateActor)
		require.EqualError(t, err, "users: audit companion departure plan: trail unavailable")
		assert.ErrorIs(t, err, recordErr)
	})

	t.Run("without a recorder an unattributed widening is written", func(t *testing.T) {
		journal := &auditedUpdateJournal{}
		require.NoError(t, NewAuditedStudentUpdates(journal, nil).WidenAccompaniedDays(context.Background(), companion(), []string{peopledirectory.PickupDayTuesday}, 0))
		assert.Equal(t, []string{"update"}, journal.steps)
	})
}

// NewAuditedStudentUpdates binds the command without the row lock the
// production graph adds; the tests drive the write and history order.
func NewAuditedStudentUpdates(students StudentRowUpdate, audit StudentChangeActorRecorder) AuditedStudentUpdates {
	return AuditedStudentUpdates{students: students, audit: audit}
}
