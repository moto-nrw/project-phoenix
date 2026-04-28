package timetable

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func uniquePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (rs *Resource) replaceTemplateStudents(ctx context.Context, groupID int64, studentIDs []int64, calendarPeriodID *int64) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil
	}
	now := time.Now()
	if _, err := base.GetDB(ctx, rs.db).NewUpdate().
		Table("activities.student_enrollments").
		Set("valid_until = ?", now).
		Where("tenant_id = ?", tenantID).
		Where("activity_group_id = ?", groupID).
		Where("valid_until IS NULL").
		Exec(ctx); err != nil {
		return err
	}
	for _, studentID := range uniquePositiveIDs(studentIDs) {
		row := &activitiesModel.StudentEnrollment{
			StudentID:        studentID,
			ActivityGroupID:  groupID,
			ValidFrom:        now,
			CalendarPeriodID: calendarPeriodID,
		}
		row.SetTenantID(tenantID)
		if err := rs.studentEnrollmentRepo.Create(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

func (rs *Resource) replaceTemplateStaff(ctx context.Context, groupID int64, staffIDs []int64, primaryStaffID *int64, calendarPeriodID *int64) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil
	}
	now := time.Now()
	if _, err := base.GetDB(ctx, rs.db).NewUpdate().
		Table("activities.supervisors").
		Set("valid_until = ?", now).
		Where("tenant_id = ?", tenantID).
		Where("group_id = ?", groupID).
		Where("valid_until IS NULL").
		Exec(ctx); err != nil {
		return err
	}
	for _, staffID := range uniquePositiveIDs(staffIDs) {
		isPrimary := primaryStaffID != nil && *primaryStaffID == staffID
		row := &activitiesModel.SupervisorPlanned{
			StaffID:          staffID,
			GroupID:          groupID,
			IsPrimary:        isPrimary,
			ValidFrom:        now,
			CalendarPeriodID: calendarPeriodID,
		}
		row.SetTenantID(tenantID)
		if err := rs.activitySupervisorRepo.Create(ctx, row); err != nil {
			return err
		}
	}
	return nil
}
