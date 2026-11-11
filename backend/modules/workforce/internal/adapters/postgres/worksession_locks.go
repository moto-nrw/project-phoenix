package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The locking reads of open work sessions. Each admits the work session row
// as a leaf of the unit of work's lock order before it waits (#6702).

func (s *Store) LockOpenWorkSession(ctx context.Context, id int64) (workforce.WorkSession, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return workforce.WorkSession{}, false, domain.OperationStats{}, err
	}
	if err := tenant.OrderLock(ctx, tenant.LockLeaf, "active.work_sessions"); err != nil {
		return workforce.WorkSession{}, false, domain.OperationStats{}, err
	}
	row := &workSessionRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`).
		Where(`"work_session".id = ?`, id).
		Where(`"work_session".check_out_time IS NULL`), aliasWorkSession, scope).
		For("UPDATE")
	found, stats, err := scanOne(ctx, query, "lock open work session")
	if err != nil || !found {
		return workforce.WorkSession{}, found, stats, err
	}
	return workSessionToDomain(*row), true, stats, nil
}

func (s *Store) OpenWorkSessionOn(ctx context.Context, staffID int64, date string, lock bool) (workforce.WorkSession, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return workforce.WorkSession{}, false, domain.OperationStats{}, err
	}
	row := &workSessionRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableWorkSessions+` AS "work_session"`).
		Where(`"work_session".staff_id = ?`, staffID).
		Where(`"work_session".date = ?`, date).
		Where(`"work_session".check_out_time IS NULL`), aliasWorkSession, scope)
	if lock {
		if err := tenant.OrderLock(ctx, tenant.LockLeaf, "active.work_sessions"); err != nil {
			return workforce.WorkSession{}, false, domain.OperationStats{}, err
		}
		query = query.For("UPDATE")
	}
	found, stats, err := scanOne(ctx, query, "find open work session on day")
	if err != nil || !found {
		return workforce.WorkSession{}, found, stats, err
	}
	return workSessionToDomain(*row), true, stats, nil
}
