package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

// instanceStudentRow maps the planning columns of schedule.instance_students.
// The attendance columns still present on the table are a rollback-only
// mirror of active.activity_session_attendance (#2762) and are neither read
// nor written here.
type instanceStudentRow struct {
	bun.BaseModel `bun:"table:instance_students,alias:instance_student"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	InstanceID    int64     `bun:"instance_id,notnull"`
	StudentID     int64     `bun:"student_id,notnull"`
	RoomID        *int64    `bun:"room_id"`
}

type plannedInstanceStudentRow struct {
	ID         int64  `bun:"id"`
	InstanceID int64  `bun:"instance_id"`
	StudentID  int64  `bun:"student_id"`
	Date       string `bun:"date"`
	StartTime  string `bun:"start_time"`
}

func (s *Store) FindInstanceStudent(ctx context.Context, id int64) (domain.InstanceStudent, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.InstanceStudent{}, false, domain.OperationStats{}, err
	}
	row := instanceStudentRow{}
	found, stats, err := scanOne(ctx, instanceStudentSelect(db, &row, tenantID).
		Where(`"instance_student".id = ?`, id), "find instance student")
	return instanceStudentToDomain(row), found, stats, err
}

func (s *Store) ListInstanceStudents(ctx context.Context, filter domain.InstanceStudentFilter) ([]domain.InstanceStudent, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []instanceStudentRow{}
	stats, err := scanAll(ctx, filterInstanceStudents(instanceStudentSelect(db, &rows, tenantID), filter), "list instance students")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.InstanceStudent, 0, len(rows))
	for _, row := range rows {
		result = append(result, instanceStudentToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

// ListPlannedInstanceStudents joins the instance's planning day and block
// start onto every participant.
func (s *Store) ListPlannedInstanceStudents(ctx context.Context, filter domain.InstanceStudentFilter) ([]domain.PlannedInstanceStudent, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []plannedInstanceStudentRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`"instance_student".id, "instance_student".instance_id, "instance_student".student_id`).
		ColumnExpr(`"activity_instance".date::text AS date, "activity_instance".start_time::text AS start_time`).
		Join(instanceStudentActivityJoin).
		Where(`"instance_student".tenant_id = ?`, tenantID)
	query = filterInstanceStudentIDs(query, filter)
	query = filterInstanceStudentDates(query, filter)
	query = query.OrderExpr(`"instance_student".id ASC`)
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit).Offset(filter.Offset)
	}
	stats, err := scanAll(ctx, query, "list planned instance students")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.PlannedInstanceStudent, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.PlannedInstanceStudent(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

const instanceStudentActivityJoin = `INNER JOIN schedule.activity_instances AS "activity_instance"
			ON "activity_instance".id = "instance_student".instance_id
			AND "activity_instance".tenant_id = "instance_student".tenant_id`

func filterInstanceStudents(query *bun.SelectQuery, filter domain.InstanceStudentFilter) *bun.SelectQuery {
	if needsInstanceStudentActivityJoin(filter) {
		query = query.Join(instanceStudentActivityJoin)
	}
	query = filterInstanceStudentIDs(query, filter)
	query = filterInstanceStudentDates(query, filter)
	query = orderInstanceStudents(query, filter)
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit).Offset(filter.Offset)
	}
	return query
}

func needsInstanceStudentActivityJoin(filter domain.InstanceStudentFilter) bool {
	return filter.Date != nil || filter.FromDate != nil || filter.ToDate != nil || filter.CurrentTime != nil ||
		filter.FromClock != nil || filter.ExcludeCancelled ||
		filter.OrderByStudentActivityTime || filter.OrderByActivityDateTime
}

func filterInstanceStudentIDs(query *bun.SelectQuery, filter domain.InstanceStudentFilter) *bun.SelectQuery {
	if len(filter.IDs) > 0 {
		query = query.Where(`"instance_student".id IN (?)`, bun.List(filter.IDs))
	}
	if len(filter.InstanceIDs) > 0 {
		query = query.Where(`"instance_student".instance_id IN (?)`, bun.List(filter.InstanceIDs))
	}
	if len(filter.StudentIDs) > 0 {
		query = query.Where(`"instance_student".student_id IN (?)`, bun.List(filter.StudentIDs))
	}
	return query
}

func filterInstanceStudentDates(query *bun.SelectQuery, filter domain.InstanceStudentFilter) *bun.SelectQuery {
	if filter.Date != nil {
		query = query.Where(`"activity_instance".date = ?::date`, *filter.Date)
	}
	if filter.FromDate != nil {
		query = query.Where(`"activity_instance".date >= ?::date`, *filter.FromDate)
	}
	if filter.ToDate != nil {
		query = query.Where(`"activity_instance".date <= ?::date`, *filter.ToDate)
	}
	if filter.CurrentTime != nil {
		query = query.Where(activityInstanceNotCancelled).
			Where(`"activity_instance".start_time <= ?::time`, *filter.CurrentTime).
			Where(`"activity_instance".end_time > ?::time`, *filter.CurrentTime)
	}
	if filter.FromClock != nil {
		query = query.Where(`"activity_instance".start_time >= ?::time`, *filter.FromClock)
	}
	if filter.ExcludeCancelled {
		query = query.Where(activityInstanceNotCancelled)
	}
	return query
}

func orderInstanceStudents(query *bun.SelectQuery, filter domain.InstanceStudentFilter) *bun.SelectQuery {
	switch {
	case filter.OrderByCreated:
		return query.OrderExpr(`"instance_student".created_at ASC, "instance_student".id ASC`)
	case filter.OrderByInstanceStudent:
		return query.OrderExpr(`"instance_student".instance_id ASC, "instance_student".student_id ASC`)
	case filter.OrderByStudentActivityTime:
		return query.OrderExpr(`"instance_student".student_id ASC, "activity_instance".start_time ASC, "activity_instance".id ASC`)
	case filter.OrderByActivityDateTime:
		return query.OrderExpr(`"activity_instance".date ASC, "activity_instance".start_time ASC, "instance_student".id ASC`)
	default:
		return query
	}
}

func (s *Store) CreateInstanceStudent(ctx context.Context, fields domain.InstanceStudentFields) (domain.InstanceStudent, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.InstanceStudent{}, domain.OperationStats{}, err
	}
	row := newInstanceStudentRow(tenantID, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`schedule.instance_students`).Returning(instanceStudentColumns).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.InstanceStudent{}, stats, classifyWriteError("create instance student", err, &stats)
	}
	stats.Rows = 1
	return instanceStudentToDomain(row), stats, nil
}

// EnsureInstanceStudent adds the child to the roster when it is not on it
// yet. The no-op update on conflict lets RETURNING report the existing row;
// xmax tells an insert from an update.
func (s *Store) EnsureInstanceStudent(ctx context.Context, instanceID, studentID int64) (domain.InstanceStudent, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.InstanceStudent{}, false, domain.OperationStats{}, err
	}
	var row struct {
		instanceStudentRow
		Inserted bool `bun:"inserted"`
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`INSERT INTO schedule.instance_students AS target (tenant_id, instance_id, student_id)
		VALUES (?, ?, ?)
		ON CONFLICT (instance_id, student_id) DO UPDATE SET updated_at = target.updated_at
		RETURNING target.id, target.tenant_id, target.created_at, target.updated_at, target.instance_id, target.student_id, target.room_id,
			(target.xmax = 0) AS inserted`, tenantID, instanceID, studentID).Scan(ctx, &row)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.InstanceStudent{}, false, stats, classifyWriteError("ensure instance student", err, &stats)
	}
	if row.Inserted {
		stats.Rows = 1
	}
	return instanceStudentToDomain(row.instanceStudentRow), row.Inserted, stats, nil
}

func (s *Store) UpdateInstanceStudent(ctx context.Context, id int64, fields domain.InstanceStudentFields) (domain.InstanceStudent, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.InstanceStudent{}, false, domain.OperationStats{}, err
	}
	row := instanceStudentRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(`schedule.instance_students`).
		Set("instance_id = ?", fields.InstanceID).Set("student_id = ?", fields.StudentID).Set("room_id = ?", fields.RoomID).
		Set("updated_at = NOW()").Where("id = ?", id).Where("tenant_id = ?", tenantID).
		Returning(instanceStudentColumns).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.InstanceStudent{}, false, stats, nil
	}
	if err != nil {
		return domain.InstanceStudent{}, false, stats, classifyWriteError("update instance student", err, &stats)
	}
	stats.Rows = 1
	return instanceStudentToDomain(row), true, stats, nil
}

func (s *Store) DeleteInstanceStudent(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().Table("schedule.instance_students").Where("tenant_id = ?", tenantID).Where("id = ?", id), "delete instance student")
}

func (s *Store) DeleteInstanceStudentsByInstance(ctx context.Context, instanceID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().Table("schedule.instance_students").Where("tenant_id = ?", tenantID).Where("instance_id = ?", instanceID), "delete instance students by instance")
}

func (s *Store) ListStudentInstanceRefsBefore(ctx context.Context, cutoff string) ([]domain.StudentInstanceRef, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []domain.StudentInstanceRef{}
	query := db.NewSelect().TableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`"instance_student".student_id, "instance_student".instance_id`).
		Join(instanceStudentActivityJoin).
		Where(`"instance_student".tenant_id = ?`, tenantID).Where(`"activity_instance".date < ?::date`, cutoff).
		OrderExpr(`"instance_student".student_id ASC, "instance_student".instance_id ASC`)
	stats, err := scanAllInto(ctx, query, &rows, "list student instance refs before")
	stats.Rows = int64(len(rows))
	return rows, stats, err
}

func (s *Store) ListPlannedStudentIDs(ctx context.Context, studentIDs []int64, date string) ([]int64, domain.OperationStats, error) {
	if len(studentIDs) == 0 {
		return []int64{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	ids := []int64{}
	query := db.NewSelect().TableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`DISTINCT "instance_student".student_id`).
		Join(instanceStudentActivityJoin).
		Where(`"instance_student".tenant_id = ?`, tenantID).Where(`"instance_student".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"activity_instance".date = ?::date`, date).Where(activityInstanceNotCancelled).
		OrderExpr(`"instance_student".student_id ASC`)
	stats, err := scanAllInto(ctx, query, &ids, "list planned student ids")
	stats.Rows = int64(len(ids))
	return ids, stats, err
}

func (s *Store) LockInstanceStudentAssignments(ctx context.Context, instanceID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	var ids []int64
	return scanAllInto(ctx, db.NewSelect().Table("schedule.instance_students").Column("id").
		Where("tenant_id = ?", tenantID).Where("instance_id = ?", instanceID).OrderExpr("id").For("UPDATE"), &ids, "lock attendance")
}

func (s *Store) LockInstanceStudentsByID(ctx context.Context, ids []int64) (domain.OperationStats, error) {
	if len(ids) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	var locked []int64
	return scanAllInto(ctx, db.NewSelect().Table("schedule.instance_students").Column("id").
		Where("tenant_id = ?", tenantID).Where("id IN (?)", bun.List(ids)).OrderExpr("id").For("UPDATE"), &locked, "lock instance students")
}

const instanceStudentColumns = `id, tenant_id, created_at, updated_at, instance_id, student_id, room_id`

func instanceStudentSelect(db bun.IDB, model any, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`"instance_student".id, "instance_student".tenant_id, "instance_student".created_at, "instance_student".updated_at`).
		ColumnExpr(`"instance_student".instance_id, "instance_student".student_id, "instance_student".room_id`).
		Where(`"instance_student".tenant_id = ?`, tenantID)
}

func newInstanceStudentRow(tenantID int64, fields domain.InstanceStudentFields) instanceStudentRow {
	return instanceStudentRow{TenantID: tenantID, InstanceID: fields.InstanceID, StudentID: fields.StudentID, RoomID: fields.RoomID}
}

func instanceStudentToDomain(row instanceStudentRow) domain.InstanceStudent {
	return domain.InstanceStudent{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		InstanceID: row.InstanceID, StudentID: row.StudentID, RoomID: row.RoomID}
}

func instanceStudentsToDomain(rows []instanceStudentRow) []domain.InstanceStudent {
	result := make([]domain.InstanceStudent, 0, len(rows))
	for _, row := range rows {
		result = append(result, instanceStudentToDomain(row))
	}
	return result
}

// scanRestoredInstanceStudents reads RETURNING rows of a participant insert.
func scanRestoredInstanceStudents(ctx context.Context, query *bun.InsertQuery) ([]domain.InstanceStudent, domain.OperationStats, error) {
	rows := []instanceStudentRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Returning(instanceStudentColumns).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, stats, fmt.Errorf("timetable postgres: restore instance students: %w", err)
	}
	stats.Rows = int64(len(rows))
	return instanceStudentsToDomain(rows), stats, nil
}
