package feedback_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingEngine struct {
	created    []feedback.CreateEntry
	rejections []recordedRejection
}

type recordedRejection struct {
	operation string
	err       error
}

func (*recordingEngine) Available(context.Context) (bool, error) { return true, nil }
func (e *recordingEngine) Submit(_ context.Context, entry feedback.CreateEntry) (feedback.Entry, error) {
	e.created = append(e.created, entry)
	return feedback.Entry{ID: 1, Value: entry.Value, Day: entry.Day, Time: entry.Time, StudentID: entry.StudentID}, nil
}
func (*recordingEngine) SubmitBatch(context.Context, []feedback.CreateEntry) ([]feedback.Entry, error) {
	return nil, nil
}
func (*recordingEngine) LookupEntry(context.Context, int64) (feedback.Entry, error) {
	return feedback.Entry{}, nil
}
func (*recordingEngine) EraseEntry(context.Context, int64) error { return nil }
func (*recordingEngine) FindEntries(context.Context, feedback.Filter) ([]feedback.Entry, error) {
	return nil, nil
}
func (*recordingEngine) DeleteExpired(context.Context) (int, error)          { return 0, nil }
func (*recordingEngine) CountForStudent(context.Context, int64) (int, error) { return 0, nil }
func (e *recordingEngine) ObserveRejection(operation string, _ time.Duration, err error) {
	e.rejections = append(e.rejections, recordedRejection{operation: operation, err: err})
}

func TestModuleNormalizesAndValidatesCreateAtPublicSeam(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := feedback.NewModule(engine)

	entry, err := module.Submit(context.Background(), feedback.CreateEntry{
		Value: "  positive  ", Day: "2026-08-31", Time: "09:07:05", StudentID: 42,
	})

	require.NoError(t, err)
	assert.Equal(t, "positive", entry.Value)
	require.Len(t, engine.created, 1)
	assert.Equal(t, "positive", engine.created[0].Value)

	_, err = module.Submit(context.Background(), feedback.CreateEntry{
		Value: "maybe", Day: "2026-08-31", Time: "09:07:05", StudentID: 42,
	})
	require.ErrorIs(t, err, feedback.ErrInvalidEntryData)
	assert.Len(t, engine.created, 1, "invalid input must not reach persistence")
	require.Len(t, engine.rejections, 1)
	assert.Equal(t, "submit", engine.rejections[0].operation)
	assert.Equal(t, "invalid_entry_data", feedback.ErrorCode(engine.rejections[0].err))
}

func TestModuleRejectsInvalidDateRangesAtPublicSeam(t *testing.T) {
	t.Parallel()
	module := feedback.NewModule(&recordingEngine{})

	_, err := module.FindEntries(context.Background(), feedback.Filter{DayFrom: datePtr("2026-09-02"), DayTo: datePtr("2026-09-01")})

	require.ErrorIs(t, err, feedback.ErrInvalidDateRange)
	var rangeErr *feedback.InvalidDateRangeError
	require.ErrorAs(t, err, &rangeErr)
	assert.Equal(t, feedback.Date("2026-09-02"), rangeErr.StartDate)
}

func TestModuleRejectsWholeBatchBeforePersistence(t *testing.T) {
	t.Parallel()
	module := feedback.NewModule(&recordingEngine{})

	_, err := module.SubmitBatch(context.Background(), []feedback.CreateEntry{
		{Value: feedback.ValuePositive, Day: "2026-08-31", Time: "09:07:05", StudentID: 42},
		{Value: "invalid", Day: "2026-08-31", Time: "09:07:05", StudentID: 42},
	})

	require.ErrorIs(t, err, feedback.ErrInvalidEntryData)
	var batchErr *feedback.BatchOperationError
	require.ErrorAs(t, err, &batchErr)
	assert.Len(t, batchErr.Errors, 1)
}

func TestErrorCodeIsStable(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "entry_not_found", feedback.ErrorCode(feedback.ErrEntryNotFound))
	assert.Equal(t, "invalid_entry_data", feedback.ErrorCode(&feedback.InvalidEntryDataError{Err: errors.New("bad")}))
	assert.Equal(t, "internal_error", feedback.ErrorCode(errors.New("database down")))
}

func datePtr(value feedback.Date) *feedback.Date { return &value }
