package audit

import (
	"context"
	"testing"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
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

func (s *ledgerStore) DeleteExpiredProtocol(_ context.Context, kind string, cutoff time.Time) (int64, error) {
	return s.record(kind, cutoff)
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
	// The database evaluates its windows in UTC (migration 1.15.534): 90 days
	// are 90 times 24 hours, three years the same UTC clock time.
	assert.False(t, store.cutoffs["auth"].After(before.Add(-90*24*time.Hour)), "never newer than 90 days")

	_, _, err = command.DeleteExpiredDataAccessLog(context.Background())
	require.NoError(t, err)
	berlin := time.Now().In(calendar.Berlin)
	assert.Equal(t, time.Date(berlin.Year()-1, time.January, 1, 0, 0, 0, 0, calendar.Berlin), store.cutoffs["access"],
		"an access stays until the end of the following year")

	_, _, err = command.DeleteExpiredDeletionLog(context.Background())
	require.NoError(t, err)
	assert.False(t, store.cutoffs["deletions"].After(before.UTC().AddDate(-3, 0, 0)), "never newer than three years")
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

// The access log and the operator ledger keep a row until the end of the
// year after it (#3956, #4047): the cutoff is 1 January of last year
// (Berlin), read a minute early so a lagging database clock agrees.
func TestFollowingYearCutoffIsNewYearOfLastYear(t *testing.T) {
	t.Parallel()
	newYear := func(year int) time.Time { return time.Date(year, time.January, 1, 0, 0, 0, 0, calendar.Berlin) }
	at := func(year int, month time.Month, day, hour, minute, second int) time.Time {
		return time.Date(year, month, day, hour, minute, second, 0, calendar.Berlin)
	}
	assert.Equal(t, newYear(2025), followingYearCutoff(at(2026, time.October, 15, 12, 0, 0)))
	assert.Equal(t, newYear(2025), followingYearCutoff(at(2027, time.January, 1, 0, 0, 30)), "within the margin")
	assert.Equal(t, newYear(2026), followingYearCutoff(at(2027, time.January, 1, 0, 1, 30)))
}

type operatorLedgerStore struct {
	commandStore
	called bool
}

func (s *operatorLedgerStore) DeleteExpiredOperatorAuditLog(context.Context) (int64, error) {
	s.called = true
	return 4, nil
}

func TestCommandDeletesTheOperatorLedgerAtTheFollowingYearCutoff(t *testing.T) {
	t.Parallel()
	store := &operatorLedgerStore{}
	command, err := NewCommand(store, func(AppendObservation) {})
	require.NoError(t, err)

	deleted, err := command.DeleteExpiredOperatorAuditLog(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 4, deleted)
	assert.True(t, store.called)

	plain, err := NewCommand(&commandStore{}, func(AppendObservation) {})
	require.NoError(t, err)
	_, err = plain.DeleteExpiredOperatorAuditLog(context.Background())
	require.ErrorContains(t, err, "operator ledger retention store")
}

func TestCommandDeletesTheProtocolsInsideTheirFixedWindows(t *testing.T) {
	t.Parallel()
	store := &ledgerStore{}
	command, err := NewCommand(store, func(AppendObservation) {})
	require.NoError(t, err)
	before := time.Now()

	for _, kind := range ProtocolRetentionKinds {
		deleted, _, err := command.DeleteExpiredProtocol(context.Background(), kind)
		require.NoError(t, err)
		assert.Equal(t, 2, deleted)
	}

	berlin := time.Now().In(calendar.Berlin)
	lastYearStart := time.Date(berlin.Year()-1, time.January, 1, 0, 0, 0, 0, calendar.Berlin)
	for _, kind := range []string{
		auditModels.DeletionTypeClassListChangeRetention,
		auditModels.DeletionTypeSubstitutionChangeRetention,
		auditModels.DeletionTypeFileEventRetention,
	} {
		assert.Equal(t, lastYearStart, store.cutoffs[kind], "%s stays until the end of the following year", kind)
	}
	for _, kind := range []string{
		auditModels.DeletionTypeStudentDeletionLogRetention,
		auditModels.DeletionTypeEnrollmentDeletionLogRetention,
		auditModels.DeletionTypeEnrollmentRestorationLogRetention,
		auditModels.DeletionTypeTimeTrackingDeletionLogRetention,
		auditModels.DeletionTypeStaffMasterDataChangeRetention,
		auditModels.DeletionTypePersonnelNumberChangeRetention,
		auditModels.DeletionTypeGuardianFinancialChangeRetention,
	} {
		assert.False(t, store.cutoffs[kind].After(before.UTC().AddDate(-3, 0, 0)), "%s is never deleted before three years", kind)
	}

	_, _, err = command.DeleteExpiredProtocol(context.Background(), auditModels.DeletionTypeAuthEventRetention)
	require.ErrorContains(t, err, "unknown protocol", "only the protocols of #4044 go through this path")
}
