package postgres

import (
	"context"
	"fmt"
	"time"

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
	query := db.NewRaw(`WITH member AS (SELECT unnest(ARRAY[?]::bigint[]) AS id)
 SELECT member.id AS membership_id, profile.work_time_model_id, profile.rotation_anchor_date,
  staff_work_schedule.id AS schedule_id, staff_work_schedule.tenant_id AS schedule_tenant_id,
  staff_work_schedule.staff_id AS schedule_staff_id, staff_work_schedule.week_index AS schedule_week_index,
  staff_work_schedule.rotation_length AS schedule_rotation_length, staff_work_schedule.day_of_week AS schedule_day_of_week,
  staff_work_schedule.target_minutes AS schedule_target_minutes, staff_work_schedule.start_time AS schedule_start_time,
  staff_work_schedule.rotation_anchor_date AS schedule_rotation_anchor_date, staff_work_schedule.valid_from AS schedule_valid_from,
  staff_work_schedule.valid_until AS schedule_valid_until, staff_work_schedule.created_at AS schedule_created_at,
  staff_work_schedule.updated_at AS schedule_updated_at
 FROM member
 LEFT JOIN users.staff_employment_profiles AS profile ON profile.membership_id = member.id AND profile.tenant_id = ?
 LEFT JOIN config.staff_work_schedules AS staff_work_schedule ON staff_work_schedule.staff_id = member.id
  AND staff_work_schedule.tenant_id = ? AND staff_work_schedule.valid_from <= ?
  AND (staff_work_schedule.valid_until IS NULL OR staff_work_schedule.valid_until > ?)
 ORDER BY member.id, staff_work_schedule.day_of_week, staff_work_schedule.id`,
		bun.List(staffIDs), scope.TenantID, scope.TenantID, nullableDate(to), nullableDate(from))
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
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
