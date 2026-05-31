package students

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var errStudentStatusDayReassigned = errors.New("student reassigned out of caller scope")

func (rs *Resource) getStudentStatusDays(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	if !rs.checkStudentReadAccess(r, student) {
		renderError(w, r, ErrorForbidden(errors.New("full access required")))
		return
	}
	if rs.StudentStatusDayRepo == nil {
		common.Respond(w, r, http.StatusOK, []StudentStatusDayResponse{}, "Student status days retrieved successfully")
		return
	}

	from, to, err := parseStatusDayRange(r)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	rows, err := rs.StudentStatusDayRepo.FindActiveByStudentAndDateRange(r.Context(), student.ID, from, to)
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
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}
	if rs.StudentStatusDayRepo == nil {
		renderError(w, r, ErrorInternalServer(errors.New("student status day repository not configured")))
		return
	}

	userPermissions := jwt.PermissionsFromCtx(r.Context())
	authorized, authErr := canUpdateStudent(r.Context(), userPermissions, student, rs.UserContextService)
	if !authorized {
		renderError(w, r, ErrorForbidden(authErr))
		return
	}

	dates, err := parseStatusDayDates(req.Dates)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	now := time.Now()
	today := timezone.DateOfUTC(now)
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		fresh, err := rs.StudentRepo.FindByIDForUpdate(ctx, student.ID)
		if err != nil {
			return err
		}
		if ok, _ := canUpdateStudent(ctx, userPermissions, fresh, rs.UserContextService); !ok {
			return errStudentStatusDayReassigned
		}

		oppositeStatus := active.StudentStatusDayExcused
		if req.Status == active.StudentStatusDayExcused {
			oppositeStatus = active.StudentStatusDaySick
		}
		if err := rs.StudentStatusDayRepo.MarkClearedForDates(ctx, fresh.ID, oppositeStatus, dates, now, active.StudentStatusSourceManual); err != nil {
			return err
		}
		for _, date := range dates {
			if err := rs.StudentStatusDayRepo.UpsertReported(ctx, &active.StudentStatusDay{
				StudentID:  fresh.ID,
				Date:       date,
				Status:     req.Status,
				ReportedAt: now,
				Source:     active.StudentStatusSourcePlanned,
			}); err != nil {
				return err
			}
		}
		if containsDate(dates, today) {
			applyLiveStatusForToday(fresh, req.Status, now)
			if err := rs.StudentRepo.Update(ctx, fresh); err != nil {
				return err
			}
		}

		studentID := student.ID
		capturedTenantID := tenantID
		tenant.RegisterAfterCommit(ctx, func() {
			rs.broadcastStudentUpdated(capturedTenantID, studentID)
		})
		return nil
	}); err != nil {
		if errors.Is(err, errStudentStatusDayReassigned) {
			renderError(w, r, ErrorForbidden(err))
			return
		}
		renderError(w, r, common.ErrorInternalServerWrap("failed to create student status days", err))
		return
	}

	rows, err := rs.StudentStatusDayRepo.FindActiveByStudentAndDateRange(r.Context(), student.ID, minDate(dates), maxDate(dates))
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to fetch student status days", err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newStudentStatusDayResponses(rows), "Student status days created successfully")
}

func (rs *Resource) deleteStudentStatusDay(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	statusDayID, err := strconv.ParseInt(chi.URLParam(r, "statusDayId"), 10, 64)
	if err != nil || statusDayID <= 0 {
		renderError(w, r, ErrorInvalidRequest(errors.New("invalid status day id")))
		return
	}
	if rs.StudentStatusDayRepo == nil {
		renderError(w, r, ErrorInternalServer(errors.New("student status day repository not configured")))
		return
	}

	userPermissions := jwt.PermissionsFromCtx(r.Context())
	authorized, authErr := canUpdateStudent(r.Context(), userPermissions, student, rs.UserContextService)
	if !authorized {
		renderError(w, r, ErrorForbidden(authErr))
		return
	}

	now := time.Now()
	today := timezone.DateOfUTC(now)
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		row, err := rs.StudentStatusDayRepo.FindActiveByID(ctx, statusDayID)
		if err != nil {
			return err
		}
		if row.StudentID != student.ID {
			return sql.ErrNoRows
		}

		fresh, err := rs.StudentRepo.FindByIDForUpdate(ctx, student.ID)
		if err != nil {
			return err
		}
		if ok, _ := canUpdateStudent(ctx, userPermissions, fresh, rs.UserContextService); !ok {
			return errStudentStatusDayReassigned
		}

		if err := rs.StudentStatusDayRepo.MarkClearedByID(ctx, row.ID, now, active.StudentStatusSourceManual); err != nil {
			return err
		}
		if timezone.DateOfUTC(row.Date).Equal(today) {
			clearLiveStatusForToday(fresh, row.Status)
			if err := rs.StudentRepo.Update(ctx, fresh); err != nil {
				return err
			}
		}

		studentID := student.ID
		capturedTenantID := tenantID
		tenant.RegisterAfterCommit(ctx, func() {
			rs.broadcastStudentUpdated(capturedTenantID, studentID)
		})
		return nil
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			renderError(w, r, ErrorNotFound(errors.New("student status day not found")))
			return
		}
		if errors.Is(err, errStudentStatusDayReassigned) {
			renderError(w, r, ErrorForbidden(err))
			return
		}
		renderError(w, r, common.ErrorInternalServerWrap("failed to delete student status day", err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]bool{"deleted": true}, "Student status day deleted successfully")
}

func parseStatusDayRange(r *http.Request) (time.Time, time.Time, error) {
	now := time.Now()
	fromRaw := r.URL.Query().Get("from")
	toRaw := r.URL.Query().Get("to")
	if fromRaw == "" {
		fromRaw = timezone.DateOfUTC(now).Format(dateFormatYYYYMMDD)
	}
	if toRaw == "" {
		toRaw = timezone.DateOfUTC(now.AddDate(0, 2, 0)).Format(dateFormatYYYYMMDD)
	}

	from, err := time.Parse(dateFormatYYYYMMDD, fromRaw)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid from date format, expected YYYY-MM-DD")
	}
	to, err := time.Parse(dateFormatYYYYMMDD, toRaw)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid to date format, expected YYYY-MM-DD")
	}
	from = timezone.DateOfUTC(from)
	to = timezone.DateOfUTC(to)
	if to.Before(from) {
		return time.Time{}, time.Time{}, errors.New("to must be after from")
	}
	return from, to, nil
}

func parseStatusDayDates(rawDates []string) ([]time.Time, error) {
	dates := make([]time.Time, 0, len(rawDates))
	for _, rawDate := range rawDates {
		date, err := time.Parse(dateFormatYYYYMMDD, rawDate)
		if err != nil {
			return nil, errors.New("invalid date format, expected YYYY-MM-DD")
		}
		dates = append(dates, timezone.DateOfUTC(date))
	}
	return dates, nil
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
	}
}

func containsDate(dates []time.Time, needle time.Time) bool {
	needle = timezone.DateOfUTC(needle)
	for _, date := range dates {
		if timezone.DateOfUTC(date).Equal(needle) {
			return true
		}
	}
	return false
}

func minDate(dates []time.Time) time.Time {
	min := dates[0]
	for _, date := range dates[1:] {
		if date.Before(min) {
			min = date
		}
	}
	return min
}

func maxDate(dates []time.Time) time.Time {
	max := dates[0]
	for _, date := range dates[1:] {
		if date.After(max) {
			max = date
		}
	}
	return max
}
