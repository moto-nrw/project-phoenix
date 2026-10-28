package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

func (s *Store) FindVisit(ctx context.Context, id int64) (*studentpresence.Visit, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	row := new(visitRow)
	started := time.Now()
	err = db.NewSelect().Model(row).ModelTableExpr(`active.visits AS "visit"`).
		Where("visit.tenant_id = ?", tenantID).Where("visit.id = ?", id).Scan(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("find visit: %w", err)
	}
	stats.Rows = 1
	return row.record(), stats, nil
}

func visitQuery(db bun.IDB, tenantID int64, rows any, filter studentpresence.VisitFilter) *bun.SelectQuery {
	query := db.NewSelect().Model(rows).ModelTableExpr(`active.visits AS "visit"`).Where("visit.tenant_id = ?", tenantID)
	if filter.IDs != nil {
		query = query.Where("visit.id IN (?)", bun.List(filter.IDs))
	}
	if filter.StudentIDs != nil {
		query = query.Where("visit.student_id IN (?)", bun.List(filter.StudentIDs))
	}
	if filter.ActiveGroupIDs != nil {
		query = query.Where("visit.active_group_id IN (?)", bun.List(filter.ActiveGroupIDs))
	}
	query = visitTimeQuery(query, filter)
	if filter.OpenOnly {
		query = query.Where("visit.exit_time IS NULL")
	}
	if filter.ClosedOnly {
		query = query.Where("visit.exit_time IS NOT NULL")
	}
	if filter.StudentOrder {
		query = query.OrderExpr("visit.student_id ASC")
	}
	if filter.NewestFirst {
		query = query.OrderExpr("visit.entry_time DESC")
	} else {
		query = query.OrderExpr("visit.entry_time ASC")
	}
	if filter.Limit > 0 {
		query = query.Limit(int64(filter.Limit))
	}
	if filter.Offset > 0 {
		query = query.Offset(int64(filter.Offset))
	}
	if filter.ForUpdate {
		query = query.For("UPDATE")
	}
	return query
}

func visitTimeQuery(query *bun.SelectQuery, filter studentpresence.VisitFilter) *bun.SelectQuery {
	if filter.EnteredFrom != nil {
		query = query.Where("visit.entry_time >= ?", *filter.EnteredFrom)
	}
	if filter.EnteredUntil != nil {
		query = query.Where("visit.entry_time <= ?", *filter.EnteredUntil)
	}
	if filter.OverlapFrom != nil {
		query = query.Where("(visit.exit_time IS NULL OR visit.exit_time >= ?)", *filter.OverlapFrom)
	}
	if filter.OverlapUntil != nil {
		query = query.Where("visit.entry_time <= ?", *filter.OverlapUntil)
	}
	if filter.ClosedSince != nil {
		query = query.Where("(visit.exit_time IS NULL OR visit.exit_time >= ?)", *filter.ClosedSince)
	}
	return query
}

func (s *Store) ListVisits(ctx context.Context, filter studentpresence.VisitFilter) ([]*studentpresence.Visit, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []*visitRow{}
	if (filter.IDs != nil && len(filter.IDs) == 0) || (filter.StudentIDs != nil && len(filter.StudentIDs) == 0) || (filter.ActiveGroupIDs != nil && len(filter.ActiveGroupIDs) == 0) {
		return []*studentpresence.Visit{}, ports.Stats{}, nil
	}
	started := time.Now()
	err = visitQuery(db, tenantID, &rows, filter).Scan(ctx)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("list visits: %w", err)
	}
	return visitRecordsFromRows(rows), stats, nil
}

// openVisitPerStudentIndex is the partial unique index of migration 1.15.47
// that allows one open visit per student and tenant.
const openVisitPerStudentIndex = "uniq_active_visits_open_per_student"

