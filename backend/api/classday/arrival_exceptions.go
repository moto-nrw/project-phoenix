package classday

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// Class-wide arrival day exceptions through "moto schule" (#2970): the same
// rows the OGS sets in the Kindersuche (#2962), entered by the Lehrkraft of
// the class. Every write passes four gates in this order — school scope
// (SchoolMiddleware), the class_day:arrival_exception_write permission, the
// school's setting operations.school_portal_write_scope, and the caller's
// education.class_teachers assignment for the class — before the shared
// service refuses past dates and weekends. The handlers know one seam, the
// class-day view's ClassDayArrivalExceptionService; there is no second
// write path.

// arrivalExceptionListDays is how far ahead the list looks by default,
// matching the OGS dialog.
const arrivalExceptionListDays = 60

// ErrSchoolWriteDisabled is returned when the school keeps moto schule
// read-only (operations.school_portal_write_scope = none).
var ErrSchoolWriteDisabled = errors.New("Die OGS hat das Eintragen für die Schule nicht freigegeben") //nolint:staticcheck // ST1005: user-facing German message

// ErrStaffRecordRequired is returned when the caller has no users.staff row
// to attribute the entry to.
var ErrStaffRecordRequired = errors.New("Zu Ihrem Konto gibt es keinen Mitarbeiterdatensatz") //nolint:staticcheck // ST1005: user-facing German message

// Option tunes optional wiring of the class-day resource.
type Option func(*Resource)

// WithArrivalExceptions enables the class-wide arrival exception routes.
// Without it they exist but answer 500 "not configured", so the route table
// stays stable across wirings.
func WithArrivalExceptions(svc enrollmentSvc.ClassDayArrivalExceptionService) Option {
	return func(rs *Resource) { rs.arrivalExceptions = svc }
}

// ArrivalExceptionRequest is the body of PUT /school/class-day/arrival-exceptions/{schoolClass}/{date}.
// Same shape as the OGS request (api/students ClassArrivalExceptionRequest).
type ArrivalExceptionRequest struct {
	ArrivalTime string  `json:"arrival_time"` // HH:MM
	Reason      *string `json:"reason,omitempty"`
}

// Bind implements render.Binder.
func (r *ArrivalExceptionRequest) Bind(_ *http.Request) error {
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

// ArrivalExceptionResponse is one class-wide day exception on the wire.
type ArrivalExceptionResponse struct {
	SchoolClass string  `json:"school_class"`
	Date        string  `json:"date"`
	ArrivalTime string  `json:"arrival_time"`
	Reason      *string `json:"reason,omitempty"`
	CreatedAt   string  `json:"created_at"`
	// Origin is "ogs" or "school": who entered the row.
	Origin string `json:"origin"`
}

// ArrivalExceptionListResponse is the GET payload.
type ArrivalExceptionListResponse struct {
	SchoolClass string                     `json:"school_class"`
	CanEdit     bool                       `json:"can_edit"`
	Exceptions  []ArrivalExceptionResponse `json:"exceptions"`
}

// ArrivalExceptionBlockStartResponse answers the preset lookup; Start is ""
// when the date has no block for the class.
type ArrivalExceptionBlockStartResponse struct {
	SchoolClass string `json:"school_class"`
	Date        string `json:"date"`
	Start       string `json:"start"`
}

var arrivalExceptionErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: enrollmentSvc.ErrClassDayArrivalExceptionPastDate, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, "class_arrival_exception_past_date")
	}},
	{Target: enrollmentSvc.ErrClassDayArrivalExceptionWeekend, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, "class_arrival_exception_weekend")
	}},
	{Target: enrollmentSvc.ErrClassDayArrivalExceptionClassNotFound, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, "class_arrival_exception_class_not_found")
	}},
	{Target: enrollmentSvc.ErrClassDayArrivalExceptionNotFound, Render: common.ErrorNotFound},
}, common.ErrorInternalServer)

