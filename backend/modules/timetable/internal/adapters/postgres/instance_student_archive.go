package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

type rosterRemovalRow struct {
	bun.BaseModel      `bun:"table:grade_transition_roster_removals,alias:roster_removal"`
	ID                 int64      `bun:"id,pk,autoincrement"`
	TenantID           int64      `bun:"tenant_id,notnull"`
	TransitionID       int64      `bun:"transition_id,notnull"`
	InstanceID         int64      `bun:"instance_id,notnull"`
	StudentID          int64      `bun:"student_id,notnull"`
	RoomID             *int64     `bun:"room_id"`
	Status             string     `bun:"status,notnull"`
	Substatus          *string    `bun:"substatus"`
	Note               *string    `bun:"note"`
	IsUnplanned        bool       `bun:"is_unplanned,notnull"`
	NotScheduled       bool       `bun:"not_scheduled,notnull"`
	ManualStatusAt     *time.Time `bun:"manual_status_at"`
	StudentStatusDayID *int64     `bun:"student_status_day_id"`
	CreatedAt          time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

func (s *Store) ArchivePlannedInstanceStudents(ctx context.Context, transitionID int64, studentIDs []int64, from, currentDate, currentClock string) (int64, domain.OperationStats, error) {
	if len(studentIDs) == 0 {
		return 0, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	removed := []instanceStudentRow{}
	stats, err := deletePlannedInstanceStudents(ctx, db, tenantID, studentIDs, from, currentDate, currentClock, &removed)
	if err != nil || len(removed) == 0 {
		return 0, stats, err
	}
	archive := rosterRemovalRows(removed, transitionID)
	queryStats, err := upsertRosterRemovals(ctx, db, archive)
	stats.Add(queryStats)
	if err != nil {
		return 0, stats, err
	}
	return int64(len(removed)), stats, nil
}

func deletePlannedInstanceStudents(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64, from, currentDate, currentClock string, removed *[]instanceStudentRow) (domain.OperationStats, error) {
	query := db.NewDelete().Model(removed).ModelTableExpr(`schedule.instance_students AS "attendance"`).
		TableExpr(`schedule.activity_instances AS "instance"`).
		Where(`"attendance".instance_id = "instance".id`).Where(`"attendance".tenant_id = ?`, tenantID).
		Where(`"attendance".student_id IN (?)`, bun.List(studentIDs)).Where(`"attendance".checked_in_at IS NULL`).
		Where(`"attendance".checked_out_at IS NULL`).
		Where(`("attendance".manual_status_at IS NULL AND "attendance".status <> 'present') OR ("instance".status = 'planned' AND ("instance".date > ?::date OR ("instance".date = ?::date AND "instance".start_time > ?::time)))`, currentDate, currentDate, currentClock).
		Where(`"instance".date >= ?::date`, from).Where(`"instance".status NOT IN ('completed', 'cancelled')`).
		Returning(`"attendance".*`)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	stats.Rows = int64(len(*removed))
	if err != nil {
		return stats, classifyWriteError("archive planned instance students", err, &stats)
	}
	return stats, nil
}

func rosterRemovalRows(values []instanceStudentRow, transitionID int64) []rosterRemovalRow {
	result := make([]rosterRemovalRow, 0, len(values))
	for _, value := range values {
		result = append(result, rosterRemovalRow{TenantID: value.TenantID, TransitionID: transitionID,
			InstanceID: value.InstanceID, StudentID: value.StudentID, RoomID: value.RoomID, Status: value.Status,
			Substatus: value.Substatus, Note: value.Note, IsUnplanned: value.IsUnplanned,
			NotScheduled: value.NotScheduled, ManualStatusAt: value.ManualStatusAt,
			StudentStatusDayID: value.StudentStatusDayID})
	}
	return result
}

func upsertRosterRemovals(ctx context.Context, db bun.IDB, rows []rosterRemovalRow) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewInsert().Model(&rows).ModelTableExpr(`schedule.grade_transition_roster_removals`).
		On(`CONFLICT (transition_id, instance_id, student_id) DO UPDATE`).
		Set(`room_id = EXCLUDED.room_id`).Set(`status = EXCLUDED.status`).Set(`substatus = EXCLUDED.substatus`).
		Set(`note = EXCLUDED.note`).Set(`is_unplanned = EXCLUDED.is_unplanned`).
		Set(`not_scheduled = EXCLUDED.not_scheduled`).Set(`manual_status_at = EXCLUDED.manual_status_at`).
		Set(`student_status_day_id = EXCLUDED.student_status_day_id`).Set(`created_at = NOW()`).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, classifyWriteError("archive planned instance students", err, &stats)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, classifyWriteError("archive planned instance students", err, &stats)
	}
	return stats, nil
}

