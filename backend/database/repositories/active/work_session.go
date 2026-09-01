package active

import (
	"context"
	"fmt"
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
	db  *bun.DB
	now func() time.Time
}

// NewWorkSessionRepository creates a new WorkSessionRepository
func NewWorkSessionRepository(db *bun.DB, clocks ...func() time.Time) active.WorkSessionRepository {
	now := time.Now
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	repo := base.NewRepository[*active.WorkSession](db, tableActiveWorkSessions, "WorkSession")
	repo.TenantScoped = true
	return &WorkSessionRepository{
		Repository: repo,
		db:         db,
		now:        now,
	}
}

// LockStaffBalanceWrites serializes session and break mutations with balance
// adjustments and effective absence changes for the same staff member.
func (r *WorkSessionRepository) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	return lockStaffBalanceWrites(ctx, r.db, staffID)
}

// GetCurrentByStaffID returns the active (not checked out) session for a staff member today
func (r *WorkSessionRepository) GetCurrentByStaffID(ctx context.Context, staffID int64) (*active.WorkSession, error) {
	return r.getOpenByStaffAndDate(ctx, staffID, timezone.DateFromTime(r.now()), false)
}

// GetOpenByStaffAndDate returns the not-checked-out session of a staff member on
// an explicit calendar day. Callers that must not re-derive "today" mid-request
// (a kiosk stamp can straddle Berlin midnight) use this instead of
// GetCurrentByStaffID.
func (r *WorkSessionRepository) GetOpenByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*active.WorkSession, error) {
	return r.getOpenByStaffAndDate(ctx, staffID, date, false)
}

// GetOpenByStaffAndDateForUpdate is GetOpenByStaffAndDate with a row lock.
func (r *WorkSessionRepository) GetOpenByStaffAndDateForUpdate(ctx context.Context, staffID int64, date timezone.Date) (*active.WorkSession, error) {
	return r.getOpenByStaffAndDate(ctx, staffID, date, true)
}

func (r *WorkSessionRepository) getOpenByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date, forUpdate bool) (*active.WorkSession, error) {
	session := new(active.WorkSession)

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(session).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".staff_id = ?`, staffID).
		Where(`"work_session".date = ?`, date).
		Where(`"work_session".check_out_time IS NULL`)

	query = base.WithTenantFilter(ctx, query, "work_session")
	if forUpdate {
		query = query.For("UPDATE")
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get current by staff ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return session, nil
}

// GetLatestOpenByStaffID returns the most recent STILL RUNNING session of a
// staff member, regardless of the day it was opened on. Unlike
// GetCurrentByStaffID it does not filter on today, so a session that was opened
// before Berlin midnight and is still running stays visible after the rollover
// instead of looking like "no session today".
//
// "Still running" is the same live window the balance applies
// (services/active.BalanceSessionEnd): a block filed on today (or later) always
// counts, an older one only while its check-in is inside
// active.MaxOpenWorkSessionDuration. Past that limit the block has stopped
// counting as work everywhere else — the balance no longer credits it and the
// presence map no longer reports its owner as present — so it must stop
// counting as "clocked in" here too. Without the cutoff a single forgotten
// checkout would be reported as the current session forever and would reject
// every later check-in with "already checked in", leaving the staff member
// unable to stamp at all (#2402). The expired row stays open on disk and is
// resolved where it belongs: the auto-checkout job, or an admin edit.
func (r *WorkSessionRepository) GetLatestOpenByStaffID(ctx context.Context, staffID int64) (*active.WorkSession, error) {
	session := new(active.WorkSession)

	now := r.now()
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(session).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".staff_id = ?`, staffID).
		Where(`"work_session".check_out_time IS NULL`).
		Where(`("work_session".date >= ? OR "work_session".check_in_time > ?)`,
			timezone.DateFromTime(now), now.Add(-active.MaxOpenWorkSessionDuration)).
		OrderExpr(`"work_session".date DESC, "work_session".check_in_time DESC`).
		Limit(1)

	query = base.WithTenantFilter(ctx, query, "work_session")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get latest open by staff ID",
			Err: base.TranslateNotFound(err),
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

	query = base.WithTenantFilter(ctx, query, "work_session")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "lock open session by ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return session, nil
}

// ListOverlappingByStaffID returns every block of a staff member whose
// [check-in, check-out) interval intersects the given interval. A nil "to"
// means the candidate interval is open-ended, so every block reaching past
// "from" intersects it; an open sibling (check_out_time IS NULL) runs to
// infinity and intersects anything starting after its check-in.
//
// The comparison runs on the timestamps, never on the date column: a block is
// filed on the day of its check-in but may reach into the following days, so a
// date window would miss exactly the blocks that overlap across midnight.
func (r *WorkSessionRepository) ListOverlappingByStaffID(ctx context.Context, staffID int64, from time.Time, to *time.Time) ([]*active.WorkSession, error) {
	var sessions []*active.WorkSession
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&sessions).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".staff_id = ?`, staffID).
		Where(`("work_session".check_out_time IS NULL OR "work_session".check_out_time > ?)`, from).
		OrderExpr(`"work_session".check_in_time ASC`)

	if to != nil {
		query = query.Where(`"work_session".check_in_time < ?`, *to)
	}

	query = base.WithTenantFilter(ctx, query, "work_session")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list overlapping by staff ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return sessions, nil
}

