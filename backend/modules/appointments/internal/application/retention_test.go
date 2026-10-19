package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type retentionStore struct {
	listed, locked []appointments.RetentionCandidate
	lockedIDs      []int64
	deletedIDs     []int64
}

func (s *retentionStore) ListRetentionCandidates(context.Context, appointments.Date) ([]appointments.RetentionCandidate, error) {
	return s.listed, nil
}

func (s *retentionStore) LockRetentionCandidates(_ context.Context, _ appointments.Date, ids []int64) ([]appointments.RetentionCandidate, error) {
	s.lockedIDs = append([]int64(nil), ids...)
	return s.locked, nil
}

func (s *retentionStore) DeleteAppointments(_ context.Context, ids []int64) (int, error) {
	s.deletedIDs = append([]int64(nil), ids...)
	return len(ids), nil
}

func TestRetentionRereadsCandidateBeforeDeleting(t *testing.T) {
	t.Parallel()
	const retentionDays = 30
	cutoff := appointments.Date(calendar.TodayDate().AddDays(-retentionDays).String())
	store := &retentionStore{
		listed: []appointments.RetentionCandidate{{Appointment: &appointments.Appointment{
			ID: 1, StartDate: cutoff.AddDays(-2), EndDate: cutoff.AddDays(-1),
		}}},
		locked: []appointments.RetentionCandidate{{Appointment: &appointments.Appointment{
			ID: 1, StartDate: cutoff.AddDays(-2), EndDate: cutoff,
		}}},
	}

	deleted, err := NewRetention(store).DeleteExpiredAppointments(context.Background(), retentionDays)

	require.NoError(t, err)
	assert.Zero(t, deleted)
	assert.Equal(t, []int64{1}, store.lockedIDs)
	assert.Empty(t, store.deletedIDs, "an appointment extended after the first scan must stay")
}
