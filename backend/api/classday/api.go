// Package classday serves the read-only per-class day view for the
// Lehrkraft role (#1772): which students of an assigned school class stay in
// care on a given day, which go home, and how. Every route is gated on
// class_day:read, and the roster is additionally scoped to the caller's
// education.class_teachers assignments — deliberately NOT users:read, so
// holders never reach the tenant-wide student directory.
package classday

import (
	"cmp"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	usercontextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/uptrace/bun"
)

// ErrClassNotAssigned is returned when the requested class is not among the
// caller's education.class_teachers assignments.
var ErrClassNotAssigned = errors.New("Diese Klasse ist Ihnen nicht zugewiesen") //nolint:staticcheck // ST1005: user-facing German message

// Resource wires the class-day endpoints.
type Resource struct {
	ReportService      enrollmentSvc.ReportService
	UserContextService usercontextSvc.UserContextService
	db                 *bun.DB
	logger             *slog.Logger
	// arrivalExceptions is the one write seam of moto schule (#2970); see
	// arrival_exceptions.go. Nil leaves those routes answering 500.
	arrivalExceptions enrollmentSvc.ClassDayArrivalExceptionService
}

// NewResource creates the class-day resource. Options add the optional
// wiring (WithArrivalExceptions); without them those routes answer 500.
func NewResource(
	reportService enrollmentSvc.ReportService,
	userContextService usercontextSvc.UserContextService,
	db *bun.DB,
	logger *slog.Logger,
	opts ...Option,
) *Resource {
	rs := &Resource{
		ReportService:      reportService,
		UserContextService: userContextService,
		db:                 db,
		logger:             logger,
	}
	for _, opt := range opts {
		opt(rs)
	}
	return rs
}

func (rs *Resource) getLogger() *slog.Logger {
	return cmp.Or(rs.logger, slog.Default())
}

// SchoolRouter returns the class-day surface gated to school-scope tokens.
// Since the cutover (#2207 PR 3) this is the ONLY mount: the tenant-portal
// twin under /api/class-day is gone, so a Lehrkraft reaches the view through
// moto schule and nowhere else.
func (rs *Resource) SchoolRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedSchoolGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
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
}

// ClassesResponse lists the caller's assigned school classes.
type ClassesResponse struct {
	Classes []string `json:"classes"`
	// CanWriteArrivalException is true when the caller holds
	// class_day:arrival_exception_write AND the school opened moto schule
	// for it (#2970); the class view shows its action only then.
	CanWriteArrivalException bool `json:"can_write_arrival_exception"`
}

// getMyClasses returns the school classes assigned to the caller.
func (rs *Resource) getMyClasses(w http.ResponseWriter, r *http.Request) {
	classes, err := rs.UserContextService.GetMySchoolClasses(r.Context())
	if err != nil {
		rs.getLogger().Error("class day: load assigned classes failed",
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
	common.Respond(w, r, http.StatusOK, resp, "Classes retrieved successfully")
}

// resolveRequestedClass picks the class to show: the ?class= parameter when
// it is one of the caller's assignments (normalized comparison), otherwise
// the first assigned class when no parameter was sent.
func resolveRequestedClass(requested string, assigned []string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		if len(assigned) == 0 {
			return "", ErrClassNotAssigned
		}
		return assigned[0], nil
	}
	for _, class := range assigned {
		if schoolclass.Normalize(class) == schoolclass.Normalize(requested) {
			return class, nil
		}
	}
	return "", ErrClassNotAssigned
}

// getClassDay returns the day view for one assigned class.
// Query: class (optional when exactly the first assignment should be shown),
// date (YYYY-MM-DD, default today).
func (rs *Resource) getClassDay(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return
	}

	date := timezone.TodayDate()
	if raw := strings.TrimSpace(r.URL.Query().Get("date")); raw != "" {
		parsed, err := timezone.ParseDate(raw)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("ungültiges Datum, erwartet JJJJ-MM-TT")))
			return
		}
		date = parsed
	}

	assigned, err := rs.UserContextService.GetMySchoolClasses(r.Context())
	if err != nil {
		rs.getLogger().Error("class day: load assigned classes failed",
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	class, err := resolveRequestedClass(r.URL.Query().Get("class"), assigned)
	if err != nil {
		common.RenderError(w, r, common.ErrorForbidden(ErrClassNotAssigned))
		return
	}

	report, err := rs.ReportService.ClassDay(r.Context(), class, date, int64(claims.ID), strings.Join(claims.Roles, ","))
	if err != nil {
		if errors.Is(err, enrollmentSvc.ErrReportInvalidFilter) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		rs.getLogger().Error("class day: report failed",
			"school_class", class,
			"date", date.String(),
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, report, "Class day retrieved successfully")
}
