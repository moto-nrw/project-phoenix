package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sickNoteStoreDouble struct {
	calls  int
	before time.Time
	err    error
}

func (s *sickNoteStoreDouble) DeleteStaffSickNotesUploadedBefore(_ context.Context, before time.Time) (int, error) {
	s.calls++
	s.before = before
	return 3, s.err
}

type sickNoteWindowDouble struct {
	days int
	err  error
}

func (w sickNoteWindowDouble) StaffSickNoteRetentionDays(context.Context) (int, error) {
	return w.days, w.err
}

func newSickNoteRetentionForTest(store *sickNoteStoreDouble, window sickNoteWindowDouble) *SickNoteRetention {
	retention := NewSickNoteRetention(store, window)
	retention.today = func() calendar.Date { return calendar.NewDate(2026, time.October, 15) }
	return retention
}

func TestSickNoteRetentionDeletesBeforeTheFirstBerlinDayTheWindowKeeps(t *testing.T) {
	t.Parallel()
	for _, days := range []int{365, 90, 1095} {
		store := &sickNoteStoreDouble{}
		deleted, applied, err := newSickNoteRetentionForTest(store, sickNoteWindowDouble{days: days}).DeleteExpiredStaffSickNotes(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 3, deleted)
		assert.Equal(t, days, applied)
		assert.True(t, calendar.NewDate(2026, time.October, 15).AddDays(-days).BerlinMidnight().Equal(store.before),
			"window %d: the cutoff is Berlin midnight of the first kept day", days)
	}
}

func TestSickNoteRetentionDeletesNothingWithoutAValidWindow(t *testing.T) {
	t.Parallel()
	cases := map[string]sickNoteWindowDouble{
		"unreadable window":       {err: errors.New("settings unavailable")},
		"window below the frame":  {days: 89},
		"window above the frame":  {days: 1096},
		"window of zero days":     {days: 0},
		"negative window of days": {days: -365},
	}
	for name, window := range cases {
		store := &sickNoteStoreDouble{}
		deleted, applied, err := newSickNoteRetentionForTest(store, window).DeleteExpiredStaffSickNotes(context.Background())

		require.Error(t, err, name)
		assert.Zero(t, deleted, name)
		assert.Zero(t, applied, name)
		assert.Zero(t, store.calls, "%s: nothing is deleted on a wrong window", name)
	}
}

func TestSickNoteRetentionReportsAFailedDeletion(t *testing.T) {
	t.Parallel()
	store := &sickNoteStoreDouble{err: errors.New("cutoff refused")}

	deleted, applied, err := newSickNoteRetentionForTest(store, sickNoteWindowDouble{days: 365}).DeleteExpiredStaffSickNotes(context.Background())

	require.ErrorContains(t, err, "cutoff refused")
	assert.Zero(t, deleted)
	assert.Zero(t, applied)
}
