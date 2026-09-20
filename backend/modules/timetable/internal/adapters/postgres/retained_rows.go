package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// The rows below keep the retained persistence shapes of class arrival
// exceptions, staff notices and activity reopening that
// database/repositories/schedule served until #3220. The methods return
// driver errors unchanged so the compatibility composition keeps the legacy
// error contract of each operation.

const (
	classArrivalExceptionTable = `education.class_arrival_exceptions AS "class_arrival_exception"`
	staffNoticeTable           = `users.staff_notices AS "staff_notice"`
	staffNoticeAckTable        = `users.staff_notice_acks AS "sna"`
)

// FindClassArrivalExceptions loads the exceptions of the normalized classes
// inside [from, to], ordered by date and class.
func (s *Store) FindClassArrivalExceptions(ctx context.Context, classes []string, from, to schedule.Date) ([]*schedule.ClassArrivalException, error) {
	rows := make([]*schedule.ClassArrivalException, 0)
	query, err := s.classArrivalExceptionQuery(ctx, classes, from, to, &rows)
	if err != nil {
		return nil, err
	}
	err = query.Scan(ctx)
	return rows, err
}

func (s *Store) classArrivalExceptionQuery(ctx context.Context, classes []string, from, to schedule.Date, rows *[]*schedule.ClassArrivalException) (*bun.SelectQuery, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	return db.NewSelect().
		Model(rows).
		ModelTableExpr(classArrivalExceptionTable).
		Where(`LOWER(BTRIM("class_arrival_exception".school_class)) IN (?)`, bun.List(classes)).
		Where(`"class_arrival_exception".date >= ?`, from).
		Where(`"class_arrival_exception".date <= ?`, to).
		OrderExpr(`"class_arrival_exception".date ASC, LOWER(BTRIM("class_arrival_exception".school_class)) ASC`).
		Where(`"class_arrival_exception".tenant_id = ?`, tenantID), nil
}

// UpsertClassArrivalException replaces the exception of one class and date.
// The unique index on the normalized class plus date is the race-safe
// backstop.
func (s *Store) UpsertClassArrivalException(ctx context.Context, row *schedule.ClassArrivalException) error {
	query, err := s.classArrivalExceptionUpsertQuery(ctx, row)
	if err != nil {
		return err
	}
	return query.Scan(ctx)
}

func (s *Store) classArrivalExceptionUpsertQuery(ctx context.Context, row *schedule.ClassArrivalException) (*bun.InsertQuery, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	return db.NewInsert().
		Model(row).
		ModelTableExpr(`education.class_arrival_exceptions`).
		On("CONFLICT (tenant_id, (LOWER(BTRIM(school_class))), date) DO UPDATE").
		Set("arrival_time = EXCLUDED.arrival_time").
		Set("reason = EXCLUDED.reason").
		Set("school_class = EXCLUDED.school_class").
		Set("created_by = EXCLUDED.created_by").
		Set("origin = EXCLUDED.origin").
		Set("updated_at = NOW()").
		Returning("*"), nil
}

// DeleteClassArrivalException removes the exception of one normalized class
// and date and returns the number of deleted rows.
func (s *Store) DeleteClassArrivalException(ctx context.Context, class string, date schedule.Date) (int64, error) {
	query, err := s.classArrivalExceptionDeleteQuery(ctx, class, date)
	if err != nil {
		return 0, err
	}
	result, err := query.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) classArrivalExceptionDeleteQuery(ctx context.Context, class string, date schedule.Date) (*bun.DeleteQuery, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	return db.NewDelete().
		Model((*schedule.ClassArrivalException)(nil)).
		ModelTableExpr(classArrivalExceptionTable).
		Where(`LOWER(BTRIM("class_arrival_exception".school_class)) = ?`, class).
		Where(`"class_arrival_exception".date = ?`, date).
		Where(`"class_arrival_exception".tenant_id = ?`, tenantID), nil
}

