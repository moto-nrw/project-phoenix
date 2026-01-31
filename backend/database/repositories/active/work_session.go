package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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
	return &WorkSessionRepository{
		Repository: base.NewRepository[*active.WorkSession](db, tableActiveWorkSessions, "WorkSession"),
		db:         db,
	}
}

// Create overrides base Create to handle validation
func (r *WorkSessionRepository) Create(ctx context.Context, session *active.WorkSession) error {
	if session == nil {
		return fmt.Errorf("work session cannot be nil")
	}

	if err := session.Validate(); err != nil {
		return err
	}

	return r.Repository.Create(ctx, session)
}

// GetByStaffAndDate returns the work session for a staff member on a given date
func (r *WorkSessionRepository) GetByStaffAndDate(ctx context.Context, staffID int64, date time.Time) (*active.WorkSession, error) {
	session := new(active.WorkSession)
	err := r.db.NewSelect().
		Model(session).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".staff_id = ?`, staffID).
		Where(`"work_session".date = ?`, date.Format("2006-01-02")).
		Scan(ctx)

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
	session := new(active.WorkSession)
	today := time.Now().Format("2006-01-02")

	err := r.db.NewSelect().
		Model(session).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".staff_id = ?`, staffID).
		Where(`"work_session".date = ?`, today).
		Where(`"work_session".check_out_time IS NULL`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get current by staff ID",
			Err: err,
		}
	}

	return session, nil
}

// GetHistoryByStaffID returns work sessions for a staff member in a date range
func (r *WorkSessionRepository) GetHistoryByStaffID(ctx context.Context, staffID int64, from, to time.Time) ([]*active.WorkSession, error) {
	var sessions []*active.WorkSession
	err := r.db.NewSelect().
		Model(&sessions).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".staff_id = ?`, staffID).
		Where(`"work_session".date >= ?`, from.Format("2006-01-02")).
		Where(`"work_session".date <= ?`, to.Format("2006-01-02")).
		OrderExpr(`"work_session".date ASC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get history by staff ID",
			Err: err,
		}
	}

	return sessions, nil
}

// GetOpenSessions returns all sessions without check-out before a given date
func (r *WorkSessionRepository) GetOpenSessions(ctx context.Context, beforeDate time.Time) ([]*active.WorkSession, error) {
	var sessions []*active.WorkSession
	err := r.db.NewSelect().
		Model(&sessions).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".date < ?`, beforeDate.Format("2006-01-02")).
		Where(`"work_session".check_out_time IS NULL`).
		Scan(ctx)

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
	today := time.Now().Format("2006-01-02")

	var results []struct {
		StaffID      int64      `bun:"staff_id"`
		Status       string     `bun:"status"`
		CheckOutTime *time.Time `bun:"check_out_time"`
	}

	err := r.db.NewSelect().
		TableExpr(tableExprActiveWorkSessionsAsSession).
		ColumnExpr(`"work_session".staff_id`).
		ColumnExpr(`"work_session".status`).
		ColumnExpr(`"work_session".check_out_time`).
		Where(`"work_session".date = ?`, today).
		Scan(ctx, &results)

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
	query := r.db.NewSelect().
		Model(&sessions).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession)

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
	_, err := r.db.NewUpdate().
		Table(tableActiveWorkSessions).
		Set("break_minutes = ?", breakMinutes).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update break minutes",
			Err: err,
		}
	}

	return nil
}

// CloseSession sets the check-out time and auto_checked_out flag
func (r *WorkSessionRepository) CloseSession(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) error {
	_, err := r.db.NewUpdate().
		Table(tableActiveWorkSessions).
		Set("check_out_time = ?", checkOutTime).
		Set("auto_checked_out = ?", autoCheckedOut).
		Set("updated_at = ?", time.Now()).
		Where("id = ? AND check_out_time IS NULL", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "close session",
			Err: err,
		}
	}

	return nil
}
