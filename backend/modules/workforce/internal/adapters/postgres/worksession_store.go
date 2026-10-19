package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

const (
	tableWorkSessions            = "active.work_sessions"
	tableWorkSessionBreaks       = "active.work_session_breaks"
	tableStaffBalanceAdjustments = "active.staff_balance_adjustments"
	tableStaffVacationOpenings   = "active.staff_vacation_openings"
	tableStaffVacationQuota      = "active.staff_vacation_quota"

	aliasWorkSession            = "work_session"
	aliasWorkSessionBreak       = "work_session_break"
	aliasStaffBalanceAdjustment = "staff_balance_adjustment"
	aliasStaffVacationOpening   = "staff_vacation_opening"
	aliasStaffVacationQuota     = "staff_vacation_quota"
)

// --- work sessions ---

func (s *Store) FindWorkSession(ctx context.Context, id int64) (domain.WorkSession, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.WorkSession{}, false, domain.OperationStats{}, err
	}
	row := &workSessionRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`).
		Where(`"work_session".id = ?`, id), aliasWorkSession, scope)
	found, stats, err := scanOne(ctx, query, "find work session")
	if err != nil || !found {
		return domain.WorkSession{}, found, stats, err
	}
	return workSessionToDomain(*row), true, stats, nil
}

func (s *Store) LockOpenWorkSession(ctx context.Context, id int64) (domain.WorkSession, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.WorkSession{}, false, domain.OperationStats{}, err
	}
	row := &workSessionRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`).
		Where(`"work_session".id = ?`, id).
		Where(`"work_session".check_out_time IS NULL`), aliasWorkSession, scope).
		For("UPDATE")
	found, stats, err := scanOne(ctx, query, "lock open work session")
	if err != nil || !found {
		return domain.WorkSession{}, found, stats, err
	}
	return workSessionToDomain(*row), true, stats, nil
}

func (s *Store) OpenWorkSessionOn(ctx context.Context, staffID int64, date string, lock bool) (domain.WorkSession, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.WorkSession{}, false, domain.OperationStats{}, err
	}
	row := &workSessionRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`).
		Where(`"work_session".staff_id = ?`, staffID).
		Where(`"work_session".date = ?`, date).
		Where(`"work_session".check_out_time IS NULL`), aliasWorkSession, scope)
	if lock {
		query = query.For("UPDATE")
	}
	found, stats, err := scanOne(ctx, query, "find open work session on day")
	if err != nil || !found {
		return domain.WorkSession{}, found, stats, err
	}
	return workSessionToDomain(*row), true, stats, nil
}

// LatestOpenWorkSession applies the live window the balance applies: a block
// filed on today or later always counts, an older one only while its check-in
// is inside MaxOpenWorkSessionDuration. Past that limit a forgotten checkout
// is not a running block (#2402).
func (s *Store) LatestOpenWorkSession(ctx context.Context, staffID int64, today string, now time.Time) (domain.WorkSession, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.WorkSession{}, false, domain.OperationStats{}, err
	}
	row := &workSessionRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`).
		Where(`"work_session".staff_id = ?`, staffID).
		Where(`"work_session".check_out_time IS NULL`).
		Where(`("work_session".date >= ? OR "work_session".check_in_time > ?)`,
			today, now.Add(-domain.MaxOpenWorkSessionDuration)), aliasWorkSession, scope).
		OrderExpr(`"work_session".date DESC, "work_session".check_in_time DESC`).
		Limit(1)
	found, stats, err := scanOne(ctx, query, "find latest open work session")
	if err != nil || !found {
		return domain.WorkSession{}, found, stats, err
	}
	return workSessionToDomain(*row), true, stats, nil
}

