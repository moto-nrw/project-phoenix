package timetable

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

func (rs *Resource) templateRosterValidFrom(ctx context.Context, calendarPeriodID *int64) (timezone.Date, error) {
	if calendarPeriodID == nil {
		return timezone.TodayDate(), nil
	}
	if rs.calendarPeriodService == nil {
		return timezone.Date{}, errors.New("calendar period service not wired")
	}
	period, err := rs.calendarPeriodService.GetPeriodByID(ctx, *calendarPeriodID)
	if err != nil {
		return timezone.Date{}, err
	}
	return period.StartDate, nil
}

func renderTemplatePeriodLookupError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("calendar period not found")))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServerWrap("load calendar period failed", err))
}

func (rs *Resource) replaceTemplateStudents(ctx context.Context, groupID int64, studentIDs []int64, calendarPeriodID *int64, validFrom timezone.Date) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil
	}
	if err := rs.timetableData.CloseOpenEnrollmentsByGroupAndPeriod(ctx, groupID, calendarPeriodID, validFrom); err != nil {
		return err
	}
	for _, studentID := range uniquePositiveIDs(studentIDs) {
		row := &activitiesModel.StudentEnrollment{
			StudentID:        studentID,
			ActivityGroupID:  groupID,
			ValidFrom:        validFrom,
			CalendarPeriodID: calendarPeriodID,
		}
		row.SetTenantID(tenantID)
		if err := rs.timetableData.CreateStudentEnrollment(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

func (rs *Resource) replaceTemplateStaff(ctx context.Context, groupID int64, staffIDs []int64, primaryStaffID *int64, calendarPeriodID *int64, validFrom timezone.Date) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil
	}
	if err := rs.timetableData.CloseOpenSupervisorsByGroupAndPeriod(ctx, groupID, calendarPeriodID, validFrom); err != nil {
		return err
	}
	for _, staffID := range uniquePositiveIDs(staffIDs) {
		isPrimary := primaryStaffID != nil && *primaryStaffID == staffID
		row := &activitiesModel.SupervisorPlanned{
			StaffID:          staffID,
			GroupID:          groupID,
			IsPrimary:        isPrimary,
			ValidFrom:        validFrom,
			CalendarPeriodID: calendarPeriodID,
		}
		row.SetTenantID(tenantID)
		if err := rs.timetableData.CreatePlannedSupervisor(ctx, row); err != nil {
			return err
		}
	}
	return nil
}