func mapArrivalException(entry enrollmentSvc.ClassDayArrivalExceptionEntry) ArrivalExceptionResponse {
	return ArrivalExceptionResponse{
		SchoolClass: entry.SchoolClass,
		Date:        entry.Date,
		ArrivalTime: entry.ArrivalTime,
		Reason:      entry.Reason,
		CreatedAt:   entry.CreatedAt,
		Origin:      entry.Origin,
	}
}

// canWriteArrivalException is the verdict the classes list and the list
// response carry: permission in the token AND the setting opened it up.
func (rs *Resource) canWriteArrivalException(r *http.Request) (bool, error) {
	if rs.arrivalExceptions == nil {
		return false, nil
	}
	if !common.HasPermission(permissions.ClassDayArrivalExceptionWrite, jwt.PermissionsFromCtx(r.Context())) {
		return false, nil
	}
	return rs.arrivalExceptions.SchoolMayWrite(r.Context())
}

// requireConfigured renders 500 when the write seam is not wired.
func (rs *Resource) requireConfigured(w http.ResponseWriter, r *http.Request) bool {
	if rs.arrivalExceptions == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("arrival exceptions are not configured")))
		return false
	}
	return true
}

// requireSchoolWrite renders 403 school_write_disabled unless the setting
// opened the write. The permission itself is checked by the route.
func (rs *Resource) requireSchoolWrite(w http.ResponseWriter, r *http.Request) bool {
	allowed, err := rs.arrivalExceptions.SchoolMayWrite(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return false
	}
	if !allowed {
		common.RenderError(w, r, common.ErrorForbiddenWithCode(ErrSchoolWriteDisabled, "school_write_disabled"))
		return false
	}
	return true
}

// requireAssignedClass resolves the class against the caller's assignments
// and renders 403 when it is not one of them.
func (rs *Resource) requireAssignedClass(w http.ResponseWriter, r *http.Request, requested string) (string, bool) {
	assigned, err := rs.UserContextService.GetMySchoolClasses(r.Context())
	if err != nil {
		rs.getLogger().Error("class day: load assigned classes failed",
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return "", false
	}
	class, err := resolveRequestedClass(requested, assigned)
	if err != nil {
		common.RenderError(w, r, common.ErrorForbidden(ErrClassNotAssigned))
		return "", false
	}
	return class, true
}

// requireArrivalExceptionWrite runs the write gates in the documented order:
// wiring, setting, then class assignment. Returns the resolved class.
func (rs *Resource) requireArrivalExceptionWrite(w http.ResponseWriter, r *http.Request, requested string) (string, bool) {
	if !rs.requireConfigured(w, r) || !rs.requireSchoolWrite(w, r) {
		return "", false
	}
	return rs.requireAssignedClass(w, r, requested)
}

func parseArrivalExceptionDate(w http.ResponseWriter, r *http.Request, raw string) (timezone.Date, bool) {
	date, err := timezone.ParseDate(strings.TrimSpace(raw))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("ungültiges Datum, erwartet JJJJ-MM-TT")))
		return "", false
	}
	return date, true
}

// arrivalExceptionWindow reads the optional from/to query bounds; the
// default is today plus the next 60 days, like the OGS list.
func arrivalExceptionWindow(r *http.Request) (timezone.Date, timezone.Date, error) {
	from := timezone.TodayDate()
	to := from.AddDays(arrivalExceptionListDays)
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		parsed, err := timezone.ParseDate(raw)
		if err != nil {
			return "", "", errors.New("ungültiges from, erwartet JJJJ-MM-TT")
		}
		from = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		parsed, err := timezone.ParseDate(raw)
		if err != nil {
			return "", "", errors.New("ungültiges to, erwartet JJJJ-MM-TT")
		}
		to = parsed
	}
	return from, to, nil
}