func (s *Store) ListWorkSessions(ctx context.Context, filter domain.WorkSessionFilter) ([]domain.WorkSession, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []workSessionRow{}
	query := applyWorkSessionFilter(withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`), aliasWorkSession, scope), filter)
	if len(filter.Order) == 0 {
		query = query.OrderExpr(`"work_session".id ASC`)
	}
	for _, order := range filter.Order {
		query = orderBy(query, aliasWorkSession, string(order.Field), order.Descending)
	}
	query = limitOffset(query, filter.Limit, filter.Offset)
	stats, err := scanAll(ctx, query, "list work sessions")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	return workSessionsToDomain(rows), stats, nil
}

// ListOverlappingWorkSessions compares timestamps, never the date column: a
// block is filed on the day of its check-in but may reach into the following
// days, so a date window would miss exactly the blocks that overlap across
// midnight. An open sibling runs to infinity and intersects anything starting
// after its check-in.
func (s *Store) ListOverlappingWorkSessions(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) ([]domain.WorkSession, domain.OperationStats, error) {
	if len(staffIDs) == 0 {
		return nil, domain.OperationStats{}, nil
	}
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []workSessionRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`).
		Where(`"work_session".staff_id IN (?)`, bun.List(staffIDs)).
		Where(`("work_session".check_out_time IS NULL OR "work_session".check_out_time > ?)`, from), aliasWorkSession, scope).
		OrderExpr(`"work_session".staff_id ASC, "work_session".check_in_time ASC`)
	if to != nil {
		query = query.Where(`"work_session".check_in_time < ?`, *to)
	}
	stats, err := scanAll(ctx, query, "list overlapping work sessions")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	return workSessionsToDomain(rows), stats, nil
}

func (s *Store) CountWorkSessions(ctx context.Context, filter domain.WorkSessionFilter) (int, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := applyWorkSessionFilter(withTenant(db.NewSelect().Model((*workSessionRow)(nil)).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`), aliasWorkSession, scope), filter)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	count, err := query.Count(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("workforce postgres: count work sessions: %w", err)
	}
	return int(count), stats, nil
}

func (s *Store) OldestWorkSessionDate(ctx context.Context, before string) (string, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return "", domain.OperationStats{}, err
	}
	query := withTenant(db.NewSelect().Model((*workSessionRow)(nil)).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`).
		ColumnExpr(`TO_CHAR(MIN("work_session".date), 'YYYY-MM-DD')`), aliasWorkSession, scope)
	if before != "" {
		query = query.Where(`"work_session".date < ?`, before)
	}
	var oldest *string
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &oldest)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return "", stats, fmt.Errorf("workforce postgres: oldest work session date: %w", err)
	}
	if oldest == nil {
		return "", stats, nil
	}
	stats.Rows = 1
	return *oldest, stats, nil
}

