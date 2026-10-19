package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

// PreviewStaffOffboarding locks affected rows in table order and ascending ID
// order. The caller holds the staff balance and absence locks.
func (s *Store) PreviewStaffOffboarding(ctx context.Context, staffID int64, from string) (domain.OffboardingSnapshot, domain.OperationStats, error) {
	snapshot := domain.OffboardingSnapshot{StaffID: staffID, From: from}
	stats := domain.OperationStats{}
	db, scope, err := s.database(ctx)
	if err != nil {
		return snapshot, stats, err
	}
	lock := offboardingLock{db: db, scope: scope, staffID: staffID, from: calendarDate(from), stats: &stats}
	if snapshot.Absences, err = lock.absences(ctx); err != nil {
		return snapshot, stats, err
	}
	if snapshot.Series, err = lock.series(ctx); err != nil {
		return snapshot, stats, err
	}
	if snapshot.Shifts, err = lock.shifts(ctx); err != nil {
		return snapshot, stats, err
	}
	if snapshot.Substitutions, err = lock.substitutions(ctx); err != nil {
		return snapshot, stats, err
	}
	return snapshot, stats, nil
}

// offboardingLock reads and row-locks one staff member's rows an offboarding
// from the given day touches, adding each statement to stats.
type offboardingLock struct {
	db      bun.IDB
	scope   TenantScope
	staffID int64
	from    calendarDate
	stats   *domain.OperationStats
}

func (l offboardingLock) scan(ctx context.Context, query *bun.SelectQuery, operation string) error {
	queryStats, err := scanAll(ctx, query, operation)
	l.stats.Add(queryStats)
	return err
}

// absences locks open requests and every absence that has not ended before from.
func (l offboardingLock) absences(ctx context.Context) ([]domain.StaffAbsence, error) {
	absences := []staffAbsenceRow{}
	query := withTenant(l.db.NewSelect().Model(&absences).
		ModelTableExpr(`active.staff_absences AS "staff_absence"`).
		Where(`"staff_absence".staff_id = ?`, l.staffID).
		Where(`("staff_absence".status IN ('requested', 'question') OR "staff_absence".date_end >= ?)`, l.from).
		OrderExpr(`"staff_absence".id ASC`).For("UPDATE"), aliasStaffAbsence, l.scope)
	if err := l.scan(ctx, query, "preview offboarding absences"); err != nil {
		return nil, err
	}
	return staffAbsencesToDomain(absences), nil
}

// series locks every series still valid after from.
func (l offboardingLock) series(ctx context.Context) ([]domain.StaffShiftSeries, error) {
	series := []staffShiftSeriesRow{}
	query := withTenant(l.db.NewSelect().Model(&series).
		ModelTableExpr(`schedule.staff_shift_series AS "staff_shift_series"`).
		Where(`"staff_shift_series".staff_id = ?`, l.staffID).
		Where(`("staff_shift_series".valid_until IS NULL OR "staff_shift_series".valid_until > GREATEST("staff_shift_series".valid_from, ?))`, l.from).
		OrderExpr(`"staff_shift_series".id ASC`).For("UPDATE"), aliasStaffShiftSeries, l.scope)
	if err := l.scan(ctx, query, "preview offboarding series"); err != nil {
		return nil, err
	}
	var result []domain.StaffShiftSeries
	for _, row := range series {
		result = append(result, staffShiftSeriesToDomain(row))
	}
	return result, nil
}

// shifts locks every shift on or after from.
func (l offboardingLock) shifts(ctx context.Context) ([]domain.StaffShift, error) {
	shifts := []staffShiftRow{}
	query := withTenant(l.db.NewSelect().Model(&shifts).
		ModelTableExpr(`schedule.staff_shifts AS "staff_shift"`).
		Where(`"staff_shift".staff_id = ?`, l.staffID).
		Where(`"staff_shift".date >= ?`, l.from).
		OrderExpr(`"staff_shift".id ASC`).For("UPDATE"), aliasStaffShift, l.scope)
	if err := l.scan(ctx, query, "preview offboarding shifts"); err != nil {
		return nil, err
	}
	return staffShiftsToDomain(shifts), nil
}

// substitutions locks every substitution the staff member takes part in that
// has not ended before from.
func (l offboardingLock) substitutions(ctx context.Context) ([]domain.GroupSubstitution, error) {
	substitutions := []groupSubstitutionRow{}
	query := withTenant(l.db.NewSelect().Model(&substitutions).
		ModelTableExpr(`education.group_substitution AS "group_substitution"`).
		Where(`("group_substitution".regular_staff_id = ? OR "group_substitution".substitute_staff_id = ?)`, l.staffID, l.staffID).
		Where(`"group_substitution".end_date >= ?`, l.from).
		OrderExpr(`"group_substitution".id ASC`).For("UPDATE"), aliasGroupSubstitution, l.scope)
	if err := l.scan(ctx, query, "preview offboarding substitutions"); err != nil {
		return nil, err
	}
	var result []domain.GroupSubstitution
	for _, row := range substitutions {
		result = append(result, groupSubstitutionToDomain(row))
	}
	return result, nil
}
