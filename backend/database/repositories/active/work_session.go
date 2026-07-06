package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Table name constants
const (
	tableActiveWorkSessions              = "active.work_sessions"
	tableExprActiveWorkSessionsAsSession = `active.work_sessions AS "work_session"`
)

// WorkSessionRepository implements active.WorkSessionRepository interface
type WorkSessionRepository struct {
	*base.Repository[*active.WorkSession]
	db *bun.DB
}

// NewWorkSessionRepository creates a new WorkSessionRepository
func NewWorkSessionRepository(db *bun.DB) active.WorkSessionRepository {
	repo := base.NewRepository[*active.WorkSession](db, tableActiveWorkSessions, "WorkSession")
	repo.TenantScoped = true
	return &WorkSessionRepository{
		Repository: repo,
		db:         db,
	}
}

// GetByStaffAndDate returns the work session for a staff member on a given date
func (r *WorkSessionRepository) GetByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*active.WorkSession, error) {
	session := new(active.WorkSession)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(session).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".staff_id = ?`, staffID).
		Where(`"work_session".date = ?`, date)

	if where, val, ok := base.TenantWhere(ctx, "work_session"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get by staff and date",
			Err: err,
		}
	}

	return session, nil
}

// GetCurrentByStaffID returns the active (not checked out) session for a staff member today
func (r *WorkSessionRepository) GetCurrentByStaffID(ctx context.Context, staffID int64) (*active.WorkSession, error) {
	return r.getCurrentByStaffID(ctx, staffID, false)
}

// GetCurrentByStaffIDForUpdate returns the active session for a staff member today and locks it.
func (r *WorkSessionRepository) GetCurrentByStaffIDForUpdate(ctx context.Context, staffID int64) (*active.WorkSession, error) {
	return r.getCurrentByStaffID(ctx, staffID, true)
}

func (r *WorkSessionRepository) getCurrentByStaffID(ctx context.Context, staffID int64, forUpdate bool) (*active.WorkSession, error) {
	session := new(active.WorkSession)
	today := timezone.TodayDate()

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(session).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".staff_id = ?`, staffID).
		Where(`"work_session".date = ?`, today).
		Where(`"work_session".check_out_time IS NULL`)

	if where, val, ok := base.TenantWhere(ctx, "work_session"); ok {
		query = query.Where(where, val)
	}
	if forUpdate {
		query = query.For("UPDATE")
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get current by staff ID",
			Err: err,
		}
	}

	return session, nil
}

// LockOpenByIDForUpdate returns and locks an open session row by ID.
func (r *WorkSessionRepository) LockOpenByIDForUpdate(ctx context.Context, id int64) (*active.WorkSession, error) {
	session := new(active.WorkSession)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(session).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".id = ?`, id).
		Where(`"work_session".check_out_time IS NULL`).
		For("UPDATE")

	if where, val, ok := base.TenantWhere(ctx, "work_session"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "lock open session by ID",
			Err: err,
		}
	}

	return session, nil
}

// GetHistoryByStaffID returns work sessions for a staff member in a date range
func (r *WorkSessionRepository) GetHistoryByStaffID(ctx context.Context, staffID int64, from, to timezone.Date) ([]*active.WorkSession, error) {
	var sessions []*active.WorkSession
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&sessions).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".staff_id = ?`, staffID).
		Where(`"work_session".date >= ?`, from).
		Where(`"work_session".date <= ?`, to).
		OrderExpr(`"work_session".date ASC`)

	if where, val, ok := base.TenantWhere(ctx, "work_session"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get history by staff ID",
			Err: err,
		}
	}

	return sessions, nil
}

// GetOpenSessions returns all sessions without check-out before a given date
func (r *WorkSessionRepository) GetOpenSessions(ctx context.Context, beforeDate timezone.Date) ([]*active.WorkSession, error) {
	var sessions []*active.WorkSession
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&sessions).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".date < ?`, beforeDate).
		Where(`"work_session".check_out_time IS NULL`)

	if where, val, ok := base.TenantWhere(ctx, "work_session"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get open sessions",
			Err: err,
		}
	}

	return sessions, nil
}

// GetTodayPresenceMap returns a map of staff IDs to their work status for today
func (r *WorkSessionRepository) GetTodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	today := timezone.TodayDate()

	var results []struct {
		StaffID      int64      `bun:"staff_id"`
		Status       string     `bun:"status"`
		CheckOutTime *time.Time `bun:"check_out_time"`
	}

	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprActiveWorkSessionsAsSession).
		ColumnExpr(`"work_session".staff_id`).
		ColumnExpr(`"work_session".status`).
		ColumnExpr(`"work_session".check_out_time`).
		Where(`"work_session".date = ?`, today)

	if where, val, ok := base.TenantWhere(ctx, "work_session"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx, &results)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get today presence map",
			Err: err,
		}
	}

	presenceMap := make(map[int64]string, len(results))
	for _, r := range results {
		if r.CheckOutTime == nil {
			// Currently checked in
			presenceMap[r.StaffID] = r.Status
		} else {
			// Already checked out today - only set if not already present (active takes priority)
			if _, exists := presenceMap[r.StaffID]; !exists {
				presenceMap[r.StaffID] = "checked_out"
			}
		}
	}

	return presenceMap, nil
}

// List overrides base List to use QueryOptions and proper table alias
func (r *WorkSessionRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.WorkSession, error) {
	var sessions []*active.WorkSession
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&sessions).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession)

	if where, val, ok := base.TenantWhere(ctx, "work_session"); ok {
		query = query.Where(where, val)
	}

	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return sessions, nil
}

// UpdateBreakMinutes sets the break_minutes cache field on a session
func (r *WorkSessionRepository) UpdateBreakMinutes(ctx context.Context, id int64, breakMinutes int) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Table(tableActiveWorkSessions).
		Set("break_minutes = ?", breakMinutes).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update break minutes",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "update break minutes")
}

// CloseSession sets the check-out time and auto_checked_out flag.
// It returns false when the session was already closed or no visible row matched.
func (r *WorkSessionRepository) CloseSession(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) (bool, error) {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Table(tableActiveWorkSessions).
		Set("check_out_time = ?", checkOutTime).
		Set("auto_checked_out = ?", autoCheckedOut).
		Set("updated_at = ?", time.Now()).
		Where("id = ? AND check_out_time IS NULL", id)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "close session",
			Err: err,
		}
	}

	closed, err := result.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "close session",
			Err: err,
		}
	}

	return closed == 1, nil
}
