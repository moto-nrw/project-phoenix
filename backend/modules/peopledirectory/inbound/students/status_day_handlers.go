package students

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const maxStudentStatusDayRangeDays = 31

func (rs *Resource) getStudentStatusDays(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	// Whoever may WRITE this child's absences may also read them: the planning
	// dialog refuses to save until it has checked the existing status days, so
	// a caller authorized only by the open-care absence gate (#2232) would see
	// the actions and then be unable to use them. The payload is exactly the
	// absence data that gate covers — no Stammdaten ride along.
	if !rs.checkStudentReadAccess(r, student) && !rs.checkStudentAbsenceWriteAccess(r, student) {
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
	authorized, authErr := rs.canManageStudentStatus(r.Context(), userPermissions, student, req.Status)
	if !authorized {
		renderError(w, r, common.ErrorForbidden(authErr))
		return
	}

	dates, err := parseStatusDayDates(req.Dates)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if err := rs.StudentStatusDayService.CreateForDates(r.Context(), rs.newStatusDayCreateWriteContext(r, userPermissions, req.Status, dates), student.ID, req.Status, req.Reason, dates); err != nil {
		rs.renderStatusDayCreateError(w, r, err, false, "failed to create student status days")
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

	dates, err := bulkStatusDayDates(req.From, req.To)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	userPermissions := jwt.PermissionsFromCtx(r.Context())
	if err := rs.StudentStatusDayService.BulkCreateForDates(r.Context(), rs.newStatusDayCreateWriteContext(r, userPermissions, req.Status, dates), req.StudentIDs, req.Status, req.Reason, dates); err != nil {
		rs.renderStatusDayCreateError(w, r, err, true, "failed to bulk create student status days")
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
	authorized, authErr := rs.canManageStudentAbsence(r.Context(), userPermissions, student)
	if !authorized {
		renderError(w, r, common.ErrorForbidden(authErr))
		return
	}

	if err := rs.StudentStatusDayService.DeleteStatusDay(r.Context(), rs.newStatusDayWriteContext(r, userPermissions), statusDayID, student.ID); err != nil {
		if common.IsNotFound(err) {
			renderError(w, r, common.ErrorNotFound(errors.New("student status day not found")))
			return
		}
		if errors.Is(err, studentpresence.ErrStudentStatusDayReassigned) {
			renderError(w, r, common.ErrorForbidden(err))
			return
		}
		renderError(w, r, common.ErrorInternalServerWrap("failed to delete student status day", err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]bool{"deleted": true}, "Student status day deleted successfully")
}

// newStatusDayWriteContext bundles the collaborators the status-day write
// service needs, keeping the JWT-permission decision (canManageStudentAbsence)
// and the SSE fan-out at the HTTP boundary.
func (rs *Resource) newStatusDayWriteContext(r *http.Request, userPermissions []string) studentpresence.StatusDayWriteContext {
	tenantID := tenant.FromContext(r.Context())
	return studentpresence.StatusDayWriteContext{
		TenantID: tenantID,
		StudentService: newStatusDayStudents(rs.PeopleDirectory, func(ctx context.Context, student *Student, status string) bool {
			ok, _ := rs.canManageStudentStatus(ctx, userPermissions, student, status)
			return ok
		}),
		AfterCommit: func(studentID int64) {
			rs.broadcastStudentUpdated(tenantID, studentID)
			// Also wake the child's guardians so an open parents-app tab reflects
			// the new/cleared absence (today_absent → the "Heute" pickup tile)
			// live; the tenant-wide student_updated above never reaches the parent
			// SSE stream (#1725).
			rs.wakeChildGuardians(tenantID, studentID)
		},
	}
}

func (rs *Resource) newStatusDayCreateWriteContext(r *http.Request, userPermissions []string, status string, dates []timezone.Date) studentpresence.StatusDayWriteContext {
	writeContext := rs.newStatusDayWriteContext(r, userPermissions)
	tenantID := tenant.FromContext(r.Context())
	actorAccountID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	writeContext.AfterCreate = func(ctx context.Context, studentIDs []int64) error {
		return rs.notifyAbsenceReported(ctx, tenantID, studentIDs, status, dates, false, actorAccountID)
	}
	return writeContext
}

func parseStatusDayRange(r *http.Request) (timezone.Date, timezone.Date, error) {
	return parseStatusDayRangeAt(r, timezone.TodayDate())
}

func parseStatusDayRangeAt(r *http.Request, today timezone.Date) (timezone.Date, timezone.Date, error) {
	fromRaw := r.URL.Query().Get("from")
	toRaw := r.URL.Query().Get("to")

	from := today
	// Two calendar months ahead, mirroring time.Time.AddDate(0, 2, 0).
	to := timezone.NewDate(today.Year(), today.Month()+2, today.Day())
	var err error
	if fromRaw != "" {
		if from, err = timezone.ParseDate(fromRaw); err != nil {
			return timezone.Date(""), timezone.Date(""), errors.New("invalid from date format, expected YYYY-MM-DD")
		}
	}
	if toRaw != "" {
		if to, err = timezone.ParseDate(toRaw); err != nil {
			return timezone.Date(""), timezone.Date(""), errors.New("invalid to date format, expected YYYY-MM-DD")
		}
	}
	if to.Before(from) {
		return timezone.Date(""), timezone.Date(""), errors.New("to must be after from")
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

// statusDayConflictResponse builds the shared 409 body for single and bulk
// planned-status writes: a capped conflict sample plus the full total.
func statusDayConflictResponse(conflictErr *studentpresence.StudentStatusDayConflictError) map[string]any {
	return map[string]any{
		"status":         "error",
		"error":          "existing student status days were not overwritten",
		"conflicts":      newStudentStatusDayConflictResponses(conflictErr.SampleConflicts()),
		"conflict_count": conflictErr.ConflictTotal(),
	}
}

// bulkStatusDayDates parses the bulk range: two calendar days, in order, at
// most maxStudentStatusDayRangeDays apart, both included.
func bulkStatusDayDates(fromValue, toValue string) ([]timezone.Date, error) {
	from, err := timezone.ParseDate(fromValue)
	if err != nil {
		return nil, errors.New("invalid from date format, expected YYYY-MM-DD")
	}
	to, err := timezone.ParseDate(toValue)
	if err != nil {
		return nil, errors.New("invalid to date format, expected YYYY-MM-DD")
	}
	if to.Before(from) {
		return nil, errors.New("to must be after from")
	}
	if to.After(from.AddDays(maxStudentStatusDayRangeDays - 1)) {
		return nil, errors.New("date range cannot exceed 31 days")
	}
	return datesBetweenInclusive(from, to), nil
}

// renderStatusDayCreateError maps a status-day write failure to its response.
// rollbackOnRefusal fails the outer TenantTxMiddleware transaction closed for
// the non-5xx refusals, which would otherwise commit a partial nested write.
func (rs *Resource) renderStatusDayCreateError(w http.ResponseWriter, r *http.Request, err error, rollbackOnRefusal bool, failure string) {
	refuse := func() {
		if rollbackOnRefusal {
			tenant.MarkRollback(r.Context())
		}
	}
	var conflictErr *studentpresence.StudentStatusDayConflictError
	if errors.As(err, &conflictErr) {
		refuse()
		common.RespondWithJSON(
			w,
			r,
			http.StatusConflict,
			statusDayConflictResponse(conflictErr),
		)
		return
	}
	if errors.Is(err, studentpresence.ErrStudentStatusDayReassigned) {
		refuse()
		renderError(w, r, common.ErrorForbidden(err))
		return
	}
	if errors.Is(err, studentpresence.ErrStudentStatusDayPartialAbsenceConflict) {
		// Stable code so the frontend can show a clear message instead of
		// parsing this as an empty StudentStatusDayConflictError sample.
		refuse()
		renderError(w, r, common.ErrorConflictWithCode(err, "partial_absence_conflict"))
		return
	}
	renderError(w, r, common.ErrorInternalServerWrap(failure, err))
}
