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
	// SchoolDay serves the school-wide day overview (#4802); the capability
	// serves both facades. Without it the route answers "not configured".
	SchoolDay classday.SchoolDay
	// AbsenceReports enters and withdraws the Schulverwaltung's reports
	// (#4791); the capability serves both facades. Without it the routes
	// answer "not configured".
	AbsenceReports classday.AbsenceReports
	logger         *slog.Logger
}

// NewResource creates the class-day resource over the projection's
// school-portal capability. A nil logger falls back to the default logger.
func NewResource(capability classday.ClassDay, logger *slog.Logger) *Resource {
	if logger == nil {
		logger = slog.Default()
	}
	hints, _ := capability.(classday.DayHints)
	schoolDay, _ := capability.(classday.SchoolDay)
	reports, _ := capability.(classday.AbsenceReports)
	return &Resource{ClassDay: capability, DayHints: hints, SchoolDay: schoolDay, AbsenceReports: reports, logger: logger}
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
	// School-wide day overview (#4802): every class at once, only for the
	// holders of class_day:all_classes (Schulverwaltung).
	r.With(read, common.RequiresPermission(permissions.ClassDayAllClasses), withTx).Get("/school-day", rs.getSchoolDay)

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
	// Morning attendance (#4790): the Lehrkraft marks today's school morning
	// of an assigned class; the setting and the day are checked behind it.
	morningWrite := common.RequiresPermission(permissions.ClassDayMorningAttendanceWrite)
	r.With(morningWrite, withTx).Put("/morning-attendance/{schoolClass}/{date}", rs.putMorningAttendance)

	// Homework (#4796): the Lehrkraft enters it for an assigned class and
	// changes or deletes her own entries; the OGS feedback is in the report.
	homeworkWrite := common.RequiresPermission(permissions.ClassDayHomeworkWrite)
	r.With(homeworkWrite, withTx).Post("/homework/{schoolClass}", rs.postHomework)
	r.With(homeworkWrite, withTx).Put("/homework/{schoolClass}/{homeworkId}", rs.putHomework)
	r.With(homeworkWrite, withTx).Delete("/homework/{schoolClass}/{homeworkId}", rs.deleteHomework)
	// Class handover (#4794): the Lehrkraft hands today's class over to the
	// OGS or takes it back; the write scope and the day are checked behind it.
	handoverWrite := common.RequiresPermission(permissions.ClassDayHandoverWrite)
	r.With(handoverWrite, withTx).Put("/handover/{schoolClass}/{date}", rs.putClassHandover)
	r.With(handoverWrite, withTx).Delete("/handover/{schoolClass}/{date}", rs.deleteClassHandover)
	// Absence reports (#4791): the Schulverwaltung enters sick or excused
	// for a child missing without a report and takes it back; the day, the
	// roster and the missing report are checked behind the permission.
	reportWrite := common.RequiresPermission(permissions.ClassDayAbsenceReportWrite)
	r.With(reportWrite, withTx).Put("/absence-reports/{schoolClass}/{date}/students/{studentId}", rs.putAbsenceReport)
	r.With(reportWrite, withTx).Delete("/absence-reports/{schoolClass}/{date}/students/{studentId}", rs.deleteAbsenceReport)
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
	// CanViewSchoolDay is true when the caller holds class_day:all_classes
	// (#4802): the start page then loads the school-wide day overview once
	// instead of one class report per class.
	CanViewSchoolDay bool `json:"can_view_school_day"`
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
	resp := ClassesResponse{
		Classes:          classes,
		CanViewSchoolDay: rs.SchoolDay != nil && common.HasPermission(permissions.ClassDayAllClasses, sessiontoken.PermissionsFromCtx(r.Context())),
	}
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

	date, ok := dateQueryOrToday(w, r)
	if !ok {
		return
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
	// The projection says whether the day and the school allow the morning
	// attendance; the token decides whether this caller may write it.
	granted := sessiontoken.PermissionsFromCtx(r.Context())
	report.CanRecordMorningAttendance = report.CanRecordMorningAttendance &&
		common.HasPermission(permissions.ClassDayMorningAttendanceWrite, granted)
	report.CanHandOver = report.CanHandOver && common.HasPermission(permissions.ClassDayHandoverWrite, granted)
	// Homework needs only the permission: the class is assigned.
	report.CanWriteHomework = common.HasPermission(permissions.ClassDayHomeworkWrite, granted)
	report.CanReportAbsences = report.CanReportAbsences && rs.AbsenceReports != nil &&
		common.HasPermission(permissions.ClassDayAbsenceReportWrite, granted)

	common.Respond(w, r, http.StatusOK, report, "Class day retrieved successfully")
}

// dateQueryOrToday reads the optional ?date=YYYY-MM-DD, today without it,
// and renders 400 for a broken date.
func dateQueryOrToday(w http.ResponseWriter, r *http.Request) (calendar.Date, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("date"))
	if raw == "" {
		return calendar.TodayDate(), true
	}
	return parseArrivalExceptionDate(w, r, raw)
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