func (s *Store) RecordVisit(ctx context.Context, row *studentpresence.Visit) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	if row.TenantID != 0 && row.TenantID != tenantID {
		return ports.Stats{}, errors.New("student presence: visit belongs to another tenant")
	}
	row.TenantID = tenantID
	stored := visitRowFromRecord(row)
	started := time.Now()
	result, err := db.NewInsert().Model(stored).ModelTableExpr("active.visits").Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if isUniqueViolationOn(err, openVisitPerStudentIndex) {
		return stats, fmt.Errorf("record visit: %w", errors.Join(ports.ErrOpenVisitExists, err))
	}
	if err != nil {
		return stats, fmt.Errorf("record visit: %w", err)
	}
	*row = *stored.record()
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

func (s *Store) ReviseVisit(ctx context.Context, row *studentpresence.Visit) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	if row.TenantID != 0 && row.TenantID != tenantID {
		return ports.Stats{}, errors.New("student presence: visit belongs to another tenant")
	}
	row.TenantID = tenantID
	started := time.Now()
	// Match session end's lock order, including the destination of a move.
	// As in CloseVisits, the uncorrelated subquery locks sessions in id
	// order before the first visit row reaches the mutation.
	result, err := db.NewUpdate().Model(visitRowFromRecord(row)).ModelTableExpr(`active.visits AS "visit"`).
		WherePK().Where("visit.tenant_id = ?", tenantID).
		Where(`(SELECT count(*) FROM (
			SELECT g.id FROM active.groups AS g
			WHERE g.tenant_id = ? AND (g.id = ? OR g.id IN (
				SELECT v.active_group_id FROM active.visits AS v
				WHERE v.tenant_id = ? AND v.id = ?))
			ORDER BY g.id FOR NO KEY UPDATE OF g) AS locked_sessions) >= 0`, tenantID, row.ActiveGroupID, tenantID, row.ID).
		Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("revise visit: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, err
	}
	if stats.Rows != 1 {
		return stats, fmt.Errorf("expected 1 rows affected, got %d", stats.Rows)
	}
	return stats, nil
}

func (s *Store) DeleteVisit(ctx context.Context, id int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	// Lock the session before deleting its visit, just as session end does.
	result, err := db.NewDelete().Table("active.visits").Where("tenant_id = ?", tenantID).Where("id = ?", id).
		Where(`(SELECT count(*) FROM (
			SELECT g.id FROM active.groups AS g
			WHERE g.tenant_id = ? AND g.id IN (
				SELECT v.active_group_id FROM active.visits AS v
				WHERE v.tenant_id = ? AND v.id = ?)
			ORDER BY g.id FOR NO KEY UPDATE OF g) AS locked_sessions) >= 0`, tenantID, tenantID, id).
		Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("delete visit: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

// CloseVisits absorbs concurrent retries without reopening completed intervals.
func (s *Store) CloseVisits(ctx context.Context, ids []int64, at time.Time) ([]*studentpresence.Visit, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []*visitRow{}
	if len(ids) == 0 {
		return []*studentpresence.Visit{}, ports.Stats{}, nil
	}
	started := time.Now()
	// The sessions are locked before the visit rows, in ascending id order:
	// the visit revision trigger (#5128) locks the sessions it raises, and a
	// session end locks its session before its visits. The uncorrelated
	// subquery runs once before the first row reaches the update, so a
	// checkout and a session end of one session wait for each other instead
	// of deadlocking.
	err = db.NewUpdate().Model((*visitRow)(nil)).ModelTableExpr(`active.visits AS "visit"`).
		Set("exit_time = ?", at).Where("visit.tenant_id = ?", tenantID).
		Where("visit.id IN (?)", bun.List(ids)).Where("visit.exit_time IS NULL").
		Where(`(SELECT count(*) FROM (
			SELECT g.id FROM active.groups AS g
			WHERE g.tenant_id = ? AND g.id IN (
				SELECT v.active_group_id FROM active.visits AS v
				WHERE v.tenant_id = ? AND v.id IN (?) AND v.exit_time IS NULL)
			ORDER BY g.id FOR NO KEY UPDATE OF g) AS locked_sessions) >= 0`, tenantID, tenantID, bun.List(ids)).
		Returning("*").Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, stats, fmt.Errorf("close visits: %w", err)
	}
	return visitRecordsFromRows(rows), stats, nil
}
