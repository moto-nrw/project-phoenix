package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/uptrace/bun"
)

type teacherWithStaffRow struct {
	teacherRow
	Staff *staffRow `bun:"staff"`
}

// Both sides belong to School Membership. Reuse staffSelect for the active
// external-caregiver window and lifecycle fields; no Workforce or People SQL.
func listTeachersWithStaff(ctx context.Context, db bun.IDB, scope TenantScope, filter schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, domain.OperationStats, error) {
	rows := []teacherWithStaffRow{}
	staff := withTenant(staffSelect(db, (*staffRow)(nil), false), "staff", scope).
		Where(`"staff".deleted_at IS NULL`)
	query := teacherSelect(db, &rows).ColumnExpr(`"teacher".*`).
		Join(`LEFT JOIN (?) AS "staff" ON "staff".id = "teacher".staff_id AND "staff".tenant_id = "teacher".tenant_id`, staff)
	for _, column := range []string{
		"id", "created_at", "updated_at", "tenant_id", "person_id", "deleted_at", "personal_data_erased_at",
		"has_external_caregiver_record", "external_organization", "planned_exit_day",
		"planned_exit_by_account_id", "planned_exit_by_staff_id", "planned_exit_by_name", "planned_exit_at",
		"exit_week_notice_sent_at", "exit_day_notice_sent_at", "exit_blocked_notice_sent_at",
		"exit_access_locked_at", "exit_next_attempt_at",
	} {
		query = query.ColumnExpr(`"staff".? AS ?`, bun.Ident(column), bun.Ident("staff__"+column))
	}
	query, ok := teacherListQuery(withTenant(query, "teacher", scope), filter)
	if !ok {
		return []schoolmembership.Teacher{}, domain.OperationStats{}, nil
	}
	stats, err := scanAll(ctx, query.OrderExpr(`"teacher".id ASC`), "list teachers with staff")
	if err != nil {
		return nil, stats, err
	}
	result := make([]schoolmembership.Teacher, 0, len(rows))
	for _, row := range rows {
		teacher := teacherToPublic(row.teacherRow)
		if row.Staff != nil && row.Staff.ID != 0 {
			member := staffToPublic(*row.Staff)
			teacher.Staff = &member
		}
		result = append(result, teacher)
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}
