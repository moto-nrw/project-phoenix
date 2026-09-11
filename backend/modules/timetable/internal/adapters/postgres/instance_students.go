package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

type instanceStudentRow struct {
	bun.BaseModel      `bun:"table:instance_students,alias:instance_student"`
	ID                 int64      `bun:"id,pk,autoincrement"`
	TenantID           int64      `bun:"tenant_id,notnull"`
	CreatedAt          time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt          time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	InstanceID         int64      `bun:"instance_id,notnull"`
	StudentID          int64      `bun:"student_id,notnull"`
	RoomID             *int64     `bun:"room_id"`
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

func filterInstanceStudents(query *bun.SelectQuery, filter domain.InstanceStudentFilter) *bun.SelectQuery {
	if needsInstanceStudentActivityJoin(filter) {
		query = query.Join(`INNER JOIN schedule.activity_instances AS "activity_instance"
			ON "activity_instance".id = "instance_student".instance_id
			AND "activity_instance".tenant_id = "instance_student".tenant_id`)
	}
	query = filterInstanceStudentIDs(query, filter)
	query = filterInstanceStudentState(query, filter)
	query = filterInstanceStudentDates(query, filter)
	query = orderInstanceStudents(query, filter)
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit).Offset(filter.Offset)
	}
	return query
}

