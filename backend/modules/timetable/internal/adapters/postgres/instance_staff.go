package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

type instanceStaffRow struct {
	bun.BaseModel `bun:"table:instance_staff,alias:instance_staff"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	InstanceID    int64     `bun:"instance_id,notnull"`
	StaffID       int64     `bun:"staff_id,notnull"`
	RoomID        *int64    `bun:"room_id"`
	IsPrimary     bool      `bun:"is_primary,notnull"`
	IsSubstitute  bool      `bun:"is_substitute,notnull"`
	IsAbsent      bool      `bun:"is_absent,notnull"`
	AbsenceReason *string   `bun:"absence_reason"`
	SickAbsenceID *int64    `bun:"sick_absence_id"`
}

func (s *Store) FindInstanceStaff(ctx context.Context, id int64) (domain.InstanceStaff, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.InstanceStaff{}, false, domain.OperationStats{}, err
	}
	row := instanceStaffRow{}
	found, stats, err := scanOne(ctx, instanceStaffSelect(db, &row, tenantID).
		Where(`"instance_staff".id = ?`, id), "find instance staff")
	return instanceStaffToDomain(row), found, stats, err
}

func (s *Store) ListInstanceStaff(ctx context.Context, filter domain.InstanceStaffFilter) ([]domain.InstanceStaff, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []instanceStaffRow{}
	query := filterInstanceStaff(instanceStaffSelect(db, &rows, tenantID), filter)
	stats, err := scanAll(ctx, query, "list instance staff")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.InstanceStaff, 0, len(rows))
	for _, row := range rows {
		result = append(result, instanceStaffToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func filterInstanceStaff(query *bun.SelectQuery, filter domain.InstanceStaffFilter) *bun.SelectQuery {
	if needsActivityInstanceJoin(filter) {
		query = query.Join(`INNER JOIN schedule.activity_instances AS "activity_instance"
			ON "activity_instance".id = "instance_staff".instance_id
			AND "activity_instance".tenant_id = "instance_staff".tenant_id`)
	}
	query = filterInstanceStaffIDs(query, filter)
	query = filterInstanceStaffDates(query, filter)
	query = orderInstanceStaff(query, filter)
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit).Offset(filter.Offset)
	}
	return query
}

func needsActivityInstanceJoin(filter domain.InstanceStaffFilter) bool {
	return filter.Date != nil || filter.FromDate != nil || filter.ToDate != nil ||
		filter.OrderByActivityTime || filter.OrderByActivityDateTime
}

func filterInstanceStaffIDs(query *bun.SelectQuery, filter domain.InstanceStaffFilter) *bun.SelectQuery {
	if len(filter.IDs) > 0 {
		query = query.Where(`"instance_staff".id IN (?)`, bun.List(filter.IDs))
	}
	if len(filter.InstanceIDs) > 0 {
		query = query.Where(`"instance_staff".instance_id IN (?)`, bun.List(filter.InstanceIDs))
	}
	if len(filter.StaffIDs) > 0 {
		query = query.Where(`"instance_staff".staff_id IN (?)`, bun.List(filter.StaffIDs))
	}
	if filter.SickAbsenceID != nil {
		query = query.Where(`"instance_staff".sick_absence_id = ?`, *filter.SickAbsenceID)
	}
	return query
}

func filterInstanceStaffDates(query *bun.SelectQuery, filter domain.InstanceStaffFilter) *bun.SelectQuery {
	if filter.Date != nil {
		query = query.Where(`"activity_instance".date = ?::date`, *filter.Date)
	}
	if filter.FromDate != nil {
		query = query.Where(`"activity_instance".date >= ?::date`, *filter.FromDate)
	}
	if filter.ToDate != nil {
		query = query.Where(`"activity_instance".date <= ?::date`, *filter.ToDate)
	}
	return query
}

func orderInstanceStaff(query *bun.SelectQuery, filter domain.InstanceStaffFilter) *bun.SelectQuery {
	switch {
	case filter.OrderByCreated:
		return query.OrderExpr(`"instance_staff".created_at ASC, "instance_staff".id ASC`)
	case filter.OrderByInstanceAndCreated:
		return query.OrderExpr(`"instance_staff".instance_id ASC, "instance_staff".created_at ASC, "instance_staff".id ASC`)
	case filter.OrderByActivityTime:
		return query.OrderExpr(`"activity_instance".start_time ASC, "instance_staff".id ASC`)
	case filter.OrderByActivityDateTime:
		return query.OrderExpr(`"activity_instance".date ASC, "activity_instance".start_time ASC, "instance_staff".id ASC`)
	default:
		return query
	}
}

func (s *Store) CountNonAbsentInstanceStaff(ctx context.Context, instanceIDs []int64) (map[int64]int, domain.OperationStats, error) {
	result := make(map[int64]int)
	if len(instanceIDs) == 0 {
		return result, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []instanceStaffCount{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`schedule.instance_staff AS "instance_staff"`).
		ColumnExpr(`"instance_staff".instance_id`).ColumnExpr(`COUNT(*)::int AS count`).
		Where(`"instance_staff".tenant_id = ?`, tenantID).
		Where(`"instance_staff".instance_id IN (?)`, bun.List(instanceIDs)).
		Where(`"instance_staff".is_absent = FALSE`).GroupExpr(`"instance_staff".instance_id`)
	stats, err := scanAll(ctx, query, "count non-absent instance staff")
	if err != nil {
		return nil, stats, err
	}
	for _, row := range rows {
		result[row.InstanceID] = row.Count
	}
	stats.Rows = int64(len(rows))
	return result, stats, nil
}

type instanceStaffCount struct {
	InstanceID int64 `bun:"instance_id"`
	Count      int   `bun:"count"`
}

func (s *Store) CreateInstanceStaff(ctx context.Context, fields domain.InstanceStaffFields) (domain.InstanceStaff, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.InstanceStaff{}, domain.OperationStats{}, err
	}
	row := newInstanceStaffRow(tenantID, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`schedule.instance_staff`).
		Returning(instanceStaffColumns).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.InstanceStaff{}, stats, classifyWriteError("create instance staff", err, &stats)
	}
	stats.Rows = 1
	return instanceStaffToDomain(row), stats, nil
}

func (s *Store) UpdateInstanceStaff(ctx context.Context, id int64, fields domain.InstanceStaffFields) (domain.InstanceStaff, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.InstanceStaff{}, false, domain.OperationStats{}, err
	}
	row := instanceStaffRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(`schedule.instance_staff`).
		Set("instance_id = ?", fields.InstanceID).Set("staff_id = ?", fields.StaffID).
		Set("room_id = ?", fields.RoomID).Set("is_primary = ?", fields.IsPrimary).
		Set("is_substitute = ?", fields.IsSubstitute).Set("is_absent = ?", fields.IsAbsent).
		Set("absence_reason = ?", fields.AbsenceReason).Set("sick_absence_id = ?", fields.SickAbsenceID).
		Where("id = ?", id).Where("tenant_id = ?", tenantID).Returning(instanceStaffColumns).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.InstanceStaff{}, false, stats, nil
	}
	if err != nil {
		return domain.InstanceStaff{}, false, stats, classifyWriteError("update instance staff", err, &stats)
	}
	stats.Rows = 1
	return instanceStaffToDomain(row), true, stats, nil
}

func (s *Store) PatchInstanceStaff(ctx context.Context, id int64, fields domain.InstanceStaffFields, columns []string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewUpdate().Table("schedule.instance_staff").Where("id = ?", id).Where("tenant_id = ?", tenantID)
	for _, column := range columns {
		if column == "is_primary" {
			query = query.Set("is_primary = ?", fields.IsPrimary)
		} else {
			query = query.Set("sick_absence_id = ?", fields.SickAbsenceID)
		}
	}
	stats, err := execMeasuredWrite(ctx, query, "patch instance staff")
	return stats.Rows, stats, err
}

func (s *Store) DeleteInstanceStaff(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().Table("schedule.instance_staff").
		Where("tenant_id = ?", tenantID).Where("id = ?", id), "delete instance staff")
}

func (s *Store) DeleteInstanceStaffByInstance(ctx context.Context, instanceID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().Table("schedule.instance_staff").
		Where("tenant_id = ?", tenantID).Where("instance_id = ?", instanceID), "delete instance staff by instance")
}

func (s *Store) DeleteUpcomingInstanceStaff(ctx context.Context, staffID int64, after string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewDelete().TableExpr(`schedule.instance_staff AS "instance_staff"`).
		Where(`"instance_staff".tenant_id = ?`, tenantID).Where(`"instance_staff".staff_id = ?`, staffID).
		Where(`"instance_staff".instance_id IN (
			SELECT "activity_instance".id FROM schedule.activity_instances AS "activity_instance"
			WHERE "activity_instance".tenant_id = ?
			  AND ("activity_instance".date > ?::date OR
			       ("activity_instance".date = ?::date AND "activity_instance".status = 'planned'))
		)`, tenantID, after, after)
	stats, err := execMeasuredWrite(ctx, query, "delete upcoming instance staff")
	return stats.Rows, stats, err
}

const instanceStaffColumns = `id, tenant_id, created_at, updated_at, instance_id, staff_id,
	room_id, is_primary, is_substitute, is_absent, absence_reason, sick_absence_id`

func instanceStaffSelect(db bun.IDB, model any, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`schedule.instance_staff AS "instance_staff"`).
		ColumnExpr(`"instance_staff".id, "instance_staff".tenant_id, "instance_staff".created_at, "instance_staff".updated_at`).
		ColumnExpr(`"instance_staff".instance_id, "instance_staff".staff_id, "instance_staff".room_id`).
		ColumnExpr(`"instance_staff".is_primary, "instance_staff".is_substitute, "instance_staff".is_absent`).
		ColumnExpr(`"instance_staff".absence_reason, "instance_staff".sick_absence_id`).
		Where(`"instance_staff".tenant_id = ?`, tenantID)
}

func newInstanceStaffRow(tenantID int64, fields domain.InstanceStaffFields) instanceStaffRow {
	return instanceStaffRow{
		TenantID: tenantID, InstanceID: fields.InstanceID, StaffID: fields.StaffID, RoomID: fields.RoomID,
		IsPrimary: fields.IsPrimary, IsSubstitute: fields.IsSubstitute, IsAbsent: fields.IsAbsent,
		AbsenceReason: fields.AbsenceReason, SickAbsenceID: fields.SickAbsenceID,
	}
}

func instanceStaffToDomain(row instanceStaffRow) domain.InstanceStaff {
	return domain.InstanceStaff{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		InstanceID: row.InstanceID, StaffID: row.StaffID, RoomID: row.RoomID, IsPrimary: row.IsPrimary,
		IsSubstitute: row.IsSubstitute, IsAbsent: row.IsAbsent, AbsenceReason: row.AbsenceReason,
		SickAbsenceID: row.SickAbsenceID,
	}
}
