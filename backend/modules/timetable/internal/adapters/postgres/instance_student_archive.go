package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

// rosterRemovalRow maps schedule.grade_transition_roster_removals. The
// attendance columns hold the snapshot the caller read from Student Presence
// when the participant was archived; Timetable stores them verbatim.
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

// ArchivePlannedInstanceStudents deletes the given participants and parks
// them with their attendance snapshots for the transition.
func (s *Store) ArchivePlannedInstanceStudents(ctx context.Context, transitionID int64, entries []domain.RosterArchiveEntry) (int64, domain.OperationStats, error) {
	if len(entries) == 0 {
		return 0, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	ids := make([]int64, 0, len(entries))
	snapshots := make(map[int64]domain.ArchivedAttendance, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ParticipantID)
		snapshots[entry.ParticipantID] = entry.Attendance
	}
	removed := []instanceStudentRow{}
	query := db.NewDelete().Model(&removed).ModelTableExpr(`schedule.instance_students AS "attendance"`).
		Where(`"attendance".tenant_id = ?`, tenantID).Where(`"attendance".id IN (?)`, bun.List(ids)).
		Returning(`"attendance".id, "attendance".tenant_id, "attendance".created_at, "attendance".updated_at, "attendance".instance_id, "attendance".student_id, "attendance".room_id`)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	stats.Rows = int64(len(removed))
	if err != nil {
		return 0, stats, classifyWriteError("archive planned instance students", err, &stats)
	}
	if len(removed) == 0 {
		return 0, stats, nil
	}
	archive := make([]rosterRemovalRow, 0, len(removed))
	for _, value := range removed {
		attendance := snapshots[value.ID]
		archive = append(archive, rosterRemovalRow{TenantID: value.TenantID, TransitionID: transitionID,
			InstanceID: value.InstanceID, StudentID: value.StudentID, RoomID: value.RoomID, Status: attendance.Status,
			Substatus: attendance.Substatus, Note: attendance.Note, IsUnplanned: attendance.IsUnplanned,
			NotScheduled: attendance.NotScheduled, ManualStatusAt: attendance.ManualStatusAt,
			StudentStatusDayID: attendance.StudentStatusDayID})
	}
	queryStats, err := upsertRosterRemovals(ctx, db, archive)
	stats.Add(queryStats)
	if err != nil {
		return 0, stats, err
	}
	return int64(len(removed)), stats, nil
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
			RoomID: value.RoomID, CreatedAt: value.CreatedAt,
			Attendance: domain.ArchivedAttendance{Status: value.Status, Substatus: value.Substatus, Note: value.Note,
				IsUnplanned: value.IsUnplanned, NotScheduled: value.NotScheduled,
				ManualStatusAt: value.ManualStatusAt, StudentStatusDayID: value.StudentStatusDayID}})
	}
	return result
}

// InsertRestoredInstanceStudents puts archived participants back and returns
// the rows it inserted; a roster that lists the child again keeps its row.
func (s *Store) InsertRestoredInstanceStudents(ctx context.Context, fields []domain.InstanceStudentFields) ([]domain.InstanceStudent, domain.OperationStats, error) {
	if len(fields) == 0 {
		return []domain.InstanceStudent{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]instanceStudentRow, 0, len(fields))
	for _, value := range fields {
		rows = append(rows, newInstanceStudentRow(tenantID, value))
	}
	return scanRestoredInstanceStudents(ctx, db.NewInsert().Model(&rows).ModelTableExpr(`schedule.instance_students`).
		On(`CONFLICT (instance_id, student_id) DO NOTHING`))
}