func (s *Store) DeleteWorkSessionsOlderThan(ctx context.Context, cutoff string) (int64, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*workSessionRow)(nil)).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`).
		Where(`"work_session".date < ?`, cutoff), aliasWorkSession, scope)
	stats, err := execAffected(ctx, query, "delete work sessions older than")
	return stats.Rows, stats, err
}

// WorkPresenceMap picks up today's blocks and still-open blocks filed earlier
// whose check-in is inside the live window, so a person working across
// Berlin midnight stays present while a forgotten checkout drops out.
func (s *Store) WorkPresenceMap(ctx context.Context, today string, now time.Time) (map[int64]string, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var results []struct {
		StaffID      int64      `bun:"staff_id"`
		Status       string     `bun:"status"`
		CheckOutTime *time.Time `bun:"check_out_time"`
	}
	query := withTenant(db.NewSelect().
		TableExpr(tableWorkSessions+` AS "work_session"`).
		ColumnExpr(`"work_session".staff_id`).
		ColumnExpr(`"work_session".status`).
		ColumnExpr(`"work_session".check_out_time`).
		Where(`("work_session".date = ? OR ("work_session".check_out_time IS NULL AND "work_session".check_in_time > ?))`,
			today, now.Add(-domain.MaxOpenWorkSessionDuration)), aliasWorkSession, scope)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &results)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("workforce postgres: work presence map: %w", err)
	}
	stats.Rows = int64(len(results))
	presence := make(map[int64]string, len(results))
	for _, result := range results {
		if result.CheckOutTime == nil {
			presence[result.StaffID] = result.Status
			continue
		}
		if _, exists := presence[result.StaffID]; !exists {
			presence[result.StaffID] = "checked_out"
		}
	}
	return presence, stats, nil
}

func (s *Store) CreateWorkSession(ctx context.Context, value domain.WorkSession) (domain.WorkSession, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.WorkSession{}, domain.OperationStats{}, err
	}
	row := workSessionFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = scope.TenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableWorkSessions).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if isUniqueViolationOn(err, domain.WorkSessionOpenConstraint) {
			return domain.WorkSession{}, stats, &domain.ConflictError{Kind: domain.ErrWorkSessionAlreadyOpen, Cause: err}
		}
		return domain.WorkSession{}, stats, fmt.Errorf("workforce postgres: insert work session: %w", err)
	}
	stats.Rows = 1
	return workSessionToDomain(*row), stats, nil
}

func (s *Store) UpdateWorkSession(ctx context.Context, value domain.WorkSession) (domain.WorkSession, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.WorkSession{}, false, domain.OperationStats{}, err
	}
	row := workSessionFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = scope.TenantID
	}
	query := withTenant(db.NewUpdate().Model(row).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`).
		ExcludeColumn("created_at", "updated_at").
		Set("updated_at = NOW()").
		WherePK().
		Returning("*"), aliasWorkSession, scope)
	stats, err := execAffected(ctx, query, "update work session")
	if err != nil {
		if isUniqueViolationOn(err, domain.WorkSessionOpenConstraint) {
			return domain.WorkSession{}, false, stats, &domain.ConflictError{Kind: domain.ErrWorkSessionAlreadyOpen, Cause: err}
		}
		return domain.WorkSession{}, false, stats, err
	}
	if stats.Rows != 1 {
		stats.Rows = 0
		return domain.WorkSession{}, false, stats, nil
	}
	return workSessionToDomain(*row), true, stats, nil
}

func (s *Store) DeleteWorkSession(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*workSessionRow)(nil)).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`).
		Where(`"work_session".id = ?`, id), aliasWorkSession, scope)
	return execAffected(ctx, query, "delete work session")
}

func (s *Store) SetWorkSessionBreakMinutes(ctx context.Context, id int64, minutes int) (int64, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*workSessionRow)(nil)).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`).
		Set("break_minutes = ?", minutes).
		Set("updated_at = NOW()").
		Where(`"work_session".id = ?`, id), aliasWorkSession, scope)
	stats, err := execAffected(ctx, query, "update work session break minutes")
	return stats.Rows, stats, err
}

func (s *Store) CloseWorkSession(ctx context.Context, id int64, checkOut time.Time, autoCheckedOut bool) (bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*workSessionRow)(nil)).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`).
		Set("check_out_time = ?", checkOut).
		Set("auto_checked_out = ?", autoCheckedOut).
		Set("updated_at = NOW()").
		Where(`"work_session".id = ?`, id).
		Where(`"work_session".check_out_time IS NULL`), aliasWorkSession, scope)
	stats, err := execAffected(ctx, query, "close work session")
	if err != nil {
		return false, stats, err
	}
	return stats.Rows == 1, stats, nil
}

func applyWorkSessionFilter(query *bun.SelectQuery, filter domain.WorkSessionFilter) *bun.SelectQuery {
	if filter.IDs != nil {
		if len(filter.IDs) == 0 {
			return query.Where("FALSE")
		}
		query = query.Where(`"work_session".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.StaffID > 0 {
		query = query.Where(`"work_session".staff_id = ?`, filter.StaffID)
	}
	if filter.StaffIDs != nil {
		if len(filter.StaffIDs) == 0 {
			return query.Where("FALSE")
		}
		query = query.Where(`"work_session".staff_id IN (?)`, bun.List(filter.StaffIDs))
	}
	if filter.Date != "" {
		query = query.Where(`"work_session".date = ?`, filter.Date)
	}
	if filter.DateFrom != "" {
		query = query.Where(`"work_session".date >= ?`, filter.DateFrom)
	}
	if filter.DateTo != "" {
		query = query.Where(`"work_session".date <= ?`, filter.DateTo)
	}
	if filter.DateBefore != "" {
		query = query.Where(`"work_session".date < ?`, filter.DateBefore)
	}
	if filter.Open != nil {
		if *filter.Open {
			query = query.Where(`"work_session".check_out_time IS NULL`)
		} else {
			query = query.Where(`"work_session".check_out_time IS NOT NULL`)
		}
	}
	return query
}