func (s *Store) ListRosterRemovalCareDays(ctx context.Context, transitionID int64, studentIDs []int64, from string) ([]domain.CareDay, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []domain.CareDay{}
	query := db.NewSelect().TableExpr(`schedule.grade_transition_roster_removals AS "removal"`).
		ColumnExpr(`DISTINCT "removal".student_id, "instance".date::text AS date`).
		Join(`INNER JOIN schedule.activity_instances AS "instance" ON "instance".id = "removal".instance_id AND "instance".tenant_id = "removal".tenant_id`).
		Where(`"removal".tenant_id = ?`, tenantID).Where(`"removal".transition_id = ?`, transitionID).
		Where(`"removal".student_id IN (?)`, bun.List(studentIDs)).Where(`"instance".date >= ?::date`, from).
		Where(`"instance".status NOT IN ('completed', 'cancelled')`).OrderExpr(`"removal".student_id, "instance".date::text`)
	stats, err := scanAllInto(ctx, query, &rows, "list roster removal care days")
	stats.Rows = int64(len(rows))
	return rows, stats, err
}

func (s *Store) ConsumeRosterRemovals(ctx context.Context, transitionID int64, studentIDs []int64) ([]domain.RosterRemoval, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []rosterRemovalRow{}
	query := db.NewDelete().Model(&rows).ModelTableExpr(`schedule.grade_transition_roster_removals AS "roster_removal"`).
		Where(`"roster_removal".tenant_id = ?`, tenantID).Where(`"roster_removal".transition_id = ?`, transitionID).
		Where(`"roster_removal".student_id IN (?)`, bun.List(studentIDs)).Returning(`"roster_removal".*`)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	stats.Rows = int64(len(rows))
	if err != nil {
		return nil, stats, classifyWriteError("consume roster removals", err, &stats)
	}
	return rosterRemovalsToDomain(rows), stats, nil
}

func rosterRemovalsToDomain(values []rosterRemovalRow) []domain.RosterRemoval {
	result := make([]domain.RosterRemoval, 0, len(values))
	for _, value := range values {
		result = append(result, domain.RosterRemoval{ID: value.ID, TenantID: value.TenantID,
			TransitionID: value.TransitionID, InstanceID: value.InstanceID, StudentID: value.StudentID,
			RoomID: value.RoomID, Status: value.Status, Substatus: value.Substatus, Note: value.Note,
			IsUnplanned: value.IsUnplanned, NotScheduled: value.NotScheduled,
			ManualStatusAt: value.ManualStatusAt, StudentStatusDayID: value.StudentStatusDayID,
			CreatedAt: value.CreatedAt})
	}
	return result
}

func (s *Store) InsertRestoredInstanceStudents(ctx context.Context, fields []domain.InstanceStudentFields) (int64, domain.OperationStats, error) {
	if len(fields) == 0 {
		return 0, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	rows := make([]instanceStudentRow, 0, len(fields))
	for _, value := range fields {
		rows = append(rows, newInstanceStudentRow(tenantID, value))
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewInsert().Model(&rows).ModelTableExpr(`schedule.instance_students`).
		On(`CONFLICT (instance_id, student_id) DO NOTHING`).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, classifyWriteError("restore archived instance students", err, &stats)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return 0, stats, classifyWriteError("restore archived instance students", err, &stats)
	}
	return stats.Rows, stats, nil
}
