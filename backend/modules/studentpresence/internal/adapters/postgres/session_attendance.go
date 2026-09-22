package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type sessionAttendanceRow struct {
	bun.BaseModel      `bun:"table:active.activity_session_attendance,alias:attendance"`
	ID                 int64      `bun:"id,pk,autoincrement"`
	TenantID           int64      `bun:"tenant_id,notnull"`
	CreatedAt          time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt          time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	ParticipantID      int64      `bun:"instance_student_id,notnull"`
	Status             string     `bun:"status,notnull"`
	Substatus          *string    `bun:"substatus"`
	Note               *string    `bun:"note"`
	CheckedInAt        *time.Time `bun:"checked_in_at"`
	CheckedOutAt       *time.Time `bun:"checked_out_at"`
	IsUnplanned        bool       `bun:"is_unplanned,notnull"`
	NotScheduled       bool       `bun:"not_scheduled,notnull"`
	ManualStatusAt     *time.Time `bun:"manual_status_at"`
	StudentStatusDayID *int64     `bun:"student_status_day_id"`
	PickupExceptionID  *int64     `bun:"pickup_exception_id"`
}

func (row *sessionAttendanceRow) record() ports.SessionAttendance {
	return ports.SessionAttendance{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		ParticipantID: row.ParticipantID, Status: row.Status, Substatus: row.Substatus, Note: row.Note,
		CheckedInAt: row.CheckedInAt, CheckedOutAt: row.CheckedOutAt, IsUnplanned: row.IsUnplanned,
		NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
		StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID}
}

func sessionAttendanceSelect(db bun.IDB, model any, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`active.activity_session_attendance AS "attendance"`).
		ColumnExpr(`"attendance".id, "attendance".tenant_id, "attendance".created_at, "attendance".updated_at, "attendance".instance_student_id`).
		ColumnExpr(`"attendance".status, "attendance".substatus, "attendance".note, "attendance".checked_in_at, "attendance".checked_out_at`).
		ColumnExpr(`"attendance".is_unplanned, "attendance".not_scheduled, "attendance".manual_status_at`).
		ColumnExpr(`"attendance".student_status_day_id, "attendance".pickup_exception_id`).
		Where(`"attendance".tenant_id = ?`, tenantID).OrderExpr(`"attendance".instance_student_id ASC`)
}