// --- staff balance adjustments ---

func (s *Store) FindStaffBalanceAdjustment(ctx context.Context, id int64) (domain.StaffBalanceAdjustment, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.StaffBalanceAdjustment{}, false, domain.OperationStats{}, err
	}
	row := &staffBalanceAdjustmentRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableStaffBalanceAdjustments+` AS "staff_balance_adjustment"`).
		Where(`"staff_balance_adjustment".id = ?`, id), aliasStaffBalanceAdjustment, scope)
	found, stats, err := scanOne(ctx, query, "find staff balance adjustment")
	if err != nil || !found {
		return domain.StaffBalanceAdjustment{}, found, stats, err
	}
	return staffBalanceAdjustmentToDomain(*row), true, stats, nil
}

func (s *Store) ListStaffBalanceAdjustments(ctx context.Context, filter domain.StaffBalanceAdjustmentFilter) ([]domain.StaffBalanceAdjustment, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffBalanceAdjustmentRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableStaffBalanceAdjustments+` AS "staff_balance_adjustment"`), aliasStaffBalanceAdjustment, scope)
	if filter.StaffID > 0 {
		query = query.Where(`"staff_balance_adjustment".staff_id = ?`, filter.StaffID)
	}
	if filter.StaffIDs != nil {
		if len(filter.StaffIDs) == 0 {
			query = query.Where("FALSE")
		} else {
			query = query.Where(`"staff_balance_adjustment".staff_id IN (?)`, bun.List(filter.StaffIDs))
		}
	}
	if len(filter.Types) > 0 {
		query = query.Where(`"staff_balance_adjustment".type IN (?)`, bun.List(filter.Types))
	}
	if filter.EffectiveFrom != "" {
		query = query.Where(`"staff_balance_adjustment".effective_date >= ?`, filter.EffectiveFrom)
	}
	if filter.EffectiveTo != "" {
		query = query.Where(`"staff_balance_adjustment".effective_date <= ?`, filter.EffectiveTo)
	}
	query = limitOffset(query.OrderExpr(`"staff_balance_adjustment".staff_id ASC, "staff_balance_adjustment".effective_date ASC, "staff_balance_adjustment".id ASC`), filter.Limit, filter.Offset)
	stats, err := scanAll(ctx, query, "list staff balance adjustments")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.StaffBalanceAdjustment, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffBalanceAdjustmentToDomain(row))
	}
	return result, stats, nil
}

func (s *Store) CreateStaffBalanceAdjustment(ctx context.Context, value domain.StaffBalanceAdjustment) (domain.StaffBalanceAdjustment, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.StaffBalanceAdjustment{}, domain.OperationStats{}, err
	}
	row := staffBalanceAdjustmentFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = scope.TenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableStaffBalanceAdjustments).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StaffBalanceAdjustment{}, stats, fmt.Errorf("workforce postgres: insert staff balance adjustment: %w", err)
	}
	stats.Rows = 1
	return staffBalanceAdjustmentToDomain(*row), stats, nil
}

