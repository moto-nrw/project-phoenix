package migrations

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStudentContractSchoolDayUsesBerlinCalendar(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, scenario := range []struct {
		start, end string
		accepted   bool
	}{
		{"2026-09-21T06:00:00+02:00", "2026-09-21T18:00:00+02:00", true},
		{"2026-09-20T06:00:00+02:00", "2026-09-20T18:00:00+02:00", false},
		{"2026-09-21T06:00:00+02:00", "2026-09-22T06:00:00+02:00", false},
		{"2026-09-20T22:30:00Z", "2026-09-21T01:00:00Z", true},
	} {
		start, err := time.Parse(time.RFC3339, scenario.start)
		require.NoError(t, err)
		end, err := time.Parse(time.RFC3339, scenario.end)
		require.NoError(t, err)
		err = validateStudentContractSchoolDay(t.Context(), db, start, end)
		if scenario.accepted {
			require.NoError(t, err)
		} else {
			require.ErrorContains(t, err, "one regular Berlin weekday")
		}
	}
}

func TestStudentContractQuerySnapshotDetectsDirectArchiveAccess(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeContract(t)
	var preloaded bool
	require.NoError(t, db.NewRaw("SELECT 'pg_stat_statements' = ANY(string_to_array(replace(current_setting('shared_preload_libraries'), ' ', ''), ','))").Scan(t.Context(), &preloaded))
	before, err := readStudentContractLiveState(t.Context(), db)
	if !preloaded {
		require.ErrorContains(t, err, "pg_stat_statements must be loaded")
		return
	}
	require.NoError(t, err)
	var count int
	require.NoError(t, db.NewRaw("SELECT count(*) FROM users.students_legacy").Scan(t.Context(), &count))
	maintenance, err := readStudentContractLiveState(t.Context(), db)
	require.NoError(t, err)
	require.Equal(t, before.OldQueryFingerprint, maintenance.OldQueryFingerprint, "superuser inspection is separate from application activity")
	_, err = db.ExecContext(t.Context(), "GRANT SELECT ON users.students_legacy TO phoenix_admin")
	require.NoError(t, err)
	require.NoError(t, db.RunInTx(t.Context(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SET LOCAL ROLE phoenix_admin"); err != nil {
			return err
		}
		return tx.NewRaw(`SELECT count(*) FROM "users"."students_legacy"`).Scan(ctx, &count)
	}))
	after, err := readStudentContractLiveState(t.Context(), db)
	require.NoError(t, err)
	require.Equal(t, before.Reads, after.Reads)
	require.Equal(t, before.Writes, after.Writes)
	require.NotEqual(t, before.OldQueryFingerprint, after.OldQueryFingerprint)
	evidence, _, _ := studentContractEvidenceFixture()
	evidence.Database, evidence.DatabaseOID = after.Database, after.DatabaseOID
	evidence.SystemIdentifier = after.SystemIdentifier
	evidence.CutoverAt = *after.CutoverRecordedAt
	evidence.StatisticsResetEnd = after.StatisticsReset
	evidence.StatisticsDeallocEnd = &after.StatisticsDealloc
	evidence.OldQueryFingerprintEnd = before.OldQueryFingerprint
	require.ErrorContains(t, validateStudentContractLiveState(evidence, after), "old-object query statistics changed")
}

func TestStudentContractRejectsDisabledTrackingAndRoleOverrides(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeContract(t)
	var preloaded bool
	require.NoError(t, db.NewRaw("SELECT 'pg_stat_statements' = ANY(string_to_array(replace(current_setting('shared_preload_libraries'), ' ', ''), ','))").Scan(t.Context(), &preloaded))
	if !preloaded {
		_, err := readStudentContractLiveState(t.Context(), db)
		require.ErrorContains(t, err, "pg_stat_statements must be loaded")
		return
	}
	evidence, _, _ := studentContractEvidenceFixture()
	require.NoError(t, db.RunInTx(t.Context(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, "SET LOCAL pg_stat_statements.track = 'none'")
		require.NoError(t, err)
		state, err := readStudentContractLiveState(ctx, tx)
		require.NoError(t, err, "statistics remain readable while tracking is disabled")
		require.False(t, state.TrackingAll)
		require.ErrorContains(t, validateStudentContractLiveState(evidence, state), "query tracking must include all")
		_, err = tx.ExecContext(ctx, "SET LOCAL pg_stat_statements.track = 'all'; SET LOCAL pg_stat_statements.track_utility = off")
		require.NoError(t, err)
		state, err = readStudentContractLiveState(ctx, tx)
		require.NoError(t, err)
		require.False(t, state.TrackingAll, "COPY and other utility statements must remain visible")
		return nil
	}))
	var database string
	require.NoError(t, db.NewRaw("SELECT current_database()").Scan(t.Context(), &database))
	_, err := db.ExecContext(t.Context(), "ALTER ROLE phoenix_admin IN DATABASE ? SET pg_stat_statements.track = 'none'", bun.Ident(database))
	require.NoError(t, err)
	state, err := readStudentContractLiveState(t.Context(), db)
	require.NoError(t, err)
	require.False(t, state.TrackingAll, "a role-specific override must block the superuser's observation")
	require.ErrorContains(t, validateStudentContractLiveState(evidence, state), "query tracking must include all")
}

