package students

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const maxStudentStatusDayRangeDays = 31

var errStudentStatusDayReassigned = errors.New("student reassigned out of caller scope")

func (rs *Resource) getStudentStatusDays(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	if !rs.checkStudentReadAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(errors.New("full access required")))
		return
	}
	if rs.StudentStatusDayService == nil {
		common.Respond(w, r, http.StatusOK, []StudentStatusDayResponse{}, "Student status days retrieved successfully")
		return
	}

	from, to, err := parseStatusDayRange(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	rows, err := rs.StudentStatusDayService.GetActiveByStudentAndDateRange(r.Context(), student.ID, from, to)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to fetch student status days", err))
		return
	}

	common.Respond(w, r, http.StatusOK, newStudentStatusDayResponses(rows), "Student status days retrieved successfully")
}

func (rs *Resource) createStudentStatusDays(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	req := &CreateStudentStatusDaysRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if rs.StudentStatusDayService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("student status day repository not configured")))
		return
	}

	userPermissions := jwt.PermissionsFromCtx(r.Context())
	authorized, authErr := canUpdateStudent(r.Context(), userPermissions, student, rs.UserContextService)
	if !authorized {
		renderError(w, r, common.ErrorForbidden(authErr))
		return
	}

	dates, err := parseStatusDayDates(req.Dates)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	now := time.Now()
	today := timezone.TodayDate()
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		fresh, err := rs.StudentService.GetByIDForUpdate(ctx, student.ID)
		if err != nil {
			return err
		}
		if ok, _ := canUpdateStudent(ctx, userPermissions, fresh, rs.UserContextService); !ok {
			return errStudentStatusDayReassigned
		}

		if err := rs.clearOtherStatusDaysForDates(ctx, fresh.ID, req.Status, dates, now); err != nil {
			return err
		}
		notePtr := strutil.TrimPtrToNil(&req.Reason)
		for _, date := range dates {
			if err := rs.StudentStatusDayService.UpsertReported(ctx, &active.StudentStatusDay{
				StudentID:  fresh.ID,
				Date:       date,
				Status:     req.Status,
				ReportedAt: now,
				Source:     active.StudentStatusSourcePlanned,
				Note:       notePtr,
			}); err != nil {
				return err
			}
		}
		if slices.Contains(dates, today) {
			applyLiveStatusForToday(fresh, req.Status, now)
			if err := rs.StudentService.Update(ctx, fresh); err != nil {
				return err
			}
		}

		studentID := student.ID
		capturedTenantID := tenantID
		tenant.RegisterAfterCommit(ctx, func() {
			rs.broadcastStudentUpdated(capturedTenantID, studentID)
			// Also wake the child's guardians so an open parents-app tab reflects
			// the new/cleared absence (today_absent → the "Heute" pickup tile)
			// live; the tenant-wide student_updated above never reaches the parent
			// SSE stream (#1725).
			rs.wakeChildGuardians(capturedTenantID, studentID)
		})
		return nil
	}); err != nil {
		if errors.Is(err, errStudentStatusDayReassigned) {
			renderError(w, r, common.ErrorForbidden(err))
			return
		}
		renderError(w, r, common.ErrorInternalServerWrap("failed to create student status days", err))
		return
	}

	rows, err := rs.StudentStatusDayService.GetActiveByStudentAndDateRange(r.Context(), student.ID, slices.MinFunc(dates, timezone.Date.Compare), slices.MaxFunc(dates, timezone.Date.Compare))
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to fetch student status days", err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newStudentStatusDayResponses(rows), "Student status days created successfully")
}

