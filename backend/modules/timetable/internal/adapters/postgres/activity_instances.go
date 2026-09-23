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

// activityInstanceRow maps the planning columns of schedule.activity_instances.
// The execution columns still present on the table are a rollback-only mirror
// of active.activity_sessions (#2762) and are neither read nor written here;
// the planning status is read through activityInstancePlanningStatus so a
// mirrored execution state reads as planned.
type activityInstanceRow struct {
	bun.BaseModel          `bun:"table:activity_instances,alias:activity_instance"`
	ID                     int64     `bun:"id,pk,autoincrement"`
	TenantID               int64     `bun:"tenant_id,notnull"`
	CreatedAt              time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt              time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Date                   string    `bun:"date,notnull"`
	ActivityGroupID        *int64    `bun:"activity_group_id"`
	CalendarPeriodID       *int64    `bun:"calendar_period_id"`
	Title                  string    `bun:"title,notnull"`
	Description            *string   `bun:"description"`
	StartTime              string    `bun:"start_time,notnull"`
	EndTime                string    `bun:"end_time,notnull"`
	RoomID                 int64     `bun:"room_id,notnull"`
	RequiredStaff          *int      `bun:"required_staff"`
	Status                 string    `bun:"status,notnull"`
	ListKind               *string   `bun:"list_kind"`
	IsSpontaneous          bool      `bun:"is_spontaneous,notnull"`
	UnderstaffedAck        bool      `bun:"understaffed_ack,notnull"`
	UnderstaffedNote       *string   `bun:"understaffed_note"`
	CancelReason           *string   `bun:"cancel_reason"`
	Notes                  *string   `bun:"notes"`
	IdempotencyKey         *string   `bun:"idempotency_key"`
	IdempotencyFingerprint *string   `bun:"idempotency_fingerprint"`
	CreatedBy              *int64    `bun:"created_by"`
}

// activityInstancePlanningStatus projects the stored status onto the planning
// states: the mirrored execution states are planned occurrences.
const activityInstancePlanningStatus = `CASE WHEN "activity_instance".status = 'cancelled' THEN 'cancelled' ELSE 'planned' END`

// activityInstanceNotCancelled is the planning predicate for an occurrence
// that may still take place.
const activityInstanceNotCancelled = `"activity_instance".status <> 'cancelled'`

func (s *Store) FindActivityInstance(ctx context.Context, id int64) (domain.ActivityInstance, bool, domain.OperationStats, error) {
	return s.findActivityInstance(ctx, id, "")
}

// Assignment writers share the instance lock; date/status writers take it
// exclusively after their staff locks and then recheck the assignment set.
func (s *Store) LockActivityInstance(ctx context.Context, id int64, exclusive bool) (domain.ActivityInstance, bool, domain.OperationStats, error) {
	mode := "KEY SHARE"
	if exclusive {
		mode = "UPDATE"
	}
	return s.findActivityInstance(ctx, id, mode)
}

func (s *Store) findActivityInstance(ctx context.Context, id int64, lock string) (domain.ActivityInstance, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ActivityInstance{}, false, domain.OperationStats{}, err
	}
	row := activityInstanceRow{}
	query := activityInstanceSelect(db, &row, tenantID).Where(`"activity_instance".id = ?`, id)
	if lock != "" {
		query = query.For(lock)
	}
	found, stats, err := scanOne(ctx, query, "find activity instance")
	return activityInstanceToDomain(row), found, stats, err
}

