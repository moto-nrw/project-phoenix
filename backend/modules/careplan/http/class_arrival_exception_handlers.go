package careplanhttp

import (
	errors "errors"
	fmt "fmt"
	http "net/http"
	strings "strings"
	time "time"
	utf8 "unicode/utf8"

	chi "github.com/go-chi/chi/v5"
	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
	permissions "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	sessiontoken "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	securityruntime "github.com/moto-nrw/project-phoenix/modules/securityruntime"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

// classArrivalExceptionListDays is how far ahead the list looks by default.
const classArrivalExceptionListDays = 60

// ClassArrivalExceptionRequest is the body of PUT /students/class-arrival-exceptions/{schoolClass}/{date}.
type ClassArrivalExceptionRequest struct {
	ArrivalTime string  `json:"arrival_time"` // HH:MM
	Reason      *string `json:"reason,omitempty"`
}

// Bind implements render.Binder.
func (r *ClassArrivalExceptionRequest) Bind(_ *http.Request) error {
	if strings.TrimSpace(r.ArrivalTime) == "" {
		return errors.New("arrival_time is required")
	}
	if _, err := time.Parse("15:04", r.ArrivalTime); err != nil {
		return errors.New("invalid arrival_time format, expected HH:MM")
	}
	if r.Reason != nil && utf8.RuneCountInString(*r.Reason) > 255 {
		return errors.New("reason cannot exceed 255 characters")
	}
	return nil
}

// ClassArrivalExceptionResponse is one class-wide day exception on the wire.
type ClassArrivalExceptionResponse struct {
	SchoolClass string  `json:"school_class"`
	Date        string  `json:"date"`
	ArrivalTime string  `json:"arrival_time"`
	Reason      *string `json:"reason,omitempty"`
	CreatedAt   string  `json:"created_at"`
	// Origin is "ogs" or "school" (#2970): the OGS dialog says "eingetragen
	// von der Schule" for rows a Lehrkraft entered through moto schule.
	Origin string `json:"origin"`
}

// ClassArrivalExceptionListResponse is the GET payload.
type ClassArrivalExceptionListResponse struct {
	SchoolClass string                          `json:"school_class"`
	CanEdit     bool                            `json:"can_edit"`
	Exceptions  []ClassArrivalExceptionResponse `json:"exceptions"`
}

var classArrivalExceptionErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: careplan.ErrClassArrivalExceptionPastDate, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayArrivalExceptionPastDate)
	}},
	{Target: careplan.ErrClassArrivalExceptionWeekend, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayArrivalExceptionWeekend)
	}},
	{Target: careplan.ErrClassArrivalExceptionClassNotFound, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeClassdayArrivalExceptionClassNotFound)
	}},
	{Target: careplan.ErrClassArrivalExceptionNotFound, Render: common.ErrorNotFound},
}, common.ErrorInternalServer)

func mapClassArrivalException(row *careplan.ClassArrivalException) ClassArrivalExceptionResponse {
	return ClassArrivalExceptionResponse{
		SchoolClass: row.SchoolClass,
		Date:        row.Date.String(),
		ArrivalTime: row.ArrivalTime.Format("15:04"),
		Reason:      row.Reason,
		CreatedAt:   row.CreatedAt.Format(time.RFC3339),
		Origin:      classArrivalExceptionOrigin(row.Origin),
	}
}

// classArrivalExceptionOrigin reports a row without a recorded origin as an
// OGS exception, which is what every row written before the field existed is.
func classArrivalExceptionOrigin(origin string) string {
	if origin == "" {
		return careplan.ClassArrivalExceptionOriginOGS
	}
	return origin
}

// canEditClassArrivalExceptions applies operations.class_arrival_exception_editors:
// administrators always may, every staff member holding users:update may once
// the school opened it up.
func (rs *Resource) canEditClassArrivalExceptions(r *http.Request) (bool, error) {
	userPermissions := sessiontoken.PermissionsFromCtx(r.Context())
	if securityruntime.HasAdminWildcard(userPermissions) {
		return true, nil
	}
	if !securityruntime.HasPermission(permissions.UsersUpdate, userPermissions) {
		return false, nil
	}
	if rs.SettingsService == nil {
		return false, errors.New("settings service is not configured")
	}
	editors, err := rs.SettingsService.ResolveString(r.Context(), settingClassArrivalExceptionEditors)
	if err != nil {
		return false, fmt.Errorf("resolve class arrival exception editors: %w", err)
	}
	return editors == classArrivalExceptionEditorsAllStaff, nil
}

// requireClassArrivalExceptionEditor renders 403 unless the caller may write.
func (rs *Resource) requireClassArrivalExceptionEditor(w http.ResponseWriter, r *http.Request) bool {
	allowed, err := rs.canEditClassArrivalExceptions(r)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return false
	}
	if !allowed {
		renderError(w, r, common.ErrorForbiddenWithCode(
			errors.New("class arrival exceptions may only be set by administrators"),
			common.CodeClassdayArrivalExceptionEditorRequired,
		))
		return false
	}
	return true
}