func (s *Store) UpdateStaffBalanceAdjustment(ctx context.Context, value domain.StaffBalanceAdjustment) (domain.StaffBalanceAdjustment, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.StaffBalanceAdjustment{}, false, domain.OperationStats{}, err
	}
	row := staffBalanceAdjustmentFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = scope.TenantID
	}
	query := withTenant(db.NewUpdate().Model(row).
		ModelTableExpr(tableStaffBalanceAdjustments+` AS "staff_balance_adjustment"`).
		ExcludeColumn("created_at", "updated_at").
		Set("updated_at = NOW()").
		WherePK().
		Returning("*"), aliasStaffBalanceAdjustment, scope)
	stats, err := execAffected(ctx, query, "update staff balance adjustment")
	if err != nil {
		return domain.StaffBalanceAdjustment{}, false, stats, err
	}
	if stats.Rows != 1 {
		stats.Rows = 0
		return domain.StaffBalanceAdjustment{}, false, stats, nil
	}
	return staffBalanceAdjustmentToDomain(*row), true, stats, nil
}

func (s *Store) DeleteStaffBalanceAdjustment(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*staffBalanceAdjustmentRow)(nil)).
		ModelTableExpr(tableStaffBalanceAdjustments+` AS "staff_balance_adjustment"`).
		Where(`"staff_balance_adjustment".id = ?`, id), aliasStaffBalanceAdjustment, scope)
	return execAffected(ctx, query, "delete staff balance adjustment")
}

// --- staff vacation openings ---

func (s *Store) FindStaffVacationOpening(ctx context.Context, id int64) (domain.StaffVacationOpening, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.StaffVacationOpening{}, false, domain.OperationStats{}, err
	}
	row := &staffVacationOpeningRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableStaffVacationOpenings+` AS "staff_vacation_opening"`).
		Where(`"staff_vacation_opening".id = ?`, id), aliasStaffVacationOpening, scope)
	found, stats, err := scanOne(ctx, query, "find staff vacation opening")
	if err != nil || !found {
		return domain.StaffVacationOpening{}, found, stats, err
	}
	return staffVacationOpeningToDomain(*row), true, stats, nil
}

func (s *Store) ListStaffVacationOpenings(ctx context.Context, filter domain.StaffVacationFilter) ([]domain.StaffVacationOpening, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffVacationOpeningRow{}
	query := applyStaffVacationFilter(withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableStaffVacationOpenings+` AS "staff_vacation_opening"`), aliasStaffVacationOpening, scope), aliasStaffVacationOpening, filter)
	stats, err := scanAll(ctx, query, "list staff vacation openings")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.StaffVacationOpening, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffVacationOpeningToDomain(row))
	}
	return result, stats, nil
}

func (s *Store) CreateStaffVacationOpening(ctx context.Context, value domain.StaffVacationOpening) (domain.StaffVacationOpening, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.StaffVacationOpening{}, domain.OperationStats{}, err
	}
	row := staffVacationOpeningFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = scope.TenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableStaffVacationOpenings).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StaffVacationOpening{}, stats, fmt.Errorf("workforce postgres: insert staff vacation opening: %w", err)
	}
	stats.Rows = 1
	return staffVacationOpeningToDomain(*row), stats, nil
}

func (s *Store) UpdateStaffVacationOpening(ctx context.Context, value domain.StaffVacationOpening) (domain.StaffVacationOpening, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.StaffVacationOpening{}, false, domain.OperationStats{}, err
	}
	row := staffVacationOpeningFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = scope.TenantID
	}
	query := withTenant(db.NewUpdate().Model(row).
		ModelTableExpr(tableStaffVacationOpenings+` AS "staff_vacation_opening"`).
		ExcludeColumn("created_at", "updated_at").
		Set("updated_at = NOW()").
		WherePK().
		Returning("*"), aliasStaffVacationOpening, scope)
	stats, err := execAffected(ctx, query, "update staff vacation opening")
	if err != nil {
		return domain.StaffVacationOpening{}, false, stats, err
	}
	if stats.Rows != 1 {
		stats.Rows = 0
		return domain.StaffVacationOpening{}, false, stats, nil
	}
	return staffVacationOpeningToDomain(*row), true, stats, nil
}

