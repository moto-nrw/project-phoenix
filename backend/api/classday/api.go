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
	"github.com/moto-nrw/project-phoenix/auth/authorize"
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
}

// NewResource creates the class-day resource.
func NewResource(
	reportService enrollmentSvc.ReportService,
	userContextService usercontextSvc.UserContextService,
	db *bun.DB,
	logger *slog.Logger,
) *Resource {
	return &Resource{
		ReportService:      reportService,
		UserContextService: userContextService,
		db:                 db,
		logger:             logger,
	}
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
	r.With(authorize.RequiresPermission(permissions.ClassDayRead), withTx).Get("/classes", rs.getMyClasses)
	r.With(authorize.RequiresPermission(permissions.ClassDayRead), withTx).Get("/", rs.getClassDay)
}

// ClassesResponse lists the caller's assigned school classes.
type ClassesResponse struct {
	Classes []string `json:"classes"`
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
	common.Respond(w, r, http.StatusOK, ClassesResponse{Classes: classes}, "Classes retrieved successfully")
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
