package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

const (
	tableStaffAbsences     = "active.staff_absences"
	tableStaffAbsenceTypes = "active.staff_absence_types"
	tableStaffAbsenceAudit = "active.staff_absence_audit"

	aliasStaffAbsence     = "staff_absence"
	aliasStaffAbsenceType = "staff_absence_type"

	// staffAbsenceTypeNameUniqueIndex is the case-insensitive unique index on
	// (tenant_id, LOWER(name)); two concurrent creates race on it after both
	// passed the name pre-check.
	staffAbsenceTypeNameUniqueIndex = "uniq_staff_absence_types_tenant_name"
)

// absencePriorityOrder ranks overlapping effective absences on one day; it
// must agree with domain.AbsenceTypePriority.
const absencePriorityOrder = `CASE "staff_absence".absence_type WHEN 'sick' THEN 5 WHEN 'training' THEN 4 WHEN 'vacation' THEN 3 WHEN 'comp_time' THEN 2 WHEN 'other' THEN 1 ELSE 0 END DESC`

type staffAbsenceRow struct {
	bun.BaseModel     `bun:"table:active.staff_absences,alias:staff_absence"`
	ID                int64        `bun:"id,pk,autoincrement"`
	TenantID          int64        `bun:"tenant_id,notnull"`
	StaffID           int64        `bun:"staff_id,notnull"`
	AbsenceType       string       `bun:"absence_type,notnull"`
	AbsenceTypeID     *int64       `bun:"absence_type_id"`
	DateStart         calendarDate `bun:"date_start,notnull,type:date"`
	DateEnd           calendarDate `bun:"date_end,notnull,type:date"`
	HalfDay           bool         `bun:"half_day,notnull,default:false"`
	StartHalfDay      bool         `bun:"start_half_day,notnull,default:false"`
	EndHalfDay        bool         `bun:"end_half_day,notnull,default:false"`
	Note              string       `bun:"note"`
	Status            string       `bun:"status,notnull,default:'reported'"`
	ApprovedBy        *int64       `bun:"approved_by"`
	ApprovedAt        *time.Time   `bun:"approved_at"`
	CreatedBy         int64        `bun:"created_by,notnull"`
	WorkingDays       *float64     `bun:"working_days"`
	DecisionNote      string       `bun:"decision_note"`
	RequestedAt       time.Time    `bun:"requested_at,notnull,default:current_timestamp"`
	SubstituteStaffID *int64       `bun:"substitute_staff_id"`
	CreatedAt         time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffAbsenceTypeRow struct {
	bun.BaseModel    `bun:"table:active.staff_absence_types,alias:staff_absence_type"`
	ID               int64     `bun:"id,pk,autoincrement"`
	TenantID         int64     `bun:"tenant_id,notnull"`
	Name             string    `bun:"name,notnull"`
	BaseType         string    `bun:"base_type,notnull"`
	IsActive         bool      `bun:"is_active,notnull"`
	AllowanceEnabled bool      `bun:"allowance_enabled,notnull"`
	OverrunPolicy    string    `bun:"overrun_policy,notnull"`
	CreatedAt        time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffAbsenceAuditRow struct {
	bun.BaseModel `bun:"table:active.staff_absence_audit,alias:staff_absence_audit"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	AbsenceID     int64     `bun:"absence_id,notnull"`
	FromStatus    *string   `bun:"from_status"`
	ToStatus      string    `bun:"to_status,notnull"`
	ActorID       int64     `bun:"actor_id,notnull"`
	Note          string    `bun:"note"`
	ChangedAt     time.Time `bun:"changed_at,nullzero,notnull,default:current_timestamp"`
}

// AcquireXactLock takes the transaction-scoped advisory lock for key on the
// ambient transaction or, without one, on the database directly. Outside a
// transaction the lock releases at statement end, which is exactly what the
// legacy repositories did for callers that never opened one.
func (s *Store) AcquireXactLock(ctx context.Context, key string) error {
	db, _, err := s.database(ctx)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key); err != nil {
		return fmt.Errorf("workforce postgres: acquire advisory lock: %w", err)
	}
	return nil
}

// --- staff absences ---

func (s *Store) FindStaffAbsence(ctx context.Context, id int64) (domain.StaffAbsence, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffAbsence{}, false, domain.OperationStats{}, err
	}
	row := &staffAbsenceRow{}
	query := withTenant(db.NewSelect().
		Model(row).
		ModelTableExpr(tableStaffAbsences+` AS "staff_absence"`).
		Where(`"staff_absence".id = ?`, id), aliasStaffAbsence, tenantID)
	found, stats, err := scanOne(ctx, query, "find staff absence")
	if err != nil || !found {
		return domain.StaffAbsence{}, found, stats, err
	}
	return staffAbsenceToDomain(*row), true, stats, nil
}

func (s *Store) ListStaffAbsences(ctx context.Context, filter domain.StaffAbsenceFilter) ([]domain.StaffAbsence, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffAbsenceRow{}
	query := applyStaffAbsenceFilter(withTenant(db.NewSelect().
		Model(&rows).
		ModelTableExpr(tableStaffAbsences+` AS "staff_absence"`), aliasStaffAbsence, tenantID), filter)
	if len(filter.Order) == 0 {
		query = query.OrderExpr(`"staff_absence".id ASC`)
	}
	for _, order := range filter.Order {
		column := bun.Ident(aliasStaffAbsence + "." + string(order.Field))
		if order.Descending {
			query = query.OrderExpr("? DESC", column)
		} else {
			query = query.OrderExpr("? ASC", column)
		}
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	stats, err := scanAll(ctx, query, "list staff absences")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	return staffAbsencesToDomain(rows), stats, nil
}

func (s *Store) CountStaffAbsences(ctx context.Context, filter domain.StaffAbsenceFilter) (int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := applyStaffAbsenceFilter(withTenant(db.NewSelect().
		Model((*staffAbsenceRow)(nil)).
		ModelTableExpr(tableStaffAbsences+` AS "staff_absence"`), aliasStaffAbsence, tenantID), filter)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	count, err := query.Count(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("workforce postgres: count staff absences: %w", err)
	}
	return count, stats, nil
}

func applyStaffAbsenceFilter(query *bun.SelectQuery, filter domain.StaffAbsenceFilter) *bun.SelectQuery {
	if filter.StaffID > 0 {
		query = query.Where(`"staff_absence".staff_id = ?`, filter.StaffID)
	}
	if filter.StaffIDs != nil {
		if len(filter.StaffIDs) == 0 {
			// bun renders an empty IN list as invalid SQL; an explicit empty
			// staff set matches nobody.
			return query.Where("FALSE")
		}
		query = query.Where(`"staff_absence".staff_id IN (?)`, bun.List(filter.StaffIDs))
	}
	if len(filter.Statuses) > 0 {
		query = query.Where(`"staff_absence".status IN (?)`, bun.List(filter.Statuses))
	}
	if len(filter.Types) > 0 {
		query = query.Where(`"staff_absence".absence_type IN (?)`, bun.List(filter.Types))
	}
	if filter.OverlapTo != "" {
		query = query.Where(`"staff_absence".date_start <= ?`, filter.OverlapTo)
	}
	if filter.OverlapFrom != "" {
		query = query.Where(`"staff_absence".date_end >= ?`, filter.OverlapFrom)
	}
	if filter.DateEndBefore != "" {
		query = query.Where(`"staff_absence".date_end < ?`, filter.DateEndBefore)
	}
	if filter.DateStartBefore != "" {
		query = query.Where(`"staff_absence".date_start < ?`, filter.DateStartBefore)
	}
	if filter.NonHistoricalFrom != "" {
		query = query.Where(`("staff_absence".status IN (?) OR "staff_absence".date_end >= ?)`,
			bun.List(domain.PendingAbsenceStatuses), filter.NonHistoricalFrom)
	}
	return query
}

func (s *Store) ListStaffAbsenceRequests(ctx context.Context, filter domain.StaffAbsenceRequestFilter) ([]domain.StaffAbsence, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffAbsenceRow{}
	query := withTenant(db.NewSelect().
		Model(&rows).
		ModelTableExpr(tableStaffAbsences+` AS "staff_absence"`).
		Where(`"staff_absence".status IN (?)`, bun.List(filter.Statuses)), aliasStaffAbsence, tenantID)
	if len(filter.Types) > 0 {
		query = query.Where(`"staff_absence".absence_type IN (?)`, bun.List(filter.Types))
	}
	if filter.FilterSubjects {
		if len(filter.SubjectStaffIDs) == 0 {
			return []domain.StaffAbsence{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"staff_absence".staff_id IN (?)`, bun.List(filter.SubjectStaffIDs))
	}
	if filter.Decided {
		query = query.OrderExpr(`COALESCE("staff_absence".approved_at, "staff_absence".updated_at) DESC, "staff_absence".id DESC`)
	} else {
		query = query.OrderExpr(`"staff_absence".requested_at ASC, "staff_absence".id ASC`)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	stats, err := scanAll(ctx, query, "list staff absence requests")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	return staffAbsencesToDomain(rows), stats, nil
}

// EffectiveStaffAbsencesOn returns the effective absences covering the day in
// canonical priority order; ID resolves equal priorities deterministically.
func (s *Store) EffectiveStaffAbsencesOn(ctx context.Context, date string) ([]domain.StaffAbsence, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffAbsenceRow{}
	query := withTenant(db.NewSelect().
		Model(&rows).
		ModelTableExpr(tableStaffAbsences+` AS "staff_absence"`).
		Where(`"staff_absence".date_start <= ?`, date).
		Where(`"staff_absence".date_end >= ?`, date).
		Where(`"staff_absence".status IN (?)`, bun.List(domain.EffectiveAbsenceStatuses)).
		OrderExpr(`"staff_absence".staff_id ASC`).
		OrderExpr(absencePriorityOrder).
		OrderExpr(`"staff_absence".id ASC`), aliasStaffAbsence, tenantID)
	stats, err := scanAll(ctx, query, "list effective staff absences on date")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	return staffAbsencesToDomain(rows), stats, nil
}

func (s *Store) OldestStaffAbsenceDate(ctx context.Context, column, before string) (string, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return "", domain.OperationStats{}, err
	}
	query := withTenant(db.NewSelect().
		Model((*staffAbsenceRow)(nil)).
		ModelTableExpr(tableStaffAbsences+` AS "staff_absence"`).
		ColumnExpr("TO_CHAR(MIN(?), 'YYYY-MM-DD')", bun.Ident(aliasStaffAbsence+"."+column)), aliasStaffAbsence, tenantID)
	if before != "" {
		query = query.Where("? < ?", bun.Ident(aliasStaffAbsence+"."+column), before)
	}
	stats := domain.OperationStats{Queries: 1}
	var oldest sql.NullString
	started := time.Now()
	err = query.Scan(ctx, &oldest)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return "", stats, fmt.Errorf("workforce postgres: oldest staff absence date: %w", err)
	}
	if !oldest.Valid {
		return "", stats, nil
	}
	stats.Rows = 1
	return oldest.String, stats, nil
}

