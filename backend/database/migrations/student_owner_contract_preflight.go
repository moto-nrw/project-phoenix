package migrations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// studentContractLiveState is read without selecting either retired relation.
// Database names alone do not distinguish production from a restored clone.
type studentContractLiveState struct {
	Database            string
	SystemIdentifier    string
	DatabaseOID         uint32
	CutoverRecordedAt   *time.Time
	Reads               int64
	Writes              int64
	StatisticsReset     time.Time
	StatisticsDealloc   int64
	OldQueryFingerprint string
	TrackingAll         bool
}

func readStudentContractLiveState(ctx context.Context, db bun.IDB) (studentContractLiveState, error) {
	// Include every non-superuser, not only known application roles. Historical
	// statements remain in the fingerprint so another execution changes it.
	// Superuser maintenance is reviewed separately in the operator evidence;
	// applications must never connect as that migration/inspection identity.
	// Match unqualified and quoted names too. Incidental text can conservatively
	// block Contract, but must not silently hide a possible legacy consumer.
	var state studentContractLiveState
	err := db.NewRaw(`SELECT current_database() AS database,
		(SELECT system_identifier::text FROM pg_control_system()) AS system_identifier,
		(SELECT oid FROM pg_database WHERE datname = current_database()) AS database_oid,
		(SELECT max(migrated_at) FROM public.bun_migrations WHERE name = '001015397') AS cutover_recorded_at,
		coalesce(pg_sequence_last_value('users.student_compatibility_reads'), 0) AS reads,
		coalesce(pg_sequence_last_value('users.student_compatibility_writes'), 0) AS writes,
		stats_reset AS statistics_reset, dealloc AS statistics_dealloc,
		(current_setting('pg_stat_statements.track') = 'all'
		 AND current_setting('pg_stat_statements.track_utility')::boolean AND NOT EXISTS (
		 SELECT 1 FROM pg_db_role_setting settings, unnest(settings.setconfig) setting
		 WHERE settings.setdatabase IN (0, (SELECT oid FROM pg_database WHERE datname = current_database()))
		 AND ((setting LIKE 'pg_stat_statements.track=%' AND setting <> 'pg_stat_statements.track=all')
		 OR (setting LIKE 'pg_stat_statements.track_utility=%' AND setting <> 'pg_stat_statements.track_utility=on'))
		)) AS tracking_all,
		(SELECT encode(sha256(convert_to(coalesce(string_agg(
			jsonb_build_array(s.userid, s.queryid, s.toplevel, s.calls, s.rows, s.stats_since)::text,
			'' ORDER BY s.userid, s.queryid, s.toplevel), ''), 'UTF8')), 'hex')
		 FROM public.pg_stat_statements s
		 JOIN pg_roles r ON r.oid = s.userid
		 WHERE s.dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
		 AND NOT r.rolsuper
		 AND replace(s.query, '"', '') ~* '\m(students|students_legacy|expired_privacy_consents)\M'
		) AS old_query_fingerprint
		FROM public.pg_stat_statements_info`).Scan(ctx, &state)
	if err != nil {
		return state, fmt.Errorf("student contract: read live database and statistics identity: %w", err)
	}
	return state, nil
}

// validateStudentContractLiveEvidence binds reviewed evidence to the connected
// database and the release supplied by the caller's trusted build metadata.
// It must run again under the Contract lock, not only at deployment preflight.
func validateStudentContractLiveEvidence(ctx context.Context, db bun.IDB, evidence StudentContractEvidence, policy StudentContractPolicy, release string, now time.Time) error {
	if err := ValidateStudentContractEvidence(evidence, policy, now); err != nil {
		return err
	}
	if !studentContractCommitPattern.MatchString(release) || release != evidence.ReleaseCommit {
		return errors.New("student contract: caller evidence does not match the executing release")
	}
	state, err := readStudentContractLiveState(ctx, db)
	if err != nil {
		return err
	}
	if err := validateStudentContractLiveState(evidence, state); err != nil {
		return err
	}
	if policy.RequireSchoolDay {
		return validateStudentContractSchoolDay(ctx, db, evidence.SchoolDayStart, evidence.SchoolDayEnd)
	}
	return nil
}

// Migrations keep date conversion in PostgreSQL, like the existing backfill
// gates, rather than depending on domain/shared-kernel application packages.
func validateStudentContractSchoolDay(ctx context.Context, db bun.IDB, start, end time.Time) error {
	var weekday bool
	err := db.NewRaw(`SELECT
		(?::timestamptz AT TIME ZONE 'Europe/Berlin')::date = (?::timestamptz AT TIME ZONE 'Europe/Berlin')::date
		AND EXTRACT(ISODOW FROM (?::timestamptz AT TIME ZONE 'Europe/Berlin')) BETWEEN 1 AND 5`, start, end, start).Scan(ctx, &weekday)
	if err != nil {
		return fmt.Errorf("student contract: verify school-day calendar boundary: %w", err)
	}
	if !weekday {
		return errors.New("student contract: school-day evidence must cover one regular Berlin weekday")
	}
	return nil
}

func validateStudentContractLiveState(evidence StudentContractEvidence, state studentContractLiveState) error {
	if !state.TrackingAll {
		return errors.New("student contract: query tracking must include all statements without role/database exclusions")
	}
	if state.Database != evidence.Database || state.SystemIdentifier != evidence.SystemIdentifier || state.DatabaseOID != evidence.DatabaseOID {
		return errors.New("student contract: evidence belongs to a different database or cluster")
	}
	if state.CutoverRecordedAt == nil || state.CutoverRecordedAt.IsZero() || evidence.CutoverAt.Before(*state.CutoverRecordedAt) {
		return errors.New("student contract: observation must follow the database's recorded cutover")
	}
	if evidence.CompatibilityReadsEnd == nil || evidence.CompatibilityWritesEnd == nil ||
		state.Reads != *evidence.CompatibilityReadsEnd || state.Writes != *evidence.CompatibilityWritesEnd {
		return errors.New("student contract: compatibility counters changed after observation")
	}
	if evidence.StatisticsDeallocEnd == nil || !state.StatisticsReset.Equal(evidence.StatisticsResetEnd) || state.StatisticsDealloc != *evidence.StatisticsDeallocEnd {
		return errors.New("student contract: query statistics reset or evicted entries after observation")
	}
	if state.OldQueryFingerprint != evidence.OldQueryFingerprintEnd || state.OldQueryFingerprint == "" {
		return errors.New("student contract: old-object query statistics changed after observation")
	}
	return nil
}
