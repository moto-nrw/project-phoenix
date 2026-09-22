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

type activitySessionRow struct {
	bun.BaseModel      `bun:"table:active.activity_sessions,alias:session"`
	ID                 int64           `bun:"id,pk,autoincrement"`
	TenantID           int64           `bun:"tenant_id,notnull"`
	CreatedAt          time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt          time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	InstanceID         int64           `bun:"schedule_instance_id,notnull"`
	Status             string          `bun:"status,notnull"`
	ActiveGroupID      *int64          `bun:"active_group_id"`
	StartedBy          *int64          `bun:"started_by"`
	StartedAt          *time.Time      `bun:"started_at"`
	CompletedAt        *time.Time      `bun:"completed_at"`
	CompletedBy        *int64          `bun:"completed_by"`
	ReopenUntil        *time.Time      `bun:"reopen_until"`
	CompletionSnapshot json.RawMessage `bun:"completion_snapshot,type:jsonb"`
}

func (row *activitySessionRow) record() ports.ActivitySession {
	return ports.ActivitySession{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		InstanceID: row.InstanceID, Status: row.Status, ActiveGroupID: row.ActiveGroupID, StartedBy: row.StartedBy,
		StartedAt: row.StartedAt, CompletedAt: row.CompletedAt, CompletedBy: row.CompletedBy, ReopenUntil: row.ReopenUntil,
		CompletionSnapshot: []byte(row.CompletionSnapshot)}
}

const activitySessionColumns = `id, tenant_id, created_at, updated_at, schedule_instance_id, status, active_group_id,
	started_by, started_at, completed_at, completed_by, reopen_until, completion_snapshot`

func activitySessionSelect(db bun.IDB, model any, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`active.activity_sessions AS "session"`).
		ColumnExpr(`"session".id, "session".tenant_id, "session".created_at, "session".updated_at, "session".schedule_instance_id`).
		ColumnExpr(`"session".status, "session".active_group_id, "session".started_by, "session".started_at`).
		ColumnExpr(`"session".completed_at, "session".completed_by, "session".reopen_until, "session".completion_snapshot`).
		Where(`"session".tenant_id = ?`, tenantID)
}

func (s *Store) FindActivitySession(ctx context.Context, instanceID int64) (ports.ActivitySession, bool, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.ActivitySession{}, false, ports.Stats{}, err
	}
	row := activitySessionRow{}
	started := time.Now()
	err = activitySessionSelect(db, &row, tenantID).Where(`"session".schedule_instance_id = ?`, instanceID).Scan(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return ports.ActivitySession{}, false, stats, nil
	}
	if err != nil {
		return ports.ActivitySession{}, false, stats, fmt.Errorf("find activity session: %w", err)
	}
	stats.Rows = 1
	return row.record(), true, stats, nil
}

func (s *Store) ListActivitySessions(ctx context.Context, filter ports.ActivitySessionFilter) ([]ports.ActivitySession, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []activitySessionRow{}
	query := activitySessionSelect(db, &rows, tenantID).OrderExpr(`"session".schedule_instance_id ASC`)
	if len(filter.InstanceIDs) > 0 {
		query = query.Where(`"session".schedule_instance_id IN (?)`, bun.List(filter.InstanceIDs))
	}
	if len(filter.ActiveGroupIDs) > 0 {
		query = query.Where(`"session".active_group_id IN (?)`, bun.List(filter.ActiveGroupIDs))
	}
	if filter.Status != "" {
		query = query.Where(`"session".status = ?`, filter.Status)
	}
	started := time.Now()
	err = query.Scan(ctx)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("list activity sessions: %w", err)
	}
	result := make([]ports.ActivitySession, 0, len(rows))
	for i := range rows {
		result = append(result, rows[i].record())
	}
	return result, stats, nil
}

// SessionExecution reads both owned tables in one statement: the sessions
// that ended among the instances, the attendance rows carrying the
// non-booking marker among the participants.
func (s *Store) SessionExecution(ctx context.Context, filter ports.SessionExecutionFilter) (ports.SessionExecution, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.SessionExecution{}, ports.Stats{}, err
	}
	instanceIDs, participantIDs := filter.InstanceIDs, filter.ParticipantIDs
	if instanceIDs == nil {
		instanceIDs = []int64{}
	}
	if participantIDs == nil {
		participantIDs = []int64{}
	}
	rows := []struct {
		Kind string `bun:"kind"`
		ID   int64  `bun:"id"`
	}{}
	started := time.Now()
	err = db.NewRaw(`SELECT 'completed' AS kind, "session".schedule_instance_id AS id
		FROM active.activity_sessions AS "session"
		WHERE "session".tenant_id = ? AND "session".status = 'completed' AND "session".schedule_instance_id = ANY(?::BIGINT[])
		UNION ALL
		SELECT 'not_scheduled' AS kind, "attendance".instance_student_id AS id
		FROM active.activity_session_attendance AS "attendance"
		WHERE "attendance".tenant_id = ? AND "attendance".not_scheduled AND "attendance".instance_student_id = ANY(?::BIGINT[])`,
		tenantID, pgdialect.Array(instanceIDs), tenantID, pgdialect.Array(participantIDs)).Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return ports.SessionExecution{}, stats, fmt.Errorf("session execution: %w", err)
	}
	result := ports.SessionExecution{CompletedInstanceIDs: []int64{}, NotScheduledParticipantIDs: []int64{}}
	for _, row := range rows {
		if row.Kind == "completed" {
			result.CompletedInstanceIDs = append(result.CompletedInstanceIDs, row.ID)
		} else {
			result.NotScheduledParticipantIDs = append(result.NotScheduledParticipantIDs, row.ID)
		}
	}
	return result, stats, nil
}

