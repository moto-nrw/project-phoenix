package enrollment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

type recordedPhaseRows struct{ calls *[]string }

func (r recordedPhaseRows) Phase(_ context.Context, id int64) (*Phase, error) {
	*r.calls = append(*r.calls, "read")
	return &Phase{ID: id, ServiceStartDate: "2026-09-01", ServiceEndDate: "2027-07-31"}, nil
}

// The reader takes the recurrence gate before it reads the phase, reads
// nothing when the gate fails, and only its phase carries booking bounds
// (#6657).
func TestPhaseBoundsReaderLocksBeforeItReads(t *testing.T) {
	t.Parallel()

	var calls []string
	reader := NewPhaseBoundsReader(recordedPhaseRows{calls: &calls}, func(context.Context) error {
		calls = append(calls, "lock")
		return nil
	})
	locked, err := reader.LockedPhase(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, []string{"lock", "read"}, calls)
	from, until, err := locked.BookingWindow()
	require.NoError(t, err)
	assert.Equal(t, calendar.NewDate(2026, 9, 1), from)
	assert.Equal(t, calendar.NewDate(2027, 8, 1), until, "the booking ends exclusively the day after the last day of care")

	calls = nil
	failure := errors.New("gate busy")
	_, err = NewPhaseBoundsReader(recordedPhaseRows{calls: &calls}, func(context.Context) error { return failure }).LockedPhase(context.Background(), 7)
	require.ErrorIs(t, err, ErrPhaseBoundsLock)
	require.ErrorIs(t, err, failure)
	assert.Empty(t, calls, "no read without the gate")

	_, _, err = LockedPhase{}.BookingWindow()
	require.Error(t, err, "a phase not read by the reader has no booking bounds")
}