func (rs *Resource) bulkCreateStudentStatusDays(w http.ResponseWriter, r *http.Request) {
	req := &BulkCreateStudentStatusDaysRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if rs.StudentStatusDayService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("student status day repository not configured")))
		return
	}

	from, err := timezone.ParseDate(req.From)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return
	}
	to, err := timezone.ParseDate(req.To)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return
	}
	if to.Before(from) {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("to must be after from")))
		return
	}
	if to.After(from.AddDays(maxStudentStatusDayRangeDays - 1)) {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("date range cannot exceed 31 days")))
		return
	}
	dates := datesBetweenInclusive(from, to)

	userPermissions := jwt.PermissionsFromCtx(r.Context())
	now := time.Now()
	today := timezone.TodayDate()
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		for _, studentID := range req.StudentIDs {
			fresh, err := rs.StudentService.GetByIDForUpdate(ctx, studentID)
			if err != nil {
				return err
			}
			if ok, _ := canUpdateStudent(ctx, userPermissions, fresh, rs.UserContextService); !ok {
				return errStudentStatusDayReassigned
			}
			if err := rs.clearOtherStatusDaysForDates(ctx, fresh.ID, req.Status, dates, now); err != nil {
				return err
			}
			notePtr := strutil.TrimPtrToNil(&req.Reason)
			for _, date := range dates {
				if err := rs.StudentStatusDayService.UpsertReported(ctx, &active.StudentStatusDay{
					StudentID:  fresh.ID,
					Date:       date,
					Status:     req.Status,
					ReportedAt: now,
					Source:     active.StudentStatusSourcePlanned,
					Note:       notePtr,
				}); err != nil {
					return err
				}
			}
			if slices.Contains(dates, today) {
				applyLiveStatusForToday(fresh, req.Status, now)
				if err := rs.StudentService.Update(ctx, fresh); err != nil {
					return err
				}
			}
			studentID := fresh.ID
			capturedTenantID := tenantID
			tenant.RegisterAfterCommit(ctx, func() {
				rs.broadcastStudentUpdated(capturedTenantID, studentID)
				// Also wake the child's guardians so an open parents-app tab
				// reflects the new/cleared absence live (#1725).
				rs.wakeChildGuardians(capturedTenantID, studentID)
			})
		}
		return nil
	}); err != nil {
		if errors.Is(err, errStudentStatusDayReassigned) {
			renderError(w, r, common.ErrorForbidden(err))
			return
		}
		renderError(w, r, common.ErrorInternalServerWrap("failed to bulk create student status days", err))
		return
	}

	common.Respond(w, r, http.StatusCreated, map[string]any{
		"student_count": len(req.StudentIDs),
		"date_count":    len(dates),
	}, "Student status days created successfully")
}

func (rs *Resource) deleteStudentStatusDay(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	statusDayID, ok := common.ParsePositiveInt64IDWithError(w, r, "statusDayId", "invalid status day id")
	if !ok {
		return
	}
	if rs.StudentStatusDayService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("student status day repository not configured")))
		return
	}

	userPermissions := jwt.PermissionsFromCtx(r.Context())
	authorized, authErr := canUpdateStudent(r.Context(), userPermissions, student, rs.UserContextService)
	if !authorized {
		renderError(w, r, common.ErrorForbidden(authErr))
		return
	}

	now := time.Now()
	today := timezone.TodayDate()
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		row, err := rs.StudentStatusDayService.GetActiveByID(ctx, statusDayID)
		if err != nil {
			return err
		}
		if row.StudentID != student.ID {
			return sql.ErrNoRows
		}

		fresh, err := rs.StudentService.GetByIDForUpdate(ctx, student.ID)
		if err != nil {
			return err
		}
		if ok, _ := canUpdateStudent(ctx, userPermissions, fresh, rs.UserContextService); !ok {
			return errStudentStatusDayReassigned
		}

		if err := rs.StudentStatusDayService.MarkClearedByID(ctx, row.ID, now, active.StudentStatusSourceManual); err != nil {
			return err
		}
		if row.Date == today {
			clearLiveStatusForToday(fresh, row.Status)
			if err := rs.StudentService.Update(ctx, fresh); err != nil {
				return err
			}
		}

		studentID := student.ID
		capturedTenantID := tenantID
		tenant.RegisterAfterCommit(ctx, func() {
			rs.broadcastStudentUpdated(capturedTenantID, studentID)
			// Also wake the child's guardians so an open parents-app tab reflects
			// the new/cleared absence (today_absent → the "Heute" pickup tile)
			// live; the tenant-wide student_updated above never reaches the parent
			// SSE stream (#1725).
			rs.wakeChildGuardians(capturedTenantID, studentID)
		})
		return nil
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			renderError(w, r, common.ErrorNotFound(errors.New("student status day not found")))
			return
		}
		if errors.Is(err, errStudentStatusDayReassigned) {
			renderError(w, r, common.ErrorForbidden(err))
			return
		}
		renderError(w, r, common.ErrorInternalServerWrap("failed to delete student status day", err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]bool{"deleted": true}, "Student status day deleted successfully")
}

