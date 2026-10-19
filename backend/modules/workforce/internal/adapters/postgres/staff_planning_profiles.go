package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

// StaffPlanningProfiles batches only Workforce-owned facts. Both joins carry
// the required school explicitly, including profiles without schedule rows.
func (s *Store) StaffPlanningProfiles(ctx context.Context, staffIDs []int64, from, to string) ([]domain.StaffPlanningProfile, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if scope.AcrossSchools {
		return nil, domain.OperationStats{}, fmt.Errorf("staff planning requires one school")
	}
	if len(staffIDs) == 0 {
		return nil, domain.OperationStats{}, nil
	}
	var rows []struct {
		MembershipID       int64
		WorkTimeModelID    *int64
		RotationAnchorDate *calendarDate
		Schedule           staffWorkScheduleRow `bun:"embed:schedule_"`
	}
	query := db.NewSelect().Model(&rows).
		TableExpr("unnest(ARRAY[?]::bigint[]) AS member(id)", bun.List(staffIDs)).
		ColumnExpr("member.id AS membership_id, profile.work_time_model_id, profile.rotation_anchor_date").
		Join("LEFT JOIN users.staff_employment_profiles AS profile ON profile.membership_id = member.id AND profile.tenant_id = ?", scope.TenantID).
		Join("LEFT JOIN config.staff_work_schedules AS staff_work_schedule ON staff_work_schedule.staff_id = member.id AND staff_work_schedule.tenant_id = ? AND staff_work_schedule.valid_from <= ? AND (staff_work_schedule.valid_until IS NULL OR staff_work_schedule.valid_until > ?)", scope.TenantID, nullableDate(to), nullableDate(from)).
		OrderExpr("member.id ASC, staff_work_schedule.day_of_week ASC, staff_work_schedule.id ASC")
	for _, column := range []string{"id", "tenant_id", "staff_id", "week_index", "rotation_length", "day_of_week", "target_minutes", "start_time", "rotation_anchor_date", "valid_from", "valid_until", "created_at", "updated_at"} {
		query.ColumnExpr("? AS ?", bun.Ident("staff_work_schedule."+column), bun.Ident("schedule_"+column))
	}
	stats, err := scanAll(ctx, query, "get staff planning profiles")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.StaffPlanningProfile, 0, len(staffIDs))
	positions := make(map[int64]int, len(staffIDs))
	for _, row := range rows {
		position, found := positions[row.MembershipID]
		if !found {
			position = len(result)
			positions[row.MembershipID] = position
			result = append(result, domain.StaffPlanningProfile{MembershipID: row.MembershipID, WorkTimeModelID: row.WorkTimeModelID, RotationAnchorDate: calendarDateString(row.RotationAnchorDate)})
		}
		if row.Schedule.ID != 0 {
			result[position].Schedules = append(result[position].Schedules, staffSchedulesToDomain([]staffWorkScheduleRow{row.Schedule})[0])
		}
	}
	return result, stats, nil
}