// getArrivalExceptions handles GET /school/class-day/arrival-exceptions?class=4a:
// the exceptions of one assigned class from today on. Reading needs only
// class_day:read — a Lehrkraft sees what the OGS entered even when the
// school may not write.
func (rs *Resource) getArrivalExceptions(w http.ResponseWriter, r *http.Request) {
	if !rs.requireConfigured(w, r) {
		return
	}
	class, ok := rs.requireAssignedClass(w, r, r.URL.Query().Get("class"))
	if !ok {
		return
	}
	from, to, err := arrivalExceptionWindow(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	entries, err := rs.arrivalExceptions.List(r.Context(), class, from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	canEdit, err := rs.canWriteArrivalException(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	resp := ArrivalExceptionListResponse{
		SchoolClass: class,
		CanEdit:     canEdit,
		Exceptions:  make([]ArrivalExceptionResponse, 0, len(entries)),
	}
	for _, entry := range entries {
		resp.Exceptions = append(resp.Exceptions, mapArrivalException(entry))
	}
	common.Respond(w, r, http.StatusOK, resp, "Class arrival exceptions retrieved successfully")
}

// putArrivalException handles PUT /school/class-day/arrival-exceptions/{schoolClass}/{date}.
func (rs *Resource) putArrivalException(w http.ResponseWriter, r *http.Request) {
	class, ok := rs.requireArrivalExceptionWrite(w, r, chi.URLParam(r, "schoolClass"))
	if !ok {
		return
	}
	date, ok := parseArrivalExceptionDate(w, r, chi.URLParam(r, "date"))
	if !ok {
		return
	}
	req := &ArrivalExceptionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	// created_by is the Lehrkraft's staff row (every school-portal account
	// has one, EnsureSchoolIdentity); without it the entry would be
	// attributed to nobody, so the write is refused.
	staff, err := rs.UserContextService.GetCurrentStaff(r.Context())
	if err != nil || staff == nil {
		common.RenderError(w, r, common.ErrorForbidden(ErrStaffRecordRequired))
		return
	}
	arrivalTime, _ := time.Parse("15:04", req.ArrivalTime)

	entry, err := rs.arrivalExceptions.Set(r.Context(), enrollmentSvc.ClassDayArrivalExceptionWrite{
		SchoolClass: class,
		Date:        date,
		ArrivalTime: arrivalTime,
		Reason:      req.Reason,
		CreatedBy:   staff.ID,
	})
	if err != nil {
		common.RenderError(w, r, arrivalExceptionErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, mapArrivalException(*entry), "Class arrival exception saved successfully")
}

// deleteArrivalException handles DELETE /school/class-day/arrival-exceptions/{schoolClass}/{date}.
func (rs *Resource) deleteArrivalException(w http.ResponseWriter, r *http.Request) {
	class, ok := rs.requireArrivalExceptionWrite(w, r, chi.URLParam(r, "schoolClass"))
	if !ok {
		return
	}
	date, ok := parseArrivalExceptionDate(w, r, chi.URLParam(r, "date"))
	if !ok {
		return
	}
	if err := rs.arrivalExceptions.Remove(r.Context(), class, date); err != nil {
		common.RenderError(w, r, arrivalExceptionErrorRenderer(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getArrivalExceptionBlockStart handles GET /school/class-day/arrival-exceptions/block-start?class=4a&date=YYYY-MM-DD:
// the "Unterricht fällt aus" preset. Same gates as a write — the value is
// only useful for one.
func (rs *Resource) getArrivalExceptionBlockStart(w http.ResponseWriter, r *http.Request) {
	class, ok := rs.requireArrivalExceptionWrite(w, r, r.URL.Query().Get("class"))
	if !ok {
		return
	}
	date, ok := parseArrivalExceptionDate(w, r, r.URL.Query().Get("date"))
	if !ok {
		return
	}
	start, err := rs.arrivalExceptions.EarliestBlockStart(r.Context(), class, date)
	if err != nil {
		rs.getLogger().Error("class day: block start lookup failed",
			"school_class", class,
			"date", date.String(),
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, ArrivalExceptionBlockStartResponse{SchoolClass: class, Date: date.String(), Start: start}, "Block start retrieved successfully")
}