func (s *Store) StartActivitySession(ctx context.Context, start ports.ActivitySessionStart) (ports.ActivitySession, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.ActivitySession{}, ports.Stats{}, err
	}
	row := activitySessionRow{TenantID: tenantID, InstanceID: start.InstanceID, Status: "active",
		ActiveGroupID: &start.ActiveGroupID, StartedBy: start.StartedBy, StartedAt: &start.StartedAt}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`active.activity_sessions`).Returning(activitySessionColumns).Scan(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return ports.ActivitySession{}, stats, fmt.Errorf("start activity session: %w", err)
	}
	stats.Rows = 1
	return row.record(), stats, nil
}

func (s *Store) CompleteActivitySession(ctx context.Context, completion ports.ActivitySessionCompletion) (ports.ActivitySession, bool, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.ActivitySession{}, false, ports.Stats{}, err
	}
	row := activitySessionRow{}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(`active.activity_sessions`).
		Set("status = 'completed'").Set("completed_at = ?", completion.CompletedAt).Set("completed_by = ?", completion.CompletedBy).
		Set("reopen_until = ?", completion.ReopenUntil).Set("completion_snapshot = ?", json.RawMessage(completion.CompletionSnapshot)).
		Set("updated_at = ?", completion.CompletedAt).
		Where("tenant_id = ?", tenantID).Where("schedule_instance_id = ?", completion.InstanceID).Where("status = 'active'").
		Returning(activitySessionColumns).Scan(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return ports.ActivitySession{}, false, stats, nil
	}
	if err != nil {
		return ports.ActivitySession{}, false, stats, fmt.Errorf("complete activity session: %w", err)
	}
	stats.Rows = 1
	return row.record(), true, stats, nil
}

// RecordActivitySessionCompleted keeps the nightly contract of the old
// column: an instance is stamped completed whether or not it was started.
func (s *Store) RecordActivitySessionCompleted(ctx context.Context, instanceID int64, at time.Time) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.ExecContext(ctx, `INSERT INTO active.activity_sessions (tenant_id, schedule_instance_id, status, completed_at, updated_at)
		VALUES (?, ?, 'completed', ?, ?)
		ON CONFLICT (tenant_id, schedule_instance_id) DO UPDATE SET status = 'completed', completed_at = EXCLUDED.completed_at, updated_at = EXCLUDED.updated_at`,
		tenantID, instanceID, at, at)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("record activity session completed: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

func (s *Store) ReopenActivitySession(ctx context.Context, instanceID, activeGroupID int64) (ports.ActivitySession, bool, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.ActivitySession{}, false, ports.Stats{}, err
	}
	row := activitySessionRow{}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(`active.activity_sessions`).
		Set("status = 'active'").Set("active_group_id = ?", activeGroupID).Set("completed_at = NULL").Set("completed_by = NULL").
		Set("reopen_until = NULL").Set("completion_snapshot = NULL").Set("updated_at = now()").
		Where("tenant_id = ?", tenantID).Where("schedule_instance_id = ?", instanceID).Where("status = 'completed'").
		Returning(activitySessionColumns).Scan(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return ports.ActivitySession{}, false, stats, nil
	}
	if err != nil {
		return ports.ActivitySession{}, false, stats, fmt.Errorf("reopen activity session: %w", err)
	}
	stats.Rows = 1
	return row.record(), true, stats, nil
}

func (s *Store) CompleteActivitySessionsByGroups(ctx context.Context, activeGroupIDs []int64, at time.Time) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	if len(activeGroupIDs) == 0 {
		return ports.Stats{}, nil
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.activity_sessions").
		Set("status = 'completed'").Set("completed_at = ?", at).Set("updated_at = ?", at).
		Where("tenant_id = ?", tenantID).Where("status = 'active'").Where("active_group_id IN (?)", bun.List(activeGroupIDs)).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("complete activity sessions by groups: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

func (s *Store) DiscardActivitySession(ctx context.Context, instanceID int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Table("active.activity_sessions").Where("tenant_id = ?", tenantID).Where("schedule_instance_id = ?", instanceID).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("discard activity session: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}