func (s *Store) DeleteStaffVacationOpening(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*staffVacationOpeningRow)(nil)).
		ModelTableExpr(tableStaffVacationOpenings+` AS "staff_vacation_opening"`).
		Where(`"staff_vacation_opening".id = ?`, id), aliasStaffVacationOpening, scope)
	return execAffected(ctx, query, "delete staff vacation opening")
}

// --- staff vacation quota ---

func (s *Store) FindStaffVacationQuota(ctx context.Context, id int64) (domain.StaffVacationQuota, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.StaffVacationQuota{}, false, domain.OperationStats{}, err
	}
	row := &staffVacationQuotaRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableStaffVacationQuota+` AS "staff_vacation_quota"`).
		Where(`"staff_vacation_quota".id = ?`, id), aliasStaffVacationQuota, scope)
	found, stats, err := scanOne(ctx, query, "find staff vacation quota")
	if err != nil || !found {
		return domain.StaffVacationQuota{}, found, stats, err
	}
	return staffVacationQuotaToDomain(*row), true, stats, nil
}

func (s *Store) ListStaffVacationQuotas(ctx context.Context, filter domain.StaffVacationFilter) ([]domain.StaffVacationQuota, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffVacationQuotaRow{}
	query := applyStaffVacationFilter(withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableStaffVacationQuota+` AS "staff_vacation_quota"`), aliasStaffVacationQuota, scope), aliasStaffVacationQuota, filter)
	stats, err := scanAll(ctx, query, "list staff vacation quotas")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.StaffVacationQuota, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffVacationQuotaToDomain(row))
	}
	return result, stats, nil
}

func (s *Store) CreateStaffVacationQuota(ctx context.Context, value domain.StaffVacationQuota) (domain.StaffVacationQuota, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.StaffVacationQuota{}, domain.OperationStats{}, err
	}
	row := staffVacationQuotaFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = scope.TenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableStaffVacationQuota).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StaffVacationQuota{}, stats, fmt.Errorf("workforce postgres: insert staff vacation quota: %w", err)
	}
	stats.Rows = 1
	return staffVacationQuotaToDomain(*row), stats, nil
}

func (s *Store) UpdateStaffVacationQuota(ctx context.Context, value domain.StaffVacationQuota) (domain.StaffVacationQuota, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.StaffVacationQuota{}, false, domain.OperationStats{}, err
	}
	row := staffVacationQuotaFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = scope.TenantID
	}
	query := withTenant(db.NewUpdate().Model(row).
		ModelTableExpr(tableStaffVacationQuota+` AS "staff_vacation_quota"`).
		ExcludeColumn("created_at", "updated_at").
		Set("updated_at = NOW()").
		WherePK().
		Returning("*"), aliasStaffVacationQuota, scope)
	stats, err := execAffected(ctx, query, "update staff vacation quota")
	if err != nil {
		return domain.StaffVacationQuota{}, false, stats, err
	}
	if stats.Rows != 1 {
		stats.Rows = 0
		return domain.StaffVacationQuota{}, false, stats, nil
	}
	return staffVacationQuotaToDomain(*row), true, stats, nil
}

func (s *Store) DeleteStaffVacationQuota(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*staffVacationQuotaRow)(nil)).
		ModelTableExpr(tableStaffVacationQuota+` AS "staff_vacation_quota"`).
		Where(`"staff_vacation_quota".id = ?`, id), aliasStaffVacationQuota, scope)
	return execAffected(ctx, query, "delete staff vacation quota")
}