func (s *Store) ListActivityInstances(ctx context.Context, filter domain.ActivityInstanceFilter) ([]domain.ActivityInstance, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []activityInstanceRow{}
	query := filterActivityInstances(activityInstanceSelect(db, &rows, tenantID), filter)
	stats, err := scanAll(ctx, query, "list activity instances")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.ActivityInstance, 0, len(rows))
	for _, row := range rows {
		result = append(result, activityInstanceToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func filterActivityInstances(query *bun.SelectQuery, filter domain.ActivityInstanceFilter) *bun.SelectQuery {
	query = filterActivityInstanceIDs(query, filter)
	query = filterActivityInstanceDates(query, filter)
	switch filter.Status {
	case domain.InstanceStatusCancelled:
		query = query.Where(`"activity_instance".status = 'cancelled'`)
	case domain.InstanceStatusPlanned:
		query = query.Where(activityInstanceNotCancelled)
	}
	if filter.IsSpontaneous != nil {
		query = query.Where(`"activity_instance".is_spontaneous = ?`, *filter.IsSpontaneous)
	}
	if filter.IdempotencyKey != "" {
		query = query.Where(`"activity_instance".idempotency_key = ?`, filter.IdempotencyKey)
	}
	if filter.MaterializedPlanned {
		query = query.Where(activityInstanceNotCancelled).
			Where(`"activity_instance".activity_group_id IS NOT NULL`).
			Where(`"activity_instance".calendar_period_id IS NOT NULL`).
			Where(`"activity_instance".is_spontaneous = FALSE`)
	}
	if filter.OrderByDateAndTime {
		query = query.OrderExpr(`"activity_instance".date ASC, "activity_instance".start_time ASC`)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit).Offset(filter.Offset)
	}
	return query
}

func filterActivityInstanceIDs(query *bun.SelectQuery, filter domain.ActivityInstanceFilter) *bun.SelectQuery {
	if len(filter.IDs) > 0 {
		query = query.Where(`"activity_instance".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.ActivityGroupID != nil {
		query = query.Where(`"activity_instance".activity_group_id = ?`, *filter.ActivityGroupID)
	}
	if len(filter.ActivityGroupIDs) > 0 {
		query = query.Where(`"activity_instance".activity_group_id IN (?)`, bun.List(filter.ActivityGroupIDs))
	}
	return query
}

func filterActivityInstanceDates(query *bun.SelectQuery, filter domain.ActivityInstanceFilter) *bun.SelectQuery {
	if filter.Date != nil {
		query = query.Where(`"activity_instance".date = ?::date`, *filter.Date)
	}
	if len(filter.Dates) > 0 {
		query = query.Where(`"activity_instance".date IN (?)`, bun.List(filter.Dates))
	}
	if filter.FromDate != nil {
		query = query.Where(`"activity_instance".date >= ?::date`, *filter.FromDate)
	}
	if filter.ToDate != nil {
		query = query.Where(`"activity_instance".date <= ?::date`, *filter.ToDate)
	}
	return query
}

func (s *Store) MaxActivityInstanceID(ctx context.Context) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewSelect().TableExpr(`schedule.activity_instances AS "activity_instance"`).
		ColumnExpr(`COALESCE(MAX("activity_instance".id), 0)`).
		Where(`"activity_instance".tenant_id = ?`, tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	var maxID int64
	err = query.Scan(ctx, &maxID)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("timetable postgres: max activity instance id: %w", err)
	}
	return maxID, stats, nil
}

func (s *Store) CountActivityInstances(ctx context.Context, before *string) (int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewSelect().TableExpr(`schedule.activity_instances AS "activity_instance"`).
		ColumnExpr("COUNT(*)").Where(`"activity_instance".tenant_id = ?`, tenantID)
	if before != nil {
		query = query.Where(`"activity_instance".date < ?::date`, *before)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	var count int
	err = query.Scan(ctx, &count)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("timetable postgres: count activity instances: %w", err)
	}
	stats.Rows = int64(count)
	return count, stats, nil
}

func (s *Store) OldestActivityInstanceBefore(ctx context.Context, before *string) (*string, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	query := db.NewSelect().TableExpr(`schedule.activity_instances AS "activity_instance"`).
		ColumnExpr(`MIN("activity_instance".date)::text`).
		Where(`"activity_instance".tenant_id = ?`, tenantID)
	if before != nil {
		query = query.Where(`"activity_instance".date < ?::date`, *before)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	var oldest *string
	err = query.Scan(ctx, &oldest)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("timetable postgres: find oldest activity instance: %w", err)
	}
	if oldest != nil {
		stats.Rows = 1
	}
	return oldest, stats, nil
}

func (s *Store) CreateActivityInstance(ctx context.Context, fields domain.ActivityInstanceFields) (domain.ActivityInstance, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ActivityInstance{}, domain.OperationStats{}, err
	}
	row := newActivityInstanceRow(tenantID, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`schedule.activity_instances`).
		Returning(activityInstanceColumns).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.ActivityInstance{}, stats, classifyWriteError("create activity instance", err, &stats)
	}
	stats.Rows = 1
	return activityInstanceToDomain(row), stats, nil
}

func (s *Store) CreateTemplateBackedActivityInstanceIfAbsent(ctx context.Context, fields domain.ActivityInstanceFields) (domain.ActivityInstance, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ActivityInstance{}, false, domain.OperationStats{}, err
	}
	row := newActivityInstanceRow(tenantID, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`schedule.activity_instances`).
		On("CONFLICT (tenant_id, date, activity_group_id, start_time) WHERE activity_group_id IS NOT NULL AND is_spontaneous = FALSE DO NOTHING").
		Returning(activityInstanceColumns).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	return activityInstanceInsertResult(row, stats, err, "create template-backed activity instance")
}

func (s *Store) CreateIdempotentActivityInstance(ctx context.Context, fields domain.ActivityInstanceFields) (domain.ActivityInstance, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ActivityInstance{}, false, domain.OperationStats{}, err
	}
	row := newActivityInstanceRow(tenantID, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`schedule.activity_instances`).
		On("CONFLICT (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING").
		Returning(activityInstanceColumns).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	return activityInstanceInsertResult(row, stats, err, "create idempotent activity instance")
}

func activityInstanceInsertResult(row activityInstanceRow, stats domain.OperationStats, err error, operation string) (domain.ActivityInstance, bool, domain.OperationStats, error) {
	if errors.Is(err, sql.ErrNoRows) {
		stats.DuplicatePreventionConflicts = 1
		return domain.ActivityInstance{}, false, stats, nil
	}
	if err != nil {
		return domain.ActivityInstance{}, false, stats, classifyWriteError(operation, err, &stats)
	}
	stats.Rows = 1
	return activityInstanceToDomain(row), true, stats, nil
}

// UpdateActivityInstance replaces every planning field except the status,
// which only PatchActivityInstance changes: writing the status back would
// overwrite the rollback mirror of a running block.
func (s *Store) UpdateActivityInstance(ctx context.Context, id int64, fields domain.ActivityInstanceFields) (domain.ActivityInstance, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ActivityInstance{}, false, domain.OperationStats{}, err
	}
	row := newActivityInstanceRow(tenantID, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	query := db.NewUpdate().Model(&row).ModelTableExpr(`schedule.activity_instances`).
		Where("id = ?", id).Where("tenant_id = ?", tenantID).Returning(activityInstanceColumns)
	for _, column := range activityInstanceWritableColumns {
		if column == "status" {
			continue
		}
		query = setActivityInstanceField(query, fields, column)
	}
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ActivityInstance{}, false, stats, nil
	}
	if err != nil {
		return domain.ActivityInstance{}, false, stats, classifyWriteError("update activity instance", err, &stats)
	}
	stats.Rows = 1
	return activityInstanceToDomain(row), true, stats, nil
}

func (s *Store) PatchActivityInstance(ctx context.Context, id int64, fields domain.ActivityInstanceFields, columns []string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewUpdate().Table("schedule.activity_instances").
		Where("id = ?", id).Where("tenant_id = ?", tenantID)
	for _, column := range columns {
		query = setActivityInstanceField(query, fields, column)
	}
	stats, err := execMeasuredWrite(ctx, query, "patch activity instance")
	return stats.Rows, stats, err
}

func (s *Store) DeleteActivityInstance(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().Table("schedule.activity_instances").
		Where("tenant_id = ?", tenantID).Where("id = ?", id), "delete activity instance")
}

// ListReplannableActivityInstanceIDs returns the template-backed, period-bound
// occurrences of the window a replan may replace, before the caller removes
// the ones that started.
func (s *Store) ListReplannableActivityInstanceIDs(ctx context.Context, from string, to *string, groupID *int64, preserveDeviations bool) ([]int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	query := db.NewSelect().TableExpr(`schedule.activity_instances AS "activity_instance"`).ColumnExpr(`"activity_instance".id`).
		Where(`"activity_instance".tenant_id = ?`, tenantID).
		Where(`"activity_instance".date >= ?::date`, from).
		Where(activityInstanceNotCancelled).
		Where(`"activity_instance".is_spontaneous = FALSE`).
		Where(`"activity_instance".activity_group_id IS NOT NULL`).
		OrderExpr(`"activity_instance".id ASC`)
	if preserveDeviations {
		query = query.Where(`"activity_instance".understaffed_ack = FALSE`).Where(activityInstanceNoDeviations)
	}
	if to != nil {
		query = query.Where(`"activity_instance".date <= ?::date`, *to)
	}
	if groupID != nil {
		query = query.Where(`"activity_instance".activity_group_id = ?`, *groupID)
	}
	var ids []int64
	stats, err := scanAllInto(ctx, query, &ids, "list replannable activity instances")
	stats.Rows = int64(len(ids))
	return ids, stats, err
}

const activityInstanceNoDeviations = `NOT EXISTS (
		SELECT 1 FROM schedule.instance_staff AS "deviation"
		WHERE "deviation".instance_id = "activity_instance".id
		  AND "deviation".tenant_id = "activity_instance".tenant_id
		  AND ("deviation".is_absent OR "deviation".is_substitute OR "deviation".absence_reason IS NOT NULL)
	)`

// DeleteActivityInstancesByID removes the given occurrences; the caller
// already excluded the ones with an execution.
func (s *Store) DeleteActivityInstancesByID(ctx context.Context, ids []int64) (int64, domain.OperationStats, error) {
	if len(ids) == 0 {
		return 0, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats, err := execMeasuredWrite(ctx, db.NewDelete().Table("schedule.activity_instances").
		Where("tenant_id = ?", tenantID).Where("id IN (?)", bun.List(ids)), "delete activity instances by id")
	return stats.Rows, stats, err
}

// ListRemovedWeekendActivityInstanceIDs returns the future, period-bound
// occurrences of the template on the given weekdays.
func (s *Store) ListRemovedWeekendActivityInstanceIDs(ctx context.Context, groupID int64, weekdays []int, today string) ([]int64, domain.OperationStats, error) {
	if len(weekdays) == 0 {
		return nil, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	query := db.NewSelect().TableExpr(`schedule.activity_instances AS "activity_instance"`).ColumnExpr(`"activity_instance".id`).
		Where(`"activity_instance".tenant_id = ?`, tenantID).Where(`"activity_instance".activity_group_id = ?`, groupID).
		Where(`"activity_instance".calendar_period_id IS NOT NULL`).Where(`"activity_instance".date > ?::date`, today).
		Where(activityInstanceNotCancelled).Where(`"activity_instance".is_spontaneous = FALSE`).
		Where(`date_part('isodow', "activity_instance".date)::int IN (?)`, bun.List(weekdays)).
		OrderExpr(`"activity_instance".id ASC`)
	var ids []int64
	stats, err := scanAllInto(ctx, query, &ids, "list removed weekend activity instances")
	stats.Rows = int64(len(ids))
	return ids, stats, err
}

// ListFutureTemplateActivityInstanceIDs returns the template's not cancelled,
// template-backed occurrences after the date whose list kind still matches
// the previous one.
func (s *Store) ListFutureTemplateActivityInstanceIDs(ctx context.Context, groupID int64, previousKind *string, after string) ([]int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	query := db.NewSelect().TableExpr(`schedule.activity_instances AS "activity_instance"`).ColumnExpr(`"activity_instance".id`).
		Where(`"activity_instance".tenant_id = ?`, tenantID).Where(`"activity_instance".activity_group_id = ?`, groupID).
		Where(`"activity_instance".date > ?::date`, after).Where(activityInstanceNotCancelled).
		Where(`"activity_instance".is_spontaneous = FALSE`).
		Where(`COALESCE("activity_instance".list_kind, '') = COALESCE(?, '')`, previousKind).
		OrderExpr(`"activity_instance".id ASC`)
	var ids []int64
	stats, err := scanAllInto(ctx, query, &ids, "list future template activity instances")
	stats.Rows = int64(len(ids))
	return ids, stats, err
}

func (s *Store) SetActivityInstanceListKind(ctx context.Context, ids []int64, newKind *string, updatedAt time.Time) (int64, domain.OperationStats, error) {
	if len(ids) == 0 {
		return 0, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewUpdate().Table("schedule.activity_instances").
		Set("list_kind = ?", newKind).Set("updated_at = ?", updatedAt).
		Where("tenant_id = ?", tenantID).Where("id IN (?)", bun.List(ids))
	stats, err := execMeasuredWrite(ctx, query, "set activity instance list kind")
	return stats.Rows, stats, err
}

func (s *Store) DeleteActivityInstancesBefore(ctx context.Context, before string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats, err := execMeasuredWrite(ctx, db.NewDelete().Table("schedule.activity_instances").
		Where("tenant_id = ?", tenantID).Where("date < ?::date", before), "delete activity instances before")
	return stats.Rows, stats, err
}

const activityInstanceColumns = `id, tenant_id, created_at, updated_at, date::text AS date,
	activity_group_id, calendar_period_id, title, description, start_time::text AS start_time,
	end_time::text AS end_time, room_id, required_staff,
	CASE WHEN status = 'cancelled' THEN 'cancelled' ELSE 'planned' END AS status, list_kind,
	is_spontaneous, understaffed_ack, understaffed_note, cancel_reason, notes, idempotency_key,
	idempotency_fingerprint, created_by`

func activityInstanceSelect(db bun.IDB, model any, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		ColumnExpr(`"activity_instance".id, "activity_instance".tenant_id, "activity_instance".created_at, "activity_instance".updated_at`).
		ColumnExpr(`"activity_instance".date::text AS date, "activity_instance".activity_group_id, "activity_instance".calendar_period_id`).
		ColumnExpr(`"activity_instance".title, "activity_instance".description, "activity_instance".start_time::text AS start_time, "activity_instance".end_time::text AS end_time`).
		ColumnExpr(`"activity_instance".room_id, "activity_instance".required_staff, `+activityInstancePlanningStatus+` AS status, "activity_instance".list_kind`).
		ColumnExpr(`"activity_instance".is_spontaneous, "activity_instance".understaffed_ack, "activity_instance".understaffed_note, "activity_instance".cancel_reason, "activity_instance".notes`).
		ColumnExpr(`"activity_instance".idempotency_key, "activity_instance".idempotency_fingerprint, "activity_instance".created_by`).
		Where(`"activity_instance".tenant_id = ?`, tenantID)
}

func newActivityInstanceRow(tenantID int64, fields domain.ActivityInstanceFields) activityInstanceRow {
	status := fields.Status
	if status == "" {
		status = domain.InstanceStatusPlanned
	}
	return activityInstanceRow{
		TenantID: tenantID, Date: fields.Date, ActivityGroupID: fields.ActivityGroupID,
		CalendarPeriodID: fields.CalendarPeriodID, Title: fields.Title, Description: fields.Description,
		StartTime: fields.StartTime, EndTime: fields.EndTime, RoomID: fields.RoomID,
		RequiredStaff: fields.RequiredStaff, Status: status,
		ListKind: fields.ListKind, IsSpontaneous: fields.IsSpontaneous,
		UnderstaffedAck: fields.UnderstaffedAck, UnderstaffedNote: fields.UnderstaffedNote,
		CancelReason: fields.CancelReason, Notes: fields.Notes, IdempotencyKey: fields.IdempotencyKey,
		IdempotencyFingerprint: fields.IdempotencyFingerprint, CreatedBy: fields.CreatedBy,
	}
}

func activityInstanceToDomain(row activityInstanceRow) domain.ActivityInstance {
	return domain.ActivityInstance{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Date: row.Date, ActivityGroupID: row.ActivityGroupID, CalendarPeriodID: row.CalendarPeriodID,
		Title: row.Title, Description: row.Description, StartTime: row.StartTime, EndTime: row.EndTime,
		RoomID: row.RoomID, RequiredStaff: row.RequiredStaff, Status: row.Status,
		ListKind: row.ListKind, IsSpontaneous: row.IsSpontaneous, UnderstaffedAck: row.UnderstaffedAck,
		UnderstaffedNote: row.UnderstaffedNote, CancelReason: row.CancelReason, Notes: row.Notes,
		IdempotencyKey: row.IdempotencyKey, IdempotencyFingerprint: row.IdempotencyFingerprint,
		CreatedBy: row.CreatedBy,
	}
}

var activityInstanceWritableColumns = []string{
	"date", "activity_group_id", "calendar_period_id", "title", "description", "start_time", "end_time",
	"room_id", "required_staff", "status", "list_kind", "is_spontaneous",
	"understaffed_ack", "understaffed_note", "cancel_reason", "notes", "idempotency_key",
	"idempotency_fingerprint", "created_by",
}

func setActivityInstanceField(query *bun.UpdateQuery, fields domain.ActivityInstanceFields, column string) *bun.UpdateQuery {
	switch column {
	case "date":
		return query.Set("date = ?::date", fields.Date)
	case "activity_group_id":
		return query.Set("activity_group_id = ?", fields.ActivityGroupID)
	case "calendar_period_id":
		return query.Set("calendar_period_id = ?", fields.CalendarPeriodID)
	case "title":
		return query.Set("title = ?", fields.Title)
	case "description":
		return query.Set("description = ?", fields.Description)
	case "start_time":
		return query.Set("start_time = ?", fields.StartTime)
	case "end_time":
		return query.Set("end_time = ?", fields.EndTime)
	case "room_id":
		return query.Set("room_id = ?", fields.RoomID)
	case "required_staff":
		return query.Set("required_staff = ?", fields.RequiredStaff)
	case "status":
		return query.Set("status = ?", fields.Status)
	case "list_kind":
		return query.Set("list_kind = ?", fields.ListKind)
	case "is_spontaneous":
		return query.Set("is_spontaneous = ?", fields.IsSpontaneous)
	default:
		return setActivityInstanceAnnotation(query, fields, column)
	}
}

func setActivityInstanceAnnotation(query *bun.UpdateQuery, fields domain.ActivityInstanceFields, column string) *bun.UpdateQuery {
	switch column {
	case "understaffed_ack":
		return query.Set("understaffed_ack = ?", fields.UnderstaffedAck)
	case "understaffed_note":
		return query.Set("understaffed_note = ?", fields.UnderstaffedNote)
	case "cancel_reason":
		return query.Set("cancel_reason = ?", fields.CancelReason)
	case "notes":
		return query.Set("notes = ?", fields.Notes)
	case "idempotency_key":
		return query.Set("idempotency_key = ?", fields.IdempotencyKey)
	case "idempotency_fingerprint":
		return query.Set("idempotency_fingerprint = ?", fields.IdempotencyFingerprint)
	case "created_by":
		return query.Set("created_by = ?", fields.CreatedBy)
	default:
		panic("unvalidated activity instance column: " + column)
	}
}