// ReopenCompletedActivityInstance returns a completed instance to active. The
// caller reads the changed row count from the result.
func (s *Store) ReopenCompletedActivityInstance(ctx context.Context, instanceID, activeGroupID int64) (sql.Result, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	return db.NewUpdate().Table("schedule.activity_instances").
		Set("status = 'active'").
		Set("active_group_id = ?", activeGroupID).
		Set("completed_at = NULL").
		Set("completed_by = NULL").
		Set("reopen_until = NULL").
		Set("completion_snapshot = NULL").
		Where("id = ? AND status = 'completed'", instanceID).
		Where("tenant_id = ?", tenantID).
		Exec(ctx)
}

// StaffNotice is the persisted shape of users.staff_notices. It mirrors
// the retained users.StaffNotice model field for field; WherePK uses the
// "staff_notice" alias.
type StaffNotice struct {
	bun.BaseModel           `bun:"table:staff_notices,alias:staff_notice"`
	ID                      int64          `bun:"id,pk,autoincrement"`
	CreatedAt               time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt               time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID                int64          `bun:"tenant_id,notnull"`
	Title                   string         `bun:"title,notnull"`
	Body                    string         `bun:"body,notnull"`
	Priority                string         `bun:"priority,notnull"`
	Audience                string         `bun:"audience,notnull,default:'all'"`
	ValidFrom               schedule.Date  `bun:"valid_from,notnull,type:date"`
	ValidUntil              *schedule.Date `bun:"valid_until,type:date"`
	Weekdays                []int16        `bun:"weekdays,array,notnull"`
	WeekPattern             int            `bun:"week_pattern,notnull,default:0"`
	RequiresAcknowledgement bool           `bun:"requires_acknowledgement,notnull"`
	Active                  bool           `bun:"active,notnull"`
	CreatedBy               int64          `bun:"created_by,notnull"`
}

// StaffNoticeAck is the persisted shape of users.staff_notice_acks.
type StaffNoticeAck struct {
	bun.BaseModel  `bun:"table:staff_notice_acks,alias:sna"`
	TenantID       int64     `bun:"tenant_id,notnull"`
	NoticeID       int64     `bun:"notice_id,pk"`
	AccountID      int64     `bun:"account_id,pk"`
	AcknowledgedAt time.Time `bun:"acknowledged_at,nullzero,notnull,default:current_timestamp"`
}

// StaffNoticeCount is the acknowledgement count of one notice.
type StaffNoticeCount struct {
	NoticeID int64 `bun:"notice_id"`
	Count    int   `bun:"count"`
}

// CreateStaffNotice inserts the row and fills its generated columns.
func (s *Store) CreateStaffNotice(ctx context.Context, row *StaffNotice) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	_, err = db.NewInsert().Model(row).ModelTableExpr("users.staff_notices").Exec(ctx)
	return err
}

// UpdateStaffNotice rewrites the row and returns the number of changed rows.
func (s *Store) UpdateStaffNotice(ctx context.Context, row *StaffNotice) (int64, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	result, err := db.NewUpdate().
		Model(row).
		ModelTableExpr(staffNoticeTable).
		WherePK().
		Where(`"staff_notice".tenant_id = ?`, tenantID).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// FindStaffNotice returns the row, or false when the tenant has no such
// notice.
func (s *Store) FindStaffNotice(ctx context.Context, id int64) (*StaffNotice, bool, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, false, err
	}
	row := new(StaffNotice)
	err = db.NewSelect().
		Model(row).
		ModelTableExpr(staffNoticeTable).
		Where(`"staff_notice".id = ?`, id).
		Where(`"staff_notice".tenant_id = ?`, tenantID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return row, true, nil
}

// DeleteStaffNotice removes the row; acknowledgements cascade.
func (s *Store) DeleteStaffNotice(ctx context.Context, id int64) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().
		Model((*StaffNotice)(nil)).
		ModelTableExpr(staffNoticeTable).
		Where(`"staff_notice".id = ?`, id).
		Where(`"staff_notice".tenant_id = ?`, tenantID).
		Exec(ctx)
	return err
}

