package audit

import (
	"context"
	"testing"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ledgerStore struct {
	commandStore
	cutoffs map[string]time.Time
}

func (s *ledgerStore) record(ledger string, cutoff time.Time) (int64, error) {
	if s.cutoffs == nil {
		s.cutoffs = map[string]time.Time{}
	}
	s.cutoffs[ledger] = cutoff
	return 2, nil
}

func (s *ledgerStore) DeleteExpiredAuthEvents(_ context.Context, cutoff time.Time) (int64, error) {
	return s.record("auth", cutoff)
}

func (s *ledgerStore) DeleteExpiredDataAccessLog(_ context.Context, cutoff time.Time) (int64, error) {
	return s.record("access", cutoff)
}

func (s *ledgerStore) DeleteExpiredDataDeletions(_ context.Context, cutoff time.Time) (int64, error) {
	return s.record("deletions", cutoff)
}

func TestCommandDeletesTheAuditLedgersInsideTheirFixedWindows(t *testing.T) {
	t.Parallel()
	store := &ledgerStore{}
	command, err := NewCommand(store, func(AppendObservation) {})
	require.NoError(t, err)
	before := time.Now()

	deleted, days, err := command.DeleteExpiredAuthEvents(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)
	assert.Equal(t, 90, days)
	assert.False(t, store.cutoffs["auth"].After(before.AddDate(0, 0, -90)), "never newer than 90 days")

	_, _, err = command.DeleteExpiredDataAccessLog(context.Background())
	require.NoError(t, err)
	berlin := time.Now().In(calendar.Berlin)
	assert.Equal(t, time.Date(berlin.Year()-1, time.January, 1, 0, 0, 0, 0, calendar.Berlin), store.cutoffs["access"],
		"an access stays until the end of the following year")

	_, _, err = command.DeleteExpiredDeletionLog(context.Background())
	require.NoError(t, err)
	assert.False(t, store.cutoffs["deletions"].After(before.AddDate(-3, 0, 0)), "never newer than three years")
}

func TestCommandWithoutLedgerStoreRefusesLedgerRetention(t *testing.T) {
	t.Parallel()
	command, err := NewCommand(&commandStore{}, func(AppendObservation) {})
	require.NoError(t, err)

	_, _, err = command.DeleteExpiredAuthEvents(context.Background())
	require.ErrorContains(t, err, "ledger retention store")
}

func TestCommandLogsRetentionWithoutSubject(t *testing.T) {
	t.Parallel()
	store := &commandStore{}
	command, err := NewCommand(store, func(AppendObservation) {})
	require.NoError(t, err)

	require.NoError(t, command.LogRetention(context.Background(), auditModels.DeletionTypeParentMessageRetention, 3, 365))
	entry, ok := store.event.(*auditModels.DataDeletion)
	require.True(t, ok)
	assert.Nil(t, entry.StudentID)
	assert.Nil(t, entry.StaffID)
	assert.Equal(t, 3, entry.RecordsDeleted)
	assert.Equal(t, 365, entry.Metadata["retention_days"])

	require.ErrorContains(t, command.LogRetention(context.Background(), "manual", 1, 1), "unknown data kind")
}