func needsInstanceStudentActivityJoin(filter domain.InstanceStudentFilter) bool {
	return filter.Date != nil || filter.FromDate != nil || filter.ToDate != nil || filter.CurrentTime != nil ||
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

func filterInstanceStudentState(query *bun.SelectQuery, filter domain.InstanceStudentFilter) *bun.SelectQuery {
	if filter.Status != nil {
		query = query.Where(`"instance_student".status = ?`, *filter.Status)
	}
	if filter.NotScheduledCandidatesOnly {
		query = query.Where(`"instance_student".manual_status_at IS NULL`).WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
			return group.WhereOr(`"instance_student".status = 'expected'`).
				WhereOr(`("instance_student".status = 'absent' AND "instance_student".student_status_day_id IS NOT NULL)`).
				WhereOr(`("instance_student".status = 'absent' AND "instance_student".pickup_exception_id IS NOT NULL)`)
		})
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
		query = query.Where(`"activity_instance".status IN ('planned', 'active')`).
			Where(`"activity_instance".start_time <= ?::time`, *filter.CurrentTime).
			Where(`"activity_instance".end_time > ?::time`, *filter.CurrentTime)
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

type instanceStudentCount struct {
	InstanceID int64 `bun:"instance_id"`
	Count      int   `bun:"count"`
}

func (s *Store) CountNonAbsentInstanceStudents(ctx context.Context, instanceIDs []int64) (map[int64]int, domain.OperationStats, error) {
	result := make(map[int64]int)
	if len(instanceIDs) == 0 {
		return result, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []instanceStudentCount{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`"instance_student".instance_id`).ColumnExpr(`COUNT(*)::int AS count`).
		Where(`"instance_student".tenant_id = ?`, tenantID).Where(`"instance_student".instance_id IN (?)`, bun.List(instanceIDs)).
		Where(`"instance_student".status != 'absent'`).GroupExpr(`"instance_student".instance_id`)
	stats, err := scanAll(ctx, query, "count non-absent instance students")
	for _, row := range rows {
		result[row.InstanceID] = row.Count
	}
	stats.Rows = int64(len(rows))
	return result, stats, err
}

type parallelPresenceRow struct {
	StudentID  int64     `bun:"student_id"`
	InstanceID int64     `bun:"instance_id"`
	Title      string    `bun:"title"`
	StartTime  time.Time `bun:"start_time"`
	EndTime    time.Time `bun:"end_time"`
}

func (s *Store) ListParallelStudentPresence(ctx context.Context, excludeInstanceID int64, date string, studentIDs []int64) ([]domain.ParallelPresence, domain.OperationStats, error) {
	if len(studentIDs) == 0 {
		return []domain.ParallelPresence{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []parallelPresenceRow{}
	query := parallelPresenceSelect(db, &rows, tenantID, excludeInstanceID, date, studentIDs)
	stats, err := scanAll(ctx, query, "list parallel student presence")
	result := make([]domain.ParallelPresence, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.ParallelPresence(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, err
}

func parallelPresenceSelect(db bun.IDB, rows any, tenantID, excludedID int64, date string, studentIDs []int64) *bun.SelectQuery {
	return db.NewSelect().Model(rows).ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`"instance_student".student_id, "activity_instance".id AS instance_id`).
		ColumnExpr(`"activity_instance".title, "activity_instance".start_time, "activity_instance".end_time`).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		Where(`"instance_student".tenant_id = ?`, tenantID).Where(`"instance_student".instance_id != ?`, excludedID).
		Where(`"instance_student".student_id IN (?)`, bun.List(studentIDs)).Where(`"instance_student".status = 'present'`).
		Where(`"instance_student".checked_out_at IS NULL`).Where(`"activity_instance".date = ?::date`, date).
		Where(`"activity_instance".status = 'active'`).OrderExpr(`"activity_instance".start_time DESC, "activity_instance".id DESC`)
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

func (s *Store) UpdateInstanceStudent(ctx context.Context, id int64, fields domain.InstanceStudentFields) (domain.InstanceStudent, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.InstanceStudent{}, false, domain.OperationStats{}, err
	}
	row := instanceStudentRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = updateInstanceStudent(db, &row, tenantID, id, fields).Returning(instanceStudentColumns).Scan(ctx)
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

func updateInstanceStudent(db bun.IDB, row *instanceStudentRow, tenantID, id int64, fields domain.InstanceStudentFields) *bun.UpdateQuery {
	return db.NewUpdate().Model(row).ModelTableExpr(`schedule.instance_students`).
		Set("instance_id = ?", fields.InstanceID).Set("student_id = ?", fields.StudentID).Set("room_id = ?", fields.RoomID).
		Set("status = ?", fields.Status).Set("substatus = ?", fields.Substatus).Set("note = ?", fields.Note).
		Set("checked_in_at = ?", fields.CheckedInAt).Set("checked_out_at = ?", fields.CheckedOutAt).
		Set("is_unplanned = ?", fields.IsUnplanned).Set("not_scheduled = ?", fields.NotScheduled).
		Set("manual_status_at = ?", fields.ManualStatusAt).Set("student_status_day_id = ?", fields.StudentStatusDayID).
		Set("pickup_exception_id = ?", fields.PickupExceptionID).Set("updated_at = NOW()").
		Where("id = ?", id).Where("tenant_id = ?", tenantID)
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

const instanceStudentColumns = `id, tenant_id, created_at, updated_at, instance_id, student_id, room_id, status,
	substatus, note, checked_in_at, checked_out_at, is_unplanned, not_scheduled, manual_status_at,
	student_status_day_id, pickup_exception_id`

func instanceStudentSelect(db bun.IDB, model any, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`"instance_student".id, "instance_student".tenant_id, "instance_student".created_at, "instance_student".updated_at`).
		ColumnExpr(`"instance_student".instance_id, "instance_student".student_id, "instance_student".room_id, "instance_student".status`).
		ColumnExpr(`"instance_student".substatus, "instance_student".note, "instance_student".checked_in_at, "instance_student".checked_out_at`).
		ColumnExpr(`"instance_student".is_unplanned, "instance_student".not_scheduled, "instance_student".manual_status_at`).
		ColumnExpr(`"instance_student".student_status_day_id, "instance_student".pickup_exception_id`).
		Where(`"instance_student".tenant_id = ?`, tenantID)
}

func newInstanceStudentRow(tenantID int64, fields domain.InstanceStudentFields) instanceStudentRow {
	return instanceStudentRow{TenantID: tenantID, InstanceID: fields.InstanceID, StudentID: fields.StudentID,
		RoomID: fields.RoomID, Status: fields.Status, Substatus: fields.Substatus, Note: fields.Note,
		CheckedInAt: fields.CheckedInAt, CheckedOutAt: fields.CheckedOutAt, IsUnplanned: fields.IsUnplanned,
		NotScheduled: fields.NotScheduled, ManualStatusAt: fields.ManualStatusAt,
		StudentStatusDayID: fields.StudentStatusDayID, PickupExceptionID: fields.PickupExceptionID}
}

func instanceStudentToDomain(row instanceStudentRow) domain.InstanceStudent {
	return domain.InstanceStudent{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		InstanceID: row.InstanceID, StudentID: row.StudentID, RoomID: row.RoomID, Status: row.Status,
		Substatus: row.Substatus, Note: row.Note, CheckedInAt: row.CheckedInAt, CheckedOutAt: row.CheckedOutAt,
		IsUnplanned: row.IsUnplanned, NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
		StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID}
}