// ListStaffNotices returns the tenant's notices, newest first.
func (s *Store) ListStaffNotices(ctx context.Context, includeInactive bool) ([]*StaffNotice, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*StaffNotice
	query := db.NewSelect().
		Model(&rows).
		ModelTableExpr(staffNoticeTable).
		OrderExpr(`"staff_notice".created_at DESC`)
	if !includeInactive {
		query = query.Where(`"staff_notice".active`)
	}
	err = query.Where(`"staff_notice".tenant_id = ?`, tenantID).Scan(ctx)
	return rows, err
}

// ListStaffNoticesValidOn narrows to the active notices whose period contains
// the day and whose audience is all or the reader. Important notices come
// first.
func (s *Store) ListStaffNoticesValidOn(ctx context.Context, date schedule.Date, reader, all, important string) ([]*StaffNotice, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*StaffNotice
	err = db.NewSelect().
		Model(&rows).
		ModelTableExpr(staffNoticeTable).
		Where(`"staff_notice".active`).
		Where(`"staff_notice".audience IN (?, ?)`, all, reader).
		Where(`"staff_notice".valid_from <= ?`, date).
		Where(`("staff_notice".valid_until IS NULL OR "staff_notice".valid_until >= ?)`, date).
		OrderExpr(`CASE WHEN "staff_notice".priority = ? THEN 0 ELSE 1 END, "staff_notice".created_at DESC`, important).
		Where(`"staff_notice".tenant_id = ?`, tenantID).
		Scan(ctx)
	return rows, err
}

// AcknowledgeStaffNotice stamps an acknowledgement; a repeated call keeps the
// first timestamp.
func (s *Store) AcknowledgeStaffNotice(ctx context.Context, row *StaffNoticeAck) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	_, err = db.NewInsert().
		Model(row).
		ModelTableExpr("users.staff_notice_acks").
		On("CONFLICT (notice_id, account_id) DO NOTHING").
		Exec(ctx)
	return err
}

// ListOwnStaffNoticeAcks returns the account's acknowledgements of the given
// notices.
func (s *Store) ListOwnStaffNoticeAcks(ctx context.Context, accountID int64, noticeIDs []int64) ([]*StaffNoticeAck, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*StaffNoticeAck
	err = db.NewSelect().
		Model(&rows).
		ModelTableExpr(staffNoticeAckTable).
		Where(`"sna".account_id = ?`, accountID).
		Where(`"sna".notice_id IN (?)`, bun.List(noticeIDs)).
		Where(`"sna".tenant_id = ?`, tenantID).
		Scan(ctx)
	return rows, err
}

// CountStaffNoticeAcks counts the acknowledgements of the given notices
// without the author's own.
func (s *Store) CountStaffNoticeAcks(ctx context.Context, noticeIDs []int64) ([]StaffNoticeCount, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []StaffNoticeCount
	err = db.NewSelect().
		Model((*StaffNoticeAck)(nil)).
		ModelTableExpr(staffNoticeAckTable).
		ColumnExpr(`"sna".notice_id AS notice_id`).
		ColumnExpr("COUNT(*) AS count").
		Join(`JOIN users.staff_notices AS "n" ON "n".id = "sna".notice_id`).
		Where(`"sna".notice_id IN (?)`, bun.List(noticeIDs)).
		Where(`"sna".account_id <> "n".created_by`).
		GroupExpr(`"sna".notice_id`).
		Where(`"sna".tenant_id = ?`, tenantID).
		Scan(ctx, &rows)
	return rows, err
}

// ListStaffNoticeAcks returns the acknowledgements of one notice without the
// author's own, newest first.
func (s *Store) ListStaffNoticeAcks(ctx context.Context, noticeID int64) ([]*StaffNoticeAck, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*StaffNoticeAck
	err = db.NewSelect().
		Model(&rows).
		ModelTableExpr(staffNoticeAckTable).
		Join(`JOIN users.staff_notices AS "n" ON "n".id = "sna".notice_id`).
		Where(`"sna".notice_id = ?`, noticeID).
		Where(`"sna".account_id <> "n".created_by`).
		OrderExpr(`"sna".acknowledged_at DESC, "sna".account_id ASC`).
		Where(`"sna".tenant_id = ?`, tenantID).
		Scan(ctx)
	return rows, err
}