func (s *Store) CreateStaffAbsence(ctx context.Context, value domain.StaffAbsence) (domain.StaffAbsence, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffAbsence{}, domain.OperationStats{}, err
	}
	row := staffAbsenceFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableStaffAbsences).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StaffAbsence{}, stats, fmt.Errorf("workforce postgres: insert staff absence: %w", err)
	}
	stats.Rows = 1
	return staffAbsenceToDomain(*row), stats, nil
}

func (s *Store) UpdateStaffAbsence(ctx context.Context, value domain.StaffAbsence) (domain.StaffAbsence, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffAbsence{}, false, domain.OperationStats{}, err
	}
	row := staffAbsenceFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	query := db.NewUpdate().
		Model(row).
		ModelTableExpr(tableStaffAbsences+` AS "staff_absence"`).
		ExcludeColumn("created_at", "updated_at").
		Set("updated_at = NOW()").
		WherePK().
		Returning("*")
	if tenantID > 0 {
		query = query.Where(`"staff_absence".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StaffAbsence{}, false, stats, fmt.Errorf("workforce postgres: update staff absence: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.StaffAbsence{}, false, stats, fmt.Errorf("workforce postgres: update staff absence rows affected: %w", err)
	}
	if affected != 1 {
		return domain.StaffAbsence{}, false, stats, nil
	}
	stats.Rows = affected
	return staffAbsenceToDomain(*row), true, stats, nil
}

func (s *Store) DeleteStaffAbsence(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().
		Model((*staffAbsenceRow)(nil)).
		ModelTableExpr(tableStaffAbsences+` AS "staff_absence"`).
		Where(`"staff_absence".id = ?`, id), aliasStaffAbsence, tenantID)
	return execAffected(ctx, query, "delete staff absence")
}

func (s *Store) DeleteNonHistoricalStaffAbsences(ctx context.Context, staffID int64, from string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().
		Model((*staffAbsenceRow)(nil)).
		ModelTableExpr(tableStaffAbsences+` AS "staff_absence"`).
		Where(`"staff_absence".staff_id = ?`, staffID).
		Where(`("staff_absence".status IN (?) OR "staff_absence".date_end >= ?)`, bun.List(domain.PendingAbsenceStatuses), from),
		aliasStaffAbsence, tenantID)
	stats, err := execAffected(ctx, query, "delete non-historical staff absences")
	return stats.Rows, stats, err
}

func (s *Store) DeleteStaffAbsencesOlderThan(ctx context.Context, column, cutoff string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().
		Model((*staffAbsenceRow)(nil)).
		ModelTableExpr(tableStaffAbsences+` AS "staff_absence"`).
		Where("? < ?", bun.Ident(aliasStaffAbsence+"."+column), cutoff), aliasStaffAbsence, tenantID)
	stats, err := execAffected(ctx, query, "delete staff absences older than")
	return stats.Rows, stats, err
}

// --- absence types ---

func (s *Store) ListStaffAbsenceTypes(ctx context.Context) ([]domain.StaffAbsenceType, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffAbsenceTypeRow{}
	query := withTenant(db.NewSelect().
		Model(&rows).
		ModelTableExpr(tableStaffAbsenceTypes+` AS "staff_absence_type"`).
		OrderExpr(`"staff_absence_type".name ASC`), aliasStaffAbsenceType, tenantID)
	stats, err := scanAll(ctx, query, "list staff absence types")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.StaffAbsenceType, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffAbsenceTypeToDomain(row))
	}
	return result, stats, nil
}

func (s *Store) FindStaffAbsenceType(ctx context.Context, id int64, lock bool) (domain.StaffAbsenceType, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffAbsenceType{}, false, domain.OperationStats{}, err
	}
	row := &staffAbsenceTypeRow{}
	query := withTenant(db.NewSelect().
		Model(row).
		ModelTableExpr(tableStaffAbsenceTypes+` AS "staff_absence_type"`).
		Where(`"staff_absence_type".id = ?`, id), aliasStaffAbsenceType, tenantID)
	if lock {
		query = query.For("UPDATE")
	}
	found, stats, err := scanOne(ctx, query, "find staff absence type")
	if err != nil || !found {
		return domain.StaffAbsenceType{}, found, stats, err
	}
	return staffAbsenceTypeToDomain(*row), true, stats, nil
}

func (s *Store) StaffAbsenceTypeInUse(ctx context.Context, id int64) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withTenant(db.NewSelect().
		Model((*staffAbsenceRow)(nil)).
		ModelTableExpr(tableStaffAbsences+` AS "staff_absence"`).
		Where(`"staff_absence".absence_type_id = ?`, id), aliasStaffAbsence, tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	exists, err := query.Exists(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("workforce postgres: check staff absence type usage: %w", err)
	}
	return exists, stats, nil
}

func (s *Store) CreateStaffAbsenceType(ctx context.Context, fields domain.StaffAbsenceTypeFields) (domain.StaffAbsenceType, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffAbsenceType{}, domain.OperationStats{}, err
	}
	row := &staffAbsenceTypeRow{
		TenantID: tenantID, Name: fields.Name, BaseType: fields.BaseType, IsActive: fields.IsActive,
		AllowanceEnabled: fields.AllowanceEnabled, OverrunPolicy: fields.OverrunPolicy,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableStaffAbsenceTypes).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if modelBase.IsUniqueViolationOn(err, staffAbsenceTypeNameUniqueIndex) {
			return domain.StaffAbsenceType{}, stats, &domain.ConflictError{Kind: domain.ErrAbsenceTypeNameTaken, Cause: err}
		}
		return domain.StaffAbsenceType{}, stats, fmt.Errorf("workforce postgres: insert staff absence type: %w", err)
	}
	stats.Rows = 1
	return staffAbsenceTypeToDomain(*row), stats, nil
}

// UpdateStaffAbsenceType rewrites name, active flag and allowance settings.
// base_type is deliberately not in the column list: a rename can never move
// existing absences into another calculation bucket.
func (s *Store) UpdateStaffAbsenceType(ctx context.Context, value domain.StaffAbsenceType) (domain.StaffAbsenceType, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffAbsenceType{}, false, domain.OperationStats{}, err
	}
	row := &staffAbsenceTypeRow{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name, BaseType: value.BaseType, IsActive: value.IsActive,
		AllowanceEnabled: value.AllowanceEnabled, OverrunPolicy: value.OverrunPolicy,
	}
	query := db.NewUpdate().
		Model(row).
		ModelTableExpr(tableStaffAbsenceTypes+` AS "staff_absence_type"`).
		Column("name", "is_active", "allowance_enabled", "overrun_policy").
		Set("updated_at = NOW()").
		Where(`"staff_absence_type".id = ?`, value.ID).
		Returning("*")
	if tenantID > 0 {
		query = query.Where(`"staff_absence_type".tenant_id = ?`, tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if modelBase.IsUniqueViolationOn(err, staffAbsenceTypeNameUniqueIndex) {
			return domain.StaffAbsenceType{}, false, stats, &domain.ConflictError{Kind: domain.ErrAbsenceTypeNameTaken, Cause: err}
		}
		return domain.StaffAbsenceType{}, false, stats, fmt.Errorf("workforce postgres: update staff absence type: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.StaffAbsenceType{}, false, stats, fmt.Errorf("workforce postgres: update staff absence type rows affected: %w", err)
	}
	if affected != 1 {
		return domain.StaffAbsenceType{}, false, stats, nil
	}
	stats.Rows = affected
	return staffAbsenceTypeToDomain(*row), true, stats, nil
}

// --- audit ---

func (s *Store) RecordStaffAbsenceAudit(ctx context.Context, value domain.StaffAbsenceAudit) (domain.StaffAbsenceAudit, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffAbsenceAudit{}, domain.OperationStats{}, err
	}
	row := &staffAbsenceAuditRow{
		TenantID: value.TenantID, AbsenceID: value.AbsenceID, FromStatus: value.FromStatus, ToStatus: value.ToStatus,
		ActorID: value.ActorID, Note: value.Note, ChangedAt: value.ChangedAt,
	}
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableStaffAbsenceAudit).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StaffAbsenceAudit{}, stats, fmt.Errorf("workforce postgres: insert staff absence audit: %w", err)
	}
	stats.Rows = 1
	return domain.StaffAbsenceAudit{
		ID: row.ID, TenantID: row.TenantID, AbsenceID: row.AbsenceID, FromStatus: row.FromStatus, ToStatus: row.ToStatus,
		ActorID: row.ActorID, Note: row.Note, ChangedAt: row.ChangedAt,
	}, stats, nil
}

// --- mapping ---

func staffAbsenceFromDomain(value domain.StaffAbsence) *staffAbsenceRow {
	return &staffAbsenceRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, AbsenceType: value.AbsenceType,
		AbsenceTypeID: value.AbsenceTypeID, DateStart: calendarDate(value.DateStart), DateEnd: calendarDate(value.DateEnd),
		HalfDay: value.HalfDay, StartHalfDay: value.StartHalfDay, EndHalfDay: value.EndHalfDay, Note: value.Note,
		Status: value.Status, ApprovedBy: value.ApprovedBy, ApprovedAt: value.ApprovedAt, CreatedBy: value.CreatedBy,
		WorkingDays: value.WorkingDays, DecisionNote: value.DecisionNote, RequestedAt: value.RequestedAt,
		SubstituteStaffID: value.SubstituteStaffID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func staffAbsenceToDomain(row staffAbsenceRow) domain.StaffAbsence {
	return domain.StaffAbsence{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, AbsenceType: row.AbsenceType,
		AbsenceTypeID: row.AbsenceTypeID, DateStart: string(row.DateStart), DateEnd: string(row.DateEnd),
		HalfDay: row.HalfDay, StartHalfDay: row.StartHalfDay, EndHalfDay: row.EndHalfDay, Note: row.Note,
		Status: row.Status, ApprovedBy: row.ApprovedBy, ApprovedAt: row.ApprovedAt, CreatedBy: row.CreatedBy,
		WorkingDays: row.WorkingDays, DecisionNote: row.DecisionNote, RequestedAt: row.RequestedAt,
		SubstituteStaffID: row.SubstituteStaffID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func staffAbsencesToDomain(rows []staffAbsenceRow) []domain.StaffAbsence {
	result := make([]domain.StaffAbsence, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffAbsenceToDomain(row))
	}
	return result
}

func staffAbsenceTypeToDomain(row staffAbsenceTypeRow) domain.StaffAbsenceType {
	return domain.StaffAbsenceType{
		ID: row.ID, TenantID: row.TenantID, Name: row.Name, BaseType: row.BaseType, IsActive: row.IsActive,
		AllowanceEnabled: row.AllowanceEnabled, OverrunPolicy: row.OverrunPolicy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

// execAffected runs a write and reports the affected rows as Stats.Rows.
func execAffected(ctx context.Context, query interface {
	Exec(context.Context, ...any) (sql.Result, error)
}, operation string) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("workforce postgres: %s: %w", operation, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("workforce postgres: %s rows affected: %w", operation, err)
	}
	stats.Rows = affected
	return stats, nil
}