// ListOverlappingByStaffIDs batches interval reads for the cross-staff
// Stundenkonto overview. Unlike date-based history, this deliberately includes
// a block that started before the requested calendar range.
func (r *WorkSessionRepository) ListOverlappingByStaffIDs(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) (map[int64][]*active.WorkSession, error) {
	result := make(map[int64][]*active.WorkSession, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}

	var sessions []*active.WorkSession
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&sessions).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".staff_id IN (?)`, bun.List(staffIDs)).
		Where(`("work_session".check_out_time IS NULL OR "work_session".check_out_time > ?)`, from).
		OrderExpr(`"work_session".staff_id ASC, "work_session".check_in_time ASC`)
	if to != nil {
		query = query.Where(`"work_session".check_in_time < ?`, *to)
	}
	query = base.WithTenantFilter(ctx, query, "work_session")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list overlapping by staff IDs", Err: base.TranslateNotFound(err)}
	}
	for _, session := range sessions {
		result[session.StaffID] = append(result[session.StaffID], session)
	}
	return result, nil
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
		OrderExpr(`"work_session".date ASC, "work_session".check_in_time ASC`)

	query = base.WithTenantFilter(ctx, query, "work_session")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get history by staff ID",
			Err: base.TranslateNotFound(err),
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

	query = base.WithTenantFilter(ctx, query, "work_session")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get open sessions",
			Err: base.TranslateNotFound(err),
		}
	}

	return sessions, nil
}

// GetTodayPresenceMap returns a map of staff IDs to their work status for today.
//
// Besides today's blocks it also picks up still-open blocks filed on an earlier
// date: a block opened before Berlin midnight keeps running after the rollover,
// and its owner is working right now even though no row carries today's date.
// Filtering on the date alone would report that person as absent and drop them
// out of on-duty notification filtering (#2402).
//
// That pickup is bounded by the same live limit the balance applies
// (active.MaxOpenWorkSessionDuration, see services/active.BalanceSessionEnd):
// an open block still counts while its check-in is inside that window, and a
// block filed on today counts regardless. Anything older is a checkout that
// never happened, not a person at work — without the cutoff a single
// forgotten checkout would keep its owner "present" for weeks, in today's
// overview as well as in on-duty notification filtering.
func (r *WorkSessionRepository) GetTodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	now := r.now()
	today := timezone.DateFromTime(now)
	liveOpenSince := now.Add(-active.MaxOpenWorkSessionDuration)

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
		Where(`("work_session".date = ? OR ("work_session".check_out_time IS NULL AND "work_session".check_in_time > ?))`,
			today, liveOpenSince)

	query = base.WithTenantFilter(ctx, query, "work_session")

	err := query.Scan(ctx, &results)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get today presence map",
			Err: base.TranslateNotFound(err),
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
	return r.ListWithOptions(ctx, options)
}

// UpdateBreakMinutes sets the break_minutes cache field on a session
func (r *WorkSessionRepository) UpdateBreakMinutes(ctx context.Context, id int64, breakMinutes int) error {
	session := &active.WorkSession{Model: modelBase.Model{ID: id, UpdatedAt: time.Now()}, BreakMinutes: breakMinutes}
	n, err := r.UpdateColumns(ctx, session, "break_minutes", "updated_at")
	if err != nil {
		return err
	}
	if n != 1 {
		return &modelBase.DatabaseError{
			Op:  "update break minutes",
			Err: fmt.Errorf("expected 1 rows affected, got %d", n),
		}
	}
	return nil
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
			Err: base.TranslateNotFound(err),
		}
	}

	closed, err := result.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "close session",
			Err: base.TranslateNotFound(err),
		}
	}

	return closed == 1, nil
}

// GetHistoryByStaffIDs is GetHistoryByStaffID for many staff members in one
// round trip, keyed by staff ID. A batched IN-lookup the generic filter API
// cannot express as a single query; the cross-staff Stundenkonto overview would
// otherwise issue one range query per person.
func (r *WorkSessionRepository) GetHistoryByStaffIDs(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]*active.WorkSession, error) {
	result := make(map[int64][]*active.WorkSession, len(staffIDs))
	if len(staffIDs) == 0 {
		// bun renders an empty IN list as invalid SQL.
		return result, nil
	}

	var sessions []*active.WorkSession
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&sessions).
		ModelTableExpr(tableExprActiveWorkSessionsAsSession).
		Where(`"work_session".staff_id IN (?)`, bun.List(staffIDs)).
		Where(`"work_session".date >= ?`, from).
		Where(`"work_session".date <= ?`, to).
		OrderExpr(`"work_session".staff_id ASC, "work_session".date ASC, "work_session".check_in_time ASC`)

	query = base.WithTenantFilter(ctx, query, "work_session")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get history by staff IDs",
			Err: base.TranslateNotFound(err),
		}
	}
	for _, session := range sessions {
		result[session.StaffID] = append(result[session.StaffID], session)
	}
	return result, nil
}
