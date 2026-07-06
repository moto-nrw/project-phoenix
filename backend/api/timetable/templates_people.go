package timetable

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (rs *Resource) templateRosterValidFrom(ctx context.Context, calendarPeriodID *int64) (timezone.Date, error) {
	if calendarPeriodID == nil {
		return timezone.TodayDate(), nil
	}
	if rs.CalendarPeriodService == nil {
		return timezone.Date{}, errors.New("calendar period service not wired")
	}
	period, err := rs.CalendarPeriodService.GetPeriodByID(ctx, *calendarPeriodID)
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
	if err := rs.TimetableData.CloseOpenEnrollmentsByGroupAndPeriod(ctx, groupID, calendarPeriodID, validFrom); err != nil {
		return err
	}
	for _, studentID := range sliceutil.UniquePositive(studentIDs) {
		row := &activitiesModel.StudentEnrollment{
			StudentID:        studentID,
			ActivityGroupID:  groupID,
			ValidFrom:        validFrom,
			CalendarPeriodID: calendarPeriodID,
		}
		row.SetTenantID(tenantID)
		if err := rs.TimetableData.CreateStudentEnrollment(ctx, row); err != nil {
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
	if err := rs.TimetableData.CloseOpenSupervisorsByGroupAndPeriod(ctx, groupID, calendarPeriodID, validFrom); err != nil {
		return err
	}
	for _, staffID := range sliceutil.UniquePositive(staffIDs) {
		isPrimary := primaryStaffID != nil && *primaryStaffID == staffID
		row := &activitiesModel.SupervisorPlanned{
			StaffID:          staffID,
			GroupID:          groupID,
			IsPrimary:        isPrimary,
			ValidFrom:        validFrom,
			CalendarPeriodID: calendarPeriodID,
		}
		row.SetTenantID(tenantID)
		if err := rs.TimetableData.CreatePlannedSupervisor(ctx, row); err != nil {
			return err
		}
	}
	return nil
}