func TestStudentContractLiveStateRejectsChangedEvidence(t *testing.T) {
	t.Parallel()
	evidence, _, _ := studentContractEvidenceFixture()
	state := studentContractLiveState{Database: evidence.Database, SystemIdentifier: evidence.SystemIdentifier,
		DatabaseOID: evidence.DatabaseOID, CutoverRecordedAt: &evidence.CutoverAt, StatisticsReset: evidence.StatisticsResetEnd,
		StatisticsDealloc: *evidence.StatisticsDeallocEnd, TrackingAll: true}
	state.OldQueryFingerprint = evidence.OldQueryFingerprintEnd
	require.NoError(t, validateStudentContractLiveState(evidence, state))
	t.Run("observation predates recorded cutover", func(t *testing.T) {
		changed := evidence
		changed.CutoverAt = evidence.CutoverAt.Add(-time.Second)
		require.ErrorContains(t, validateStudentContractLiveState(changed, state), "recorded cutover")
	})
	for _, scenario := range []struct {
		name   string
		change func(*studentContractLiveState)
		want   string
	}{
		{"database", func(s *studentContractLiveState) { s.Database += "_other" }, "different database or cluster"},
		{"tracking disabled", func(s *studentContractLiveState) { s.TrackingAll = false }, "query tracking must include all"},
		{"cluster", func(s *studentContractLiveState) { s.SystemIdentifier += "0" }, "different database or cluster"},
		{"database recreated", func(s *studentContractLiveState) { s.DatabaseOID++ }, "different database or cluster"},
		{"missing cutover", func(s *studentContractLiveState) { s.CutoverRecordedAt = nil }, "recorded cutover"},
		{"reads", func(s *studentContractLiveState) { s.Reads++ }, "compatibility counters changed"},
		{"writes", func(s *studentContractLiveState) { s.Writes++ }, "compatibility counters changed"},
		{"statistics reset", func(s *studentContractLiveState) { s.StatisticsReset = s.StatisticsReset.AddDate(0, 0, 1) }, "query statistics reset or evicted"},
		{"statistics eviction", func(s *studentContractLiveState) { s.StatisticsDealloc++ }, "query statistics reset or evicted"},
		{"direct archive access", func(s *studentContractLiveState) { s.OldQueryFingerprint = strings.Repeat("f", 64) }, "old-object query statistics changed"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			changed := state
			scenario.change(&changed)
			require.ErrorContains(t, validateStudentContractLiveState(evidence, changed), scenario.want)
		})
	}
}

func TestStudentContractLiveEvidenceRejectsWrongExecutingRelease(t *testing.T) {
	t.Parallel()
	evidence, policy, now := studentContractEvidenceFixture()
	require.ErrorContains(t, validateStudentContractLiveEvidence(t.Context(), nil, evidence, policy, strings.Repeat("d", 40), now), "executing release")
}

func TestStudentContractLiveStateRequiresOperationalStatistics(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeContract(t)
	var preloaded bool
	require.NoError(t, db.NewRaw("SELECT 'pg_stat_statements' = ANY(string_to_array(replace(current_setting('shared_preload_libraries'), ' ', ''), ','))").Scan(t.Context(), &preloaded))
	state, err := readStudentContractLiveState(t.Context(), db)
	if !preloaded {
		// An installed but unpreloaded extension is not proof of zero queries.
		// Operational preflight must reject this common test-service setup.
		require.ErrorContains(t, err, "pg_stat_statements must be loaded")
		return
	}
	require.NoError(t, err)
	require.NotEmpty(t, state.Database)
	require.NotEmpty(t, state.SystemIdentifier)
	require.NotZero(t, state.DatabaseOID)
	require.NotNil(t, state.CutoverRecordedAt)
	require.False(t, state.StatisticsReset.IsZero())
	require.Zero(t, state.Reads)
	require.Zero(t, state.Writes)
	// This branch runs in the dedicated operational-evidence instance. Its
	// deliberately short policy is test input, not a production acceptance.
	evidence, policy, _ := studentContractEvidenceFixture()
	now := time.Now().UTC()
	evidence.Database, evidence.DatabaseOID = state.Database, state.DatabaseOID
	evidence.SystemIdentifier = state.SystemIdentifier
	evidence.CutoverAt = *state.CutoverRecordedAt
	evidence.WindowStart = *state.CutoverRecordedAt
	evidence.WindowEnd = now
	evidence.StatisticsResetStart, evidence.StatisticsResetEnd = state.StatisticsReset, state.StatisticsReset
	evidence.StatisticsDeallocStart, evidence.StatisticsDeallocEnd = &state.StatisticsDealloc, &state.StatisticsDealloc
	evidence.OldQueryFingerprintEnd = state.OldQueryFingerprint
	evidence.Backup.CompletedAt = now
	evidence.Backup.RestoreTestedAt = now
	policy.MinimumRollbackWindow = time.Nanosecond
	policy.RequireSchoolDay = false // Synthetic seconds-long database exercise, not operating acceptance.
	require.NoError(t, validateStudentContractLiveEvidence(t.Context(), db, evidence, policy, evidence.ReleaseCommit, now))
	before := studentContractOwnerRows(t, db)
	require.NoError(t, contractStudentOwnerStorageWithEvidence(t.Context(), db, evidence, policy, evidence.ReleaseCommit))
	require.Equal(t, before, studentContractOwnerRows(t, db))
	var removed bool
	require.NoError(t, db.NewRaw("SELECT to_regclass('users.students') IS NULL AND to_regclass('users.students_legacy') IS NULL").Scan(t.Context(), &removed))
	require.True(t, removed)
}