func parseStatusDayRange(r *http.Request) (timezone.Date, timezone.Date, error) {
	today := timezone.TodayDate()
	fromRaw := r.URL.Query().Get("from")
	toRaw := r.URL.Query().Get("to")

	from := today
	// Two calendar months ahead, mirroring time.Time.AddDate(0, 2, 0).
	to := timezone.NewDate(today.Year, today.Month+2, today.Day)
	var err error
	if fromRaw != "" {
		if from, err = timezone.ParseDate(fromRaw); err != nil {
			return timezone.Date{}, timezone.Date{}, errors.New("invalid from date format, expected YYYY-MM-DD")
		}
	}
	if toRaw != "" {
		if to, err = timezone.ParseDate(toRaw); err != nil {
			return timezone.Date{}, timezone.Date{}, errors.New("invalid to date format, expected YYYY-MM-DD")
		}
	}
	if to.Before(from) {
		return timezone.Date{}, timezone.Date{}, errors.New("to must be after from")
	}
	return from, to, nil
}

func parseStatusDayDates(rawDates []string) ([]timezone.Date, error) {
	dates := make([]timezone.Date, 0, len(rawDates))
	for _, rawDate := range rawDates {
		date, err := timezone.ParseDate(rawDate)
		if err != nil {
			return nil, errors.New("invalid date format, expected YYYY-MM-DD")
		}
		dates = append(dates, date)
	}
	return dates, nil
}

func datesBetweenInclusive(from, to timezone.Date) []timezone.Date {
	dates := make([]timezone.Date, 0, from.DaysUntil(to)+1)
	for date := from; !date.After(to); date = date.AddDays(1) {
		dates = append(dates, date)
	}
	return dates
}

func (rs *Resource) clearOtherStatusDaysForDates(ctx context.Context, studentID int64, status string, dates []timezone.Date, now time.Time) error {
	for _, otherStatus := range active.StudentStatusDayStatusesExcept(status) {
		if err := rs.StudentStatusDayService.MarkClearedForDates(ctx, studentID, otherStatus, dates, now, active.StudentStatusSourceManual); err != nil {
			return err
		}
	}
	return nil
}

func applyLiveStatusForToday(student *users.Student, status string, now time.Time) {
	trueVal := true
	falseVal := false
	switch status {
	case active.StudentStatusDaySick:
		student.Sick = &trueVal
		student.SickSince = &now
		student.Excused = &falseVal
		student.ExcusedSince = nil
	case active.StudentStatusDayExcused:
		student.Excused = &trueVal
		student.ExcusedSince = &now
		student.Sick = &falseVal
		student.SickSince = nil
	case active.StudentStatusDayClassTrip:
		student.Sick = &falseVal
		student.SickSince = nil
		student.Excused = &falseVal
		student.ExcusedSince = nil
	}
}

func clearLiveStatusForToday(student *users.Student, status string) {
	falseVal := false
	switch status {
	case active.StudentStatusDaySick:
		student.Sick = &falseVal
		student.SickSince = nil
	case active.StudentStatusDayExcused:
		student.Excused = &falseVal
		student.ExcusedSince = nil
	case active.StudentStatusDayClassTrip:
		student.Sick = &falseVal
		student.SickSince = nil
		student.Excused = &falseVal
		student.ExcusedSince = nil
	}
}