func (s *Store) listSessionAttendance(ctx context.Context, query *bun.SelectQuery, rows *[]sessionAttendanceRow, operation string) ([]ports.SessionAttendance, ports.Stats, error) {
	started := time.Now()
	err := query.Scan(ctx)
	stats := ports.Stats{Queries: 1, Rows: int64(len(*rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("%s: %w", operation, err)
	}
	result := make([]ports.SessionAttendance, 0, len(*rows))
	for i := range *rows {
		result = append(result, (*rows)[i].record())
	}
	return result, stats, nil
}

func (s *Store) ListSessionAttendance(ctx context.Context, participantIDs []int64) ([]ports.SessionAttendance, ports.Stats, error) {
	if len(participantIDs) == 0 {
		return []ports.SessionAttendance{}, ports.Stats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []sessionAttendanceRow{}
	return s.listSessionAttendance(ctx, sessionAttendanceSelect(db, &rows, tenantID).
		Where(`"attendance".instance_student_id IN (?)`, bun.List(participantIDs)), &rows, "list session attendance")
}

func (s *Store) ListSessionAttendanceByStatusDay(ctx context.Context, statusDayID int64) ([]ports.SessionAttendance, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []sessionAttendanceRow{}
	return s.listSessionAttendance(ctx, sessionAttendanceSelect(db, &rows, tenantID).
		Where(`"attendance".student_status_day_id = ?`, statusDayID), &rows, "list session attendance by status day")
}

func (s *Store) ListSessionAttendanceByPickupException(ctx context.Context, pickupExceptionID int64) ([]ports.SessionAttendance, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []sessionAttendanceRow{}
	return s.listSessionAttendance(ctx, sessionAttendanceSelect(db, &rows, tenantID).
		Where(`"attendance".pickup_exception_id = ?`, pickupExceptionID), &rows, "list session attendance by pickup exception")
}

func (s *Store) FindSessionAttendance(ctx context.Context, participantID int64) (ports.SessionAttendance, bool, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.SessionAttendance{}, false, ports.Stats{}, err
	}
	row := sessionAttendanceRow{}
	started := time.Now()
	err = sessionAttendanceSelect(db, &row, tenantID).Where(`"attendance".instance_student_id = ?`, participantID).Scan(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return ports.SessionAttendance{}, false, stats, nil
	}
	if err != nil {
		return ports.SessionAttendance{}, false, stats, fmt.Errorf("find session attendance: %w", err)
	}
	stats.Rows = 1
	return row.record(), true, stats, nil
}

func (s *Store) execSessionAttendance(ctx context.Context, operation string, query *bun.RawQuery) (ports.Stats, error) {
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("%s: %w", operation, err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("%s: %w", operation, err)
	}
	return stats, nil
}

// CheckInParticipants opens observed presence. The conflict guard is the
// old check-in rule: a participant still expected, owned by a care-plan
// absence, or present but already checked out is (re)opened; a participant
// already present stays as it is. A missing row is expected attendance.
func (s *Store) CheckInParticipants(ctx context.Context, participantIDs []int64, at time.Time, walkIn bool) (ports.Stats, error) {
	if len(participantIDs) == 0 {
		return ports.Stats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	return s.execSessionAttendance(ctx, "check in participants", db.NewRaw(`
		INSERT INTO active.activity_session_attendance AS target (tenant_id, instance_student_id, status, checked_in_at, is_unplanned, updated_at)
		SELECT ?, unnest(?::bigint[]), 'present', ?, ?, now()
		ON CONFLICT (tenant_id, instance_student_id) DO UPDATE SET
			status = 'present',
			substatus = CASE WHEN target.student_status_day_id IS NOT NULL OR target.pickup_exception_id IS NOT NULL THEN NULL ELSE target.substatus END,
			student_status_day_id = NULL, pickup_exception_id = NULL,
			checked_in_at = CASE WHEN target.checked_out_at IS NOT NULL THEN EXCLUDED.checked_in_at ELSE COALESCE(target.checked_in_at, EXCLUDED.checked_in_at) END,
			checked_out_at = NULL, updated_at = now()
		WHERE target.status = 'expected' OR target.student_status_day_id IS NOT NULL OR target.pickup_exception_id IS NOT NULL
		   OR (target.status = 'present' AND target.checked_out_at IS NOT NULL)`,
		tenantID, pgdialect.Array(participantIDs), at, walkIn))
}

func (s *Store) CheckOutParticipants(ctx context.Context, participantIDs []int64, at time.Time) (ports.Stats, error) {
	if len(participantIDs) == 0 {
		return ports.Stats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	return s.execSessionAttendance(ctx, "check out participants", db.NewRaw(`
		UPDATE active.activity_session_attendance AS target SET
			checked_out_at = CASE WHEN target.checked_out_at IS NULL OR target.checked_out_at < ?1 THEN ?1 ELSE target.checked_out_at END,
			updated_at = now()
		WHERE target.tenant_id = ?0 AND target.instance_student_id = ANY(?2::bigint[])
		  AND target.status = 'present' AND target.checked_in_at IS NOT NULL AND target.checked_in_at <= ?1`,
		tenantID, at, pgdialect.Array(participantIDs)))
}

func (s *Store) CloseOpenParticipants(ctx context.Context, participantIDs []int64, at time.Time) (ports.Stats, error) {
	if len(participantIDs) == 0 {
		return ports.Stats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	return s.execSessionAttendance(ctx, "close open participants", db.NewRaw(`
		UPDATE active.activity_session_attendance AS target SET checked_out_at = ?1, updated_at = ?1
		WHERE target.tenant_id = ?0 AND target.instance_student_id = ANY(?2::bigint[])
		  AND target.status = 'present' AND target.checked_in_at IS NOT NULL AND target.checked_in_at <= ?1
		  AND target.checked_out_at IS NULL`,
		tenantID, at, pgdialect.Array(participantIDs)))
}

func (s *Store) ReconcileParticipantInterval(ctx context.Context, participantID int64, previousCheckIn time.Time, previousCheckOut *time.Time, checkIn time.Time, checkOut *time.Time) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	return s.execSessionAttendance(ctx, "reconcile participant interval", db.NewRaw(`
		UPDATE active.activity_session_attendance AS target SET checked_in_at = ?2, checked_out_at = ?3, updated_at = now()
		WHERE target.tenant_id = ?0 AND target.instance_student_id = ?1 AND target.status = 'present'
		  AND target.checked_in_at = ?4
		  AND (target.checked_out_at IS NOT DISTINCT FROM ?5 OR (?6 AND target.checked_out_at IS NULL))`,
		tenantID, participantID, checkIn, checkOut, previousCheckIn, previousCheckOut, previousCheckOut != nil))
}

// PatchSessionAttendance applies a manual decision to an existing row or
// writes the decision over expected attendance when no row exists yet.
func (s *Store) PatchSessionAttendance(ctx context.Context, participantID int64, patch ports.SessionAttendancePatch, at time.Time) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	query := db.NewUpdate().TableExpr(`active.activity_session_attendance AS "attendance"`).
		Where(`"attendance".tenant_id = ?`, tenantID).Where(`"attendance".instance_student_id = ?`, participantID)
	query, decided := sessionAttendancePatchSets(query, patch, at)
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("patch session attendance: %w", err)
	}
	if stats.Rows, err = result.RowsAffected(); err != nil {
		return stats, fmt.Errorf("patch session attendance: %w", err)
	}
	if stats.Rows > 0 {
		return stats, nil
	}
	// The participant had no row yet: the patch decides over expected
	// attendance and writes the first row.
	row := sessionAttendancePatchRow(tenantID, participantID, patch, at, decided)
	insertStarted := time.Now()
	_, err = db.NewInsert().Model(&row).ModelTableExpr(`active.activity_session_attendance`).Exec(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(insertStarted)
	if err != nil {
		return stats, fmt.Errorf("patch session attendance: %w", err)
	}
	stats.Rows = 1
	return stats, nil
}

// sessionAttendancePatchSets adds the patched columns to the update and
// reports whether the patch decides the status or substatus by hand.
func sessionAttendancePatchSets(query *bun.UpdateQuery, patch ports.SessionAttendancePatch, at time.Time) (*bun.UpdateQuery, bool) {
	decided := false
	if patch.Status != nil {
		query = query.Set(`status = ?`, *patch.Status).Set(`not_scheduled = FALSE`)
		decided = true
	}
	if patch.SubstatusClear {
		query = query.Set(`substatus = NULL`)
		decided = true
	} else if patch.Substatus != nil {
		query = query.Set(`substatus = ?`, *patch.Substatus)
		decided = true
	}
	if decided {
		query = query.Set(`student_status_day_id = NULL`).Set(`pickup_exception_id = NULL`).Set(`manual_status_at = ?`, at)
	}
	if patch.NoteClear {
		query = query.Set(`note = NULL`)
	} else if patch.Note != nil {
		query = query.Set(`note = ?`, *patch.Note)
	}
	return query.Set(`updated_at = ?`, at), decided
}

func sessionAttendancePatchRow(tenantID, participantID int64, patch ports.SessionAttendancePatch, at time.Time, decided bool) sessionAttendanceRow {
	row := sessionAttendanceRow{TenantID: tenantID, ParticipantID: participantID, Status: "expected", UpdatedAt: at}
	if patch.Status != nil {
		row.Status = *patch.Status
	}
	if patch.Substatus != nil && !patch.SubstatusClear {
		row.Substatus = patch.Substatus
	}
	if patch.Note != nil && !patch.NoteClear {
		row.Note = patch.Note
	}
	if decided {
		row.ManualStatusAt = &at
	}
	return row
}

func (s *Store) TransitionParticipants(ctx context.Context, participantIDs []int64, from, to string, at *time.Time) (ports.Stats, error) {
	if len(participantIDs) == 0 {
		return ports.Stats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	if from != "expected" {
		return s.execSessionAttendance(ctx, "transition participants", db.NewRaw(`
			UPDATE active.activity_session_attendance AS target SET status = ?3, updated_at = COALESCE(?4::timestamptz, now())
			WHERE target.tenant_id = ?0 AND target.instance_student_id = ANY(?1::bigint[]) AND target.status = ?2`,
			tenantID, pgdialect.Array(participantIDs), from, to, at))
	}
	return s.execSessionAttendance(ctx, "transition participants", db.NewRaw(`
		INSERT INTO active.activity_session_attendance AS target (tenant_id, instance_student_id, status, updated_at)
		SELECT ?0, unnest(?1::bigint[]), ?3, COALESCE(?4::timestamptz, now())
		ON CONFLICT (tenant_id, instance_student_id) DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at
		WHERE target.status = ?2`,
		tenantID, pgdialect.Array(participantIDs), from, to, at))
}

func (s *Store) MarkParticipantsNotScheduled(ctx context.Context, participantIDs []int64) (ports.Stats, error) {
	if len(participantIDs) == 0 {
		return ports.Stats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	return s.execSessionAttendance(ctx, "mark participants not scheduled", db.NewRaw(`
		INSERT INTO active.activity_session_attendance AS target (tenant_id, instance_student_id, status, not_scheduled, updated_at)
		SELECT ?0, unnest(?1::bigint[]), 'expected', TRUE, now()
		ON CONFLICT (tenant_id, instance_student_id) DO UPDATE SET
			not_scheduled = TRUE, status = 'expected',
			substatus = CASE WHEN target.student_status_day_id IS NOT NULL OR target.pickup_exception_id IS NOT NULL THEN NULL ELSE target.substatus END,
			student_status_day_id = NULL, pickup_exception_id = NULL, updated_at = EXCLUDED.updated_at
		WHERE target.manual_status_at IS NULL AND (target.status = 'expected'
		   OR (target.status = 'absent' AND target.student_status_day_id IS NOT NULL)
		   OR (target.status = 'absent' AND target.pickup_exception_id IS NOT NULL))`,
		tenantID, pgdialect.Array(participantIDs)))
}

func (s *Store) RestoreSessionAttendance(ctx context.Context, row ports.SessionAttendanceRestore) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	stats, err := s.execSessionAttendance(ctx, "restore session attendance", db.NewRaw(`
		INSERT INTO active.activity_session_attendance AS target
			(tenant_id, instance_student_id, status, substatus, note, checked_in_at, checked_out_at, is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, now())
		ON CONFLICT (tenant_id, instance_student_id) DO UPDATE SET
			status = EXCLUDED.status, substatus = EXCLUDED.substatus, note = EXCLUDED.note,
			checked_in_at = EXCLUDED.checked_in_at, checked_out_at = EXCLUDED.checked_out_at,
			is_unplanned = EXCLUDED.is_unplanned, not_scheduled = EXCLUDED.not_scheduled,
			manual_status_at = EXCLUDED.manual_status_at,
			student_status_day_id = EXCLUDED.student_status_day_id,
			pickup_exception_id = EXCLUDED.pickup_exception_id, updated_at = EXCLUDED.updated_at`,
		tenantID, row.ParticipantID, row.Status, row.Substatus, row.Note, row.CheckedInAt, row.CheckedOutAt,
		row.IsUnplanned, row.NotScheduled, row.ManualStatusAt, row.StudentStatusDayID, row.PickupExceptionID))
	if err != nil {
		return stats, fmt.Errorf("restore attendance row %d: %w", row.ParticipantID, err)
	}
	if stats.Rows != 1 {
		return stats, fmt.Errorf("restore attendance row %d: snapshot mismatch for attendance row: expected 1 rows, updated %d", row.ParticipantID, stats.Rows)
	}
	return stats, nil
}

func (s *Store) ReconnectParticipantPickupExceptions(ctx context.Context, links []ports.ParticipantPickupException) (ports.Stats, error) {
	if len(links) == 0 {
		return ports.Stats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	participantIDs, exceptionIDs := make([]int64, 0, len(links)), make([]int64, 0, len(links))
	for _, link := range links {
		participantIDs = append(participantIDs, link.ParticipantID)
		exceptionIDs = append(exceptionIDs, link.PickupExceptionID)
	}
	return s.execSessionAttendance(ctx, "reconnect participant pickup exceptions", db.NewRaw(`
		INSERT INTO active.activity_session_attendance AS target (tenant_id, instance_student_id, status, pickup_exception_id, updated_at)
		SELECT ?0, link.participant_id, 'expected', link.pickup_exception_id, now()
		FROM unnest(?1::bigint[], ?2::bigint[]) AS link(participant_id, pickup_exception_id)
		ON CONFLICT (tenant_id, instance_student_id) DO UPDATE SET pickup_exception_id = EXCLUDED.pickup_exception_id, updated_at = EXCLUDED.updated_at`,
		tenantID, pgdialect.Array(participantIDs), pgdialect.Array(exceptionIDs)))
}

func (s *Store) LockSessionAttendance(ctx context.Context, participantIDs []int64) (ports.Stats, error) {
	if len(participantIDs) == 0 {
		return ports.Stats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewSelect().TableExpr(`active.activity_session_attendance AS "attendance"`).ColumnExpr(`"attendance".id`).
		Where(`"attendance".tenant_id = ?`, tenantID).Where(`"attendance".instance_student_id IN (?)`, bun.List(participantIDs)).
		OrderExpr(`"attendance".id ASC`).For("UPDATE").Scan(ctx, &ids)
	stats := ports.Stats{Queries: 1, Rows: int64(len(ids)), StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("lock session attendance: %w", err)
	}
	return stats, nil
}

func (s *Store) ApplyStatusDayAbsences(ctx context.Context, absences []ports.StatusDayAbsence, at time.Time) (ports.Stats, error) {
	if len(absences) == 0 {
		return ports.Stats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	participantIDs, dayIDs, substatuses := make([]int64, 0, len(absences)), make([]int64, 0, len(absences)), make([]string, 0, len(absences))
	for _, absence := range absences {
		participantIDs = append(participantIDs, absence.ParticipantID)
		dayIDs = append(dayIDs, absence.StatusDayID)
		substatuses = append(substatuses, absence.Substatus)
	}
	return s.execSessionAttendance(ctx, "apply status day absences", db.NewRaw(`
		INSERT INTO active.activity_session_attendance AS target (tenant_id, instance_student_id, status, substatus, student_status_day_id, updated_at)
		SELECT ?0, absence.participant_id, 'absent', NULLIF(absence.substatus, ''), absence.status_day_id, ?4
		FROM unnest(?1::bigint[], ?2::bigint[], ?3::text[]) AS absence(participant_id, status_day_id, substatus)
		ON CONFLICT (tenant_id, instance_student_id) DO UPDATE SET
			status = 'absent', substatus = EXCLUDED.substatus, student_status_day_id = EXCLUDED.student_status_day_id, updated_at = EXCLUDED.updated_at
		WHERE NOT target.not_scheduled AND (target.status = 'expected' OR target.student_status_day_id IS NOT NULL)`,
		tenantID, pgdialect.Array(participantIDs), pgdialect.Array(dayIDs), pgdialect.Array(substatuses), at))
}

type statusDayReleaseRecord struct {
	ParticipantID    int64   `json:"participant_id"`
	Status           string  `json:"status"`
	Substatus        *string `json:"substatus"`
	ReplacementDayID *int64  `json:"replacement_day_id"`
}

const statusDayReleaseRecordset = `jsonb_to_recordset(?1::jsonb) AS release(participant_id bigint, status text, substatus text, replacement_day_id bigint)`

func encodeStatusDayReleases(releases []ports.StatusDayRelease) (string, error) {
	records := make([]statusDayReleaseRecord, 0, len(releases))
	for _, release := range releases {
		records = append(records, statusDayReleaseRecord(release))
	}
	encoded, err := json.Marshal(records)
	return string(encoded), err
}

func (s *Store) ReleaseStatusDay(ctx context.Context, statusDayID int64, releases []ports.StatusDayRelease, at time.Time) (ports.Stats, error) {
	if len(releases) == 0 {
		return ports.Stats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	payload, err := encodeStatusDayReleases(releases)
	if err != nil {
		return ports.Stats{}, fmt.Errorf("release status day: %w", err)
	}
	return s.execSessionAttendance(ctx, "release status day", db.NewRaw(`
		UPDATE active.activity_session_attendance AS target SET
			status = release.status, substatus = release.substatus, student_status_day_id = release.replacement_day_id, updated_at = ?2
		FROM `+statusDayReleaseRecordset+`
		WHERE target.tenant_id = ?0 AND target.instance_student_id = release.participant_id AND target.student_status_day_id = ?3`,
		tenantID, payload, at, statusDayID))
}

func (s *Store) ReleasePartialAbsence(ctx context.Context, pickupExceptionID int64, releases []ports.StatusDayRelease, at time.Time) (ports.Stats, error) {
	if len(releases) == 0 {
		return ports.Stats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	payload, err := encodeStatusDayReleases(releases)
	if err != nil {
		return ports.Stats{}, fmt.Errorf("release partial absence: %w", err)
	}
	return s.execSessionAttendance(ctx, "release partial absence", db.NewRaw(`
		UPDATE active.activity_session_attendance AS target SET
			status = release.status, substatus = release.substatus, student_status_day_id = release.replacement_day_id,
			pickup_exception_id = NULL, updated_at = ?2
		FROM `+statusDayReleaseRecordset+`
		WHERE target.tenant_id = ?0 AND target.instance_student_id = release.participant_id AND target.pickup_exception_id = ?3`,
		tenantID, payload, at, pickupExceptionID))
}

func (s *Store) ApplyPartialAbsences(ctx context.Context, absences []ports.PartialAbsence, at time.Time) (ports.Stats, error) {
	if len(absences) == 0 {
		return ports.Stats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	participantIDs, exceptionIDs := make([]int64, 0, len(absences)), make([]int64, 0, len(absences))
	for _, absence := range absences {
		participantIDs = append(participantIDs, absence.ParticipantID)
		exceptionIDs = append(exceptionIDs, absence.PickupExceptionID)
	}
	return s.execSessionAttendance(ctx, "apply partial absences", db.NewRaw(`
		INSERT INTO active.activity_session_attendance AS target (tenant_id, instance_student_id, status, substatus, pickup_exception_id, updated_at)
		SELECT ?0, absence.participant_id, 'absent', 'excused', absence.pickup_exception_id, ?3
		FROM unnest(?1::bigint[], ?2::bigint[]) AS absence(participant_id, pickup_exception_id)
		ON CONFLICT (tenant_id, instance_student_id) DO UPDATE SET
			status = 'absent', substatus = 'excused', student_status_day_id = NULL,
			pickup_exception_id = EXCLUDED.pickup_exception_id, updated_at = EXCLUDED.updated_at
		WHERE target.manual_status_at IS NULL AND NOT target.not_scheduled
		  AND (target.status = 'expected' OR (target.status = 'absent' AND target.pickup_exception_id IS NULL AND target.student_status_day_id IS NULL))`,
		tenantID, pgdialect.Array(participantIDs), pgdialect.Array(exceptionIDs), at))
}
