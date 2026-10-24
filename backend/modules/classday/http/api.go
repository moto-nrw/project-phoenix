// Package classdayhttp serves the read-only per-class day view for the
// Lehrkraft role (#1772): which students of an assigned school class stay in
// care on a given day, which go home, and how. Every route is gated on
// class_day:read, and the roster is additionally scoped to the caller's
// education.class_teachers assignments — deliberately NOT users:read, so
// holders never reach the tenant-wide student directory. The handlers know
// exactly one capability, the class-day projection's ClassDay seam.
package classdayhttp

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Resource wires the class-day endpoints.
type Resource struct {
	ClassDay classday.ClassDay
	// DayHints writes the day hints (#4795); the capability serves both
	// facades. Without it the day-hint routes answer "not configured".
	DayHints classday.DayHints
	logger   *slog.Logger
}

// NewResource creates the class-day resource over the projection's
// school-portal capability. A nil logger falls back to the default logger.
func NewResource(capability classday.ClassDay, logger *slog.Logger) *Resource {
	if logger == nil {
		logger = slog.Default()
	}
	hints, _ := capability.(classday.DayHints)
	return &Resource{ClassDay: capability, DayHints: hints, logger: logger}
}

// SchoolRouter returns the class-day surface gated to school-scope tokens.
// Since the cutover (#2207 PR 3) this is the ONLY mount: the tenant-portal
// twin under /api/class-day is gone, so a Lehrkraft reaches the view through
// moto schule and nowhere else.
func (rs *Resource) SchoolRouter(protectedSchoolRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	protectedSchoolRoutes(r, func(r chi.Router, withTx common.Middleware) {
		rs.registerRoutes(r, withTx)
	})

	return r
}

func (rs *Resource) registerRoutes(r chi.Router, withTx common.Middleware) {
	read := common.RequiresPermission(permissions.ClassDayRead)
	write := common.RequiresPermission(permissions.ClassDayArrivalExceptionWrite)

	r.With(read, withTx).Get("/classes", rs.getMyClasses)
	r.With(read, withTx).Get("/", rs.getClassDay)

	// Class-wide arrival day exceptions (#2970): the list is readable with
	// class_day:read, the writes and the preset lookup need the write
	// permission plus the school's setting (checked in the handler).
	r.With(read, withTx).Get("/arrival-exceptions", rs.getArrivalExceptions)
	r.With(write, withTx).Get("/arrival-exceptions/block-start", rs.getArrivalExceptionBlockStart)
	r.With(write, withTx).Put("/arrival-exceptions/{schoolClass}/{date}", rs.putArrivalException)
	r.With(write, withTx).Delete("/arrival-exceptions/{schoolClass}/{date}", rs.deleteArrivalException)

	// Class trips and day outings (#4799): the permission plus the school's
	// setting (checked in the handler).
	r.With(common.RequiresPermission(permissions.ClassDayClassTripWrite), withTx).Post("/class-trips/{schoolClass}", rs.postClassTrip)
	// Day hints for the OGS (#4795): the Lehrkraft writes today's hint for an
	// assigned class or one of its children; the setting, the class and the
	// day are checked behind the permission.
	hintWrite := common.RequiresPermission(permissions.ClassDayDayHintWrite)
	r.With(hintWrite, withTx).Put("/day-hints/{schoolClass}/{date}", rs.putDayHint)
	r.With(hintWrite, withTx).Delete("/day-hints/{schoolClass}/{date}", rs.deleteDayHint)
	r.With(hintWrite, withTx).Put("/day-hints/{schoolClass}/{date}/students/{studentId}", rs.putDayHint)
	r.With(hintWrite, withTx).Delete("/day-hints/{schoolClass}/{date}/students/{studentId}", rs.deleteDayHint)
}

// ClassesResponse lists the caller's assigned school classes.
type ClassesResponse struct {
	Classes []string `json:"classes"`
	// CanWriteArrivalException is true when the caller holds
	// class_day:arrival_exception_write AND the school opened moto schule
	// for it (#2970); the class view shows its action only then.
	CanWriteArrivalException bool `json:"can_write_arrival_exception"`
	// CanReportClassTrip is true when the caller holds
	// class_day:class_trip_write AND the school opened moto schule for it
	// (#4799).
	CanReportClassTrip bool `json:"can_report_class_trip"`
	// WeekendFollowsFriday is true when the school runs Saturday and Sunday
	// on Friday's plan (operations.weekend_follows_friday, #3921). The class
	// views then load the weekend like a school day instead of skipping it.
	WeekendFollowsFriday bool `json:"weekend_follows_friday"`
}

// getMyClasses returns the school classes assigned to the caller.
func (rs *Resource) getMyClasses(w http.ResponseWriter, r *http.Request) {
	classes, err := rs.ClassDay.GetMySchoolClasses(r.Context())
	if err != nil {
		rs.logger.Error("class day: load assigned classes failed",
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	resp := ClassesResponse{Classes: classes}
	// Without the write seam the flag stays false instead of failing the
	// list: the classes are the answer, the flag is an extra.
	resp.CanWriteArrivalException, err = rs.canWriteArrivalException(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	resp.CanReportClassTrip, err = rs.canReportClassTrip(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	resp.WeekendFollowsFriday, err = calendar.WeekendFollowsFriday(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, resp, "Classes retrieved successfully")
}

// getClassDay returns the day view for one assigned class.
// Query: class (optional when exactly the first assignment should be shown),
// date (YYYY-MM-DD, default today).
func (rs *Resource) getClassDay(w http.ResponseWriter, r *http.Request) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return
	}

	date := calendar.TodayDate()
	if raw := strings.TrimSpace(r.URL.Query().Get("date")); raw != "" {
		parsed, err := calendar.ParseDate(raw)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("ungültiges Datum, erwartet JJJJ-MM-TT")))
			return
		}
		date = parsed
	}

	class, ok := rs.requireAssignedClass(w, r, r.URL.Query().Get("class"))
	if !ok {
		return
	}

	actor := classday.Actor{AccountID: int64(claims.ID), Roles: strings.Join(claims.Roles, ",")}
	report, err := rs.ClassDay.DayReport(r.Context(), class, classday.Date(date.String()), actor)
	if err != nil {
		if errors.Is(err, classday.ErrInvalidReportFilter) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		rs.logger.Error("class day: report failed",
			"school_class", class,
			"date", date.String(),
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	// The projection says whether the day and the school allow a day hint;
	// the token decides whether this caller may write one (#4795).
	report.CanWriteDayHints = report.CanWriteDayHints &&
		common.HasPermission(permissions.ClassDayDayHintWrite, sessiontoken.PermissionsFromCtx(r.Context()))

	common.Respond(w, r, http.StatusOK, report, "Class day retrieved successfully")
}

// requireAssignedClass resolves the class against the caller's assignments
// and renders 403 when it is not one of them.
func (rs *Resource) requireAssignedClass(w http.ResponseWriter, r *http.Request, requested string) (string, bool) {
	class, err := rs.ClassDay.ResolveClass(r.Context(), requested)
	if err != nil {
		if errors.Is(err, classday.ErrClassNotAssigned) {
			common.RenderError(w, r, common.ErrorForbidden(classday.ErrClassNotAssigned))
			return "", false
		}
		rs.logger.Error("class day: load assigned classes failed",
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return "", false
	}
	return class, true
}
