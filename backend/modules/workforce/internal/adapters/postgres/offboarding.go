package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// PreviewStaffOffboarding locks affected rows in table order and ascending ID
// order. The caller holds the staff balance and absence locks.
func (s *Store) PreviewStaffOffboarding(ctx context.Context, staffID int64, from string) (domain.OffboardingSnapshot, domain.OperationStats, error) {
	snapshot := domain.OffboardingSnapshot{StaffID: staffID, From: from}
	stats := domain.OperationStats{}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return snapshot, stats, err
	}
	absences := []staffAbsenceRow{}
	query := withTenant(db.NewSelect().Model(&absences).
		ModelTableExpr(`active.staff_absences AS "staff_absence"`).
		Where(`"staff_absence".staff_id = ?`, staffID).
		Where(`("staff_absence".status IN ('requested', 'question') OR "staff_absence".date_end >= ?)`, calendarDate(from)).
		OrderExpr(`"staff_absence".id ASC`).For("UPDATE"), aliasStaffAbsence, tenantID)
	queryStats, err := scanAll(ctx, query, "preview offboarding absences")
	stats.Add(queryStats)
	if err != nil {
		return snapshot, stats, err
	}
	snapshot.Absences = staffAbsencesToDomain(absences)
	series := []staffShiftSeriesRow{}
	query = withTenant(db.NewSelect().Model(&series).
		ModelTableExpr(`schedule.staff_shift_series AS "staff_shift_series"`).
		Where(`"staff_shift_series".staff_id = ?`, staffID).
		Where(`("staff_shift_series".valid_until IS NULL OR "staff_shift_series".valid_until > GREATEST("staff_shift_series".valid_from, ?))`, calendarDate(from)).
		OrderExpr(`"staff_shift_series".id ASC`).For("UPDATE"), aliasStaffShiftSeries, tenantID)
	queryStats, err = scanAll(ctx, query, "preview offboarding series")
	stats.Add(queryStats)
	if err != nil {
		return snapshot, stats, err
	}
	for _, row := range series {
		snapshot.Series = append(snapshot.Series, staffShiftSeriesToDomain(row))
	}
	shifts := []staffShiftRow{}
	query = withTenant(db.NewSelect().Model(&shifts).
		ModelTableExpr(`schedule.staff_shifts AS "staff_shift"`).
		Where(`"staff_shift".staff_id = ?`, staffID).
		Where(`"staff_shift".date >= ?`, calendarDate(from)).
		OrderExpr(`"staff_shift".id ASC`).For("UPDATE"), aliasStaffShift, tenantID)
	queryStats, err = scanAll(ctx, query, "preview offboarding shifts")
	stats.Add(queryStats)
	if err != nil {
		return snapshot, stats, err
	}
	snapshot.Shifts = staffShiftsToDomain(shifts)
	substitutions := []groupSubstitutionRow{}
	query = withTenant(db.NewSelect().Model(&substitutions).
		ModelTableExpr(`education.group_substitution AS "group_substitution"`).
		Where(`("group_substitution".regular_staff_id = ? OR "group_substitution".substitute_staff_id = ?)`, staffID, staffID).
		Where(`"group_substitution".end_date >= ?`, calendarDate(from)).
		OrderExpr(`"group_substitution".id ASC`).For("UPDATE"), aliasGroupSubstitution, tenantID)
	queryStats, err = scanAll(ctx, query, "preview offboarding substitutions")
	stats.Add(queryStats)
	if err != nil {
		return snapshot, stats, err
	}
	for _, row := range substitutions {
		snapshot.Substitutions = append(snapshot.Substitutions, groupSubstitutionToDomain(row))
	}
	return snapshot, stats, nil
}