func (s *Store) UpsertStaffVacationQuota(ctx context.Context, value domain.StaffVacationQuota) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := staffVacationQuotaFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = scope.TenantID
	}
	var stats domain.OperationStats
	var previous *staffVacationQuotaRow
	if value.ChangedBy > 0 {
		var existing staffVacationQuotaRow
		started := time.Now()
		err := db.NewSelect().Model(&existing).
			ModelTableExpr(tableStaffVacationQuota+` AS "staff_vacation_quota"`).
			Where(`"staff_vacation_quota".tenant_id = ?`, row.TenantID).
			Where(`"staff_vacation_quota".staff_id = ?`, row.StaffID).
			Where(`"staff_vacation_quota".year = ?`, row.Year).
			Limit(1).Scan(ctx)
		stats.Add(domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)})
		switch {
		case err == nil:
			previous = &existing
		case !errors.Is(err, sql.ErrNoRows):
			return stats, fmt.Errorf("workforce postgres: load staff vacation quota: %w", err)
		}
	}
	query := db.NewInsert().Model(row).
		ModelTableExpr(tableStaffVacationQuota).
		On("CONFLICT (staff_id, year) DO UPDATE").
		Set("entitled_days = EXCLUDED.entitled_days").
		Set("carryover_days = EXCLUDED.carryover_days").
		Set("updated_at = CURRENT_TIMESTAMP")
	upsertStats, err := execAffected(ctx, query, "upsert staff vacation quota")
	stats.Add(upsertStats)
	if err != nil || value.ChangedBy <= 0 {
		return stats, err
	}
	change := &staffVacationQuotaChangeRow{
		TenantID: row.TenantID, StaffID: row.StaffID, Year: row.Year,
		NewEntitledDays: row.EntitledDays, NewCarryoverDays: row.CarryoverDays,
		Reason: strings.TrimSpace(value.ChangeReason), ChangedBy: value.ChangedBy,
	}
	if previous != nil {
		change.OldEntitledDays, change.OldCarryoverDays = &previous.EntitledDays, &previous.CarryoverDays
	}
	changeStats, err := execAffected(ctx, db.NewInsert().Model(change), "record staff vacation quota change")
	stats.Add(changeStats)
	return stats, err
}

// applyStaffVacationFilter narrows an opening or quota listing. The alias is
// bound through bun.Ident so the evaluator can resolve every predicate to the
// one table the query names.
func applyStaffVacationFilter(query *bun.SelectQuery, alias string, filter domain.StaffVacationFilter) *bun.SelectQuery {
	if filter.StaffID > 0 {
		query = query.Where("? = ?", bun.Ident(alias+".staff_id"), filter.StaffID)
	}
	if filter.StaffIDs != nil {
		if len(filter.StaffIDs) == 0 {
			return query.Where("FALSE")
		}
		query = query.Where("? IN (?)", bun.Ident(alias+".staff_id"), bun.List(filter.StaffIDs))
	}
	if filter.Year > 0 {
		query = query.Where("? = ?", bun.Ident(alias+".year"), filter.Year)
	}
	if len(filter.Order) == 0 {
		query = query.OrderExpr("? ASC, ? ASC, ? ASC", bun.Ident(alias+".staff_id"), bun.Ident(alias+".year"), bun.Ident(alias+".id"))
	}
	for _, order := range filter.Order {
		query = orderBy(query, alias, string(order.Field), order.Descending)
	}
	return limitOffset(query, filter.Limit, filter.Offset)
}

// --- shared helpers ---

func orderBy(query *bun.SelectQuery, alias, column string, descending bool) *bun.SelectQuery {
	identifier := bun.Ident(alias + "." + column)
	if descending {
		return query.OrderExpr("? DESC", identifier)
	}
	return query.OrderExpr("? ASC", identifier)
}

func limitOffset(query *bun.SelectQuery, limit, offset int) *bun.SelectQuery {
	if limit > 0 {
		query = query.Limit(int64(limit))
	}
	if offset > 0 {
		query = query.Offset(int64(offset))
	}
	return query
}