// parseClassArrivalExceptionDate reads the {date} path segment.
func parseClassArrivalExceptionDate(w http.ResponseWriter, r *http.Request) (calendar.Date, bool) {
	date, err := calendar.ParseDate(chi.URLParam(r, "date"))
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date format, expected YYYY-MM-DD")))
		return "", false
	}
	return date, true
}

// getClassArrivalExceptions handles GET /students/class-arrival-exceptions/{schoolClass}.
// Optional from/to (YYYY-MM-DD) bound the window; it defaults to today plus
// the next 60 days.
func (rs *Resource) getClassArrivalExceptions(w http.ResponseWriter, r *http.Request) {
	schoolClass := strings.TrimSpace(chi.URLParam(r, "schoolClass"))
	from, to, err := classArrivalExceptionWindow(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rows, err := rs.ArrivalScheduleService.ListClassArrivalExceptions(r.Context(), schoolClass, from, to)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	canEdit, err := rs.canEditClassArrivalExceptions(r)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	resp := ClassArrivalExceptionListResponse{
		SchoolClass: schoolClass,
		CanEdit:     canEdit,
		Exceptions:  make([]ClassArrivalExceptionResponse, 0, len(rows)),
	}
	for _, row := range rows {
		resp.Exceptions = append(resp.Exceptions, mapClassArrivalException(row))
	}
	common.Respond(w, r, http.StatusOK, resp, "Class arrival exceptions retrieved successfully")
}

func classArrivalExceptionWindow(r *http.Request) (calendar.Date, calendar.Date, error) {
	from := calendar.TodayDate()
	to := from.AddDays(classArrivalExceptionListDays)
	if raw := r.URL.Query().Get("from"); raw != "" {
		parsed, err := calendar.ParseDate(raw)
		if err != nil {
			return "", "", errors.New("invalid from format, expected YYYY-MM-DD")
		}
		from = parsed
	}
	if raw := r.URL.Query().Get("to"); raw != "" {
		parsed, err := calendar.ParseDate(raw)
		if err != nil {
			return "", "", errors.New("invalid to format, expected YYYY-MM-DD")
		}
		to = parsed
	}
	return from, to, nil
}

// putClassArrivalException handles PUT /students/class-arrival-exceptions/{schoolClass}/{date}.
func (rs *Resource) putClassArrivalException(w http.ResponseWriter, r *http.Request) {
	if !rs.requireClassArrivalExceptionEditor(w, r) {
		return
	}
	date, ok := parseClassArrivalExceptionDate(w, r)
	if !ok {
		return
	}
	req := &ClassArrivalExceptionRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	staffID := int64(0)
	if staff, err := rs.getStaffIDFromJWT(r); err == nil {
		staffID = staff
	} else if !securityruntime.HasAdminWildcard(sessiontoken.PermissionsFromCtx(r.Context())) ||
		(!errors.Is(err, errPersonNotFoundForAccount) && !errors.Is(err, errUserNotStaff)) {
		renderError(w, r, common.ErrorForbidden(err))
		return
	}
	arrivalTime, _ := parseTimeOnly(req.ArrivalTime)

	row, err := rs.ArrivalScheduleService.UpsertClassArrivalException(r.Context(), careplan.ClassArrivalExceptionInput{
		SchoolClass: chi.URLParam(r, "schoolClass"),
		Date:        date,
		ArrivalTime: arrivalTime,
		Reason:      req.Reason,
	}, staffID)
	if err != nil {
		renderError(w, r, classArrivalExceptionErrorRenderer(err))
		return
	}
	// The roster and group views refetch on this event; a class-wide change
	// concerns every child of the class, so no student ID travels with it.
	tenant.RegisterAfterCommit(r.Context(), func() { rs.broadcastArrivalScheduleChanged(0) })
	common.Respond(w, r, http.StatusOK, mapClassArrivalException(row), "Class arrival exception saved successfully")
}

// deleteClassArrivalException handles DELETE /students/class-arrival-exceptions/{schoolClass}/{date}.
func (rs *Resource) deleteClassArrivalException(w http.ResponseWriter, r *http.Request) {
	if !rs.requireClassArrivalExceptionEditor(w, r) {
		return
	}
	date, ok := parseClassArrivalExceptionDate(w, r)
	if !ok {
		return
	}
	if err := rs.ArrivalScheduleService.DeleteClassArrivalException(r.Context(), chi.URLParam(r, "schoolClass"), date); err != nil {
		renderError(w, r, classArrivalExceptionErrorRenderer(err))
		return
	}
	tenant.RegisterAfterCommit(r.Context(), func() { rs.broadcastArrivalScheduleChanged(0) })
	w.WriteHeader(http.StatusNoContent)
}
