package studentpresencehttp

import (
	context "context"
	errors "errors"
	http "net/http"
	slices "slices"

	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
	sessiontoken "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

const maxStudentStatusDayRangeDays = 31

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

	userPermissions := sessiontoken.PermissionsFromCtx(r.Context())
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

	rows, err := rs.StudentStatusDayService.FindActiveByStudentAndDateRange(r.Context(), student.ID, slices.MinFunc(dates, calendar.Date.Compare), slices.MaxFunc(dates, calendar.Date.Compare))
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to fetch student status days", err))
		return
	}

	common.Respond(w, r, http.StatusCreated, studentpresence.NewStudentStatusDayResponses(rows), "Student status days created successfully")
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

	userPermissions := sessiontoken.PermissionsFromCtx(r.Context())
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

	userPermissions := sessiontoken.PermissionsFromCtx(r.Context())
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
		StudentService: newStatusDayStudents(rs.PeopleDirectory, func(ctx context.Context, student *peopleModule.StudentState, status string) bool {
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

func (rs *Resource) newStatusDayCreateWriteContext(r *http.Request, userPermissions []string, status string, dates []calendar.Date) studentpresence.StatusDayWriteContext {
	writeContext := rs.newStatusDayWriteContext(r, userPermissions)
	tenantID := tenant.FromContext(r.Context())
	actorAccountID := int64(sessiontoken.ClaimsFromCtx(r.Context()).ID)
	writeContext.AfterCreate = func(ctx context.Context, studentIDs []int64) error {
		return rs.NotifyAbsenceReported(ctx, tenantID, studentIDs, status, dates, false, actorAccountID)
	}
	return writeContext
}

func parseStatusDayDates(rawDates []string) ([]calendar.Date, error) {
	dates := make([]calendar.Date, 0, len(rawDates))
	for _, rawDate := range rawDates {
		date, err := calendar.ParseDate(rawDate)
		if err != nil {
			return nil, errors.New("invalid date format, expected YYYY-MM-DD")
		}
		dates = append(dates, date)
	}
	return dates, nil
}

func datesBetweenInclusive(from, to calendar.Date) []calendar.Date {
	dates := make([]calendar.Date, 0, from.DaysUntil(to)+1)
	for date := from; !date.After(to); date = date.AddDays(1) {
		dates = append(dates, date)
	}
	return dates
}

// statusDayConflictBody is the shared 409 body for single and bulk
// planned-status writes: the shared error envelope plus a capped conflict
// sample and the full total as extension members. The code lets the frontend
// word the refusal without reading the sentence (ADR 0006).
type statusDayConflictBody struct {
	*common.ErrResponse
	Conflicts     []studentpresence.StudentStatusDayResponse `json:"conflicts"`
	ConflictCount int                                        `json:"conflict_count"`
}

func statusDayConflictResponse(conflictErr *studentpresence.StudentStatusDayConflictError) *statusDayConflictBody {
	return &statusDayConflictBody{
		ErrResponse: &common.ErrResponse{
			Err:            conflictErr,
			HTTPStatusCode: http.StatusConflict,
			Detail:         "existing student status days were not overwritten",
			Code:           common.CodeStudentsStatusDayConflict,
		},
		Conflicts:     studentpresence.NewStudentStatusDayConflictResponses(conflictErr.SampleConflicts()),
		ConflictCount: conflictErr.ConflictTotal(),
	}
}

// bulkStatusDayDates parses the bulk range: two calendar days, in order, at
// most maxStudentStatusDayRangeDays apart, both included.
func bulkStatusDayDates(fromValue, toValue string) ([]calendar.Date, error) {
	from, err := calendar.ParseDate(fromValue)
	if err != nil {
		return nil, errors.New("invalid from date format, expected YYYY-MM-DD")
	}
	to, err := calendar.ParseDate(toValue)
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
		renderError(w, r, statusDayConflictResponse(conflictErr))
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
		renderError(w, r, common.ErrorConflictWithCode(err, common.CodeStudentsPartialAbsenceConflict))
		return
	}
	renderError(w, r, common.ErrorInternalServerWrap(failure, err))
}
