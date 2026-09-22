// Package statisticshttp exposes the Statistik report (#2606): attendance and
// absence quotas per child, group and period plus room utilization, as JSON
// and as PDF / Excel / Word export through the shared listexport pipeline.
package statisticshttp

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/uptrace/bun"
)

// Resource is the statistics API resource.
type Resource struct {
	Service    studentpresence.StatisticsReports
	ListExport *listexport.RendererService
	DB         *bun.DB
	Logger     *slog.Logger
}

// NewResource creates the statistics resource.
func NewResource(service studentpresence.StatisticsReports, listExport *listexport.RendererService, db *bun.DB, logger *slog.Logger) *Resource {
	return &Resource{Service: service, ListExport: listExport, DB: db, Logger: logger}
}

func (rs *Resource) logger() *slog.Logger {
	if rs.Logger == nil {
		return slog.Default()
	}
	return rs.Logger
}

// Router registers the statistics routes. Both routes need config:read
// (the report gate used by the other admin reports) AND users:read (the
// per-child rows are personal data), mirroring the class-roster export.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	common.ProtectedTenantGroup(r, rs.DB, func(r chi.Router, withTx common.Middleware) {
		guard := common.RequiresAllPermissions(permissions.ConfigRead, permissions.UsersRead)
		r.With(guard, withTx).Get("/report", rs.getReport)
		r.With(guard, withTx).Get("/export", rs.exportReport)
	})
	return r
}

var renderError = common.RulesRenderer([]common.ErrorRule{
	{Target: studentpresence.ErrInvalidStatisticsRange, Render: common.ErrorInvalidRequest},
}, func(err error) render.Renderer {
	return common.ErrorInternalServerWrap("statistics failed", err)
})

func (rs *Resource) getReport(w http.ResponseWriter, r *http.Request) {
	filters, err := parseFilters(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	report, err := rs.Service.Report(r.Context(), filters, actorFromRequest(r))
	if err != nil {
		rs.logFailure("statistics report failed", err)
		common.RenderError(w, r, renderError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toReportResponse(report), "")
}

func (rs *Resource) exportReport(w http.ResponseWriter, r *http.Request) {
	if rs.ListExport == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("statistics export service is not configured")))
		return
	}
	filters, err := parseFilters(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	format, err := parseFormat(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	section, err := parseSection(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	report, err := rs.Service.ReportForExport(r.Context(), filters, actorFromRequest(r), string(format)+"/"+section)
	if err != nil {
		rs.logFailure("statistics export failed", err)
		common.RenderError(w, r, renderError(err))
		return
	}
	doc, name := buildSectionDocument(report, section)
	filename := name + "-" + report.From.String() + "-" + report.To.String()
	file, err := rs.ListExport.Render(doc, format, filename)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func (rs *Resource) logFailure(msg string, err error) {
	if errors.Is(err, studentpresence.ErrInvalidStatisticsRange) {
		return
	}
	rs.logger().Error(msg, slog.String("error", err.Error()))
}

func actorFromRequest(r *http.Request) studentpresence.StatisticsActor {
	claims := jwt.ClaimsFromCtx(r.Context())
	return studentpresence.StatisticsActor{
		AccountID: int64(claims.ID),
		Role:      strings.Join(claims.Roles, ","),
	}
}

// parseFilters reads from, to (YYYY-MM-DD, required) and group_id (repeatable).
func parseFilters(r *http.Request) (studentpresence.StatisticsFilters, error) {
	q := r.URL.Query()
	from, err := timezone.ParseDate(strings.TrimSpace(q.Get("from")))
	if err != nil {
		return studentpresence.StatisticsFilters{}, fmt.Errorf("invalid from date: %w", err)
	}
	to, err := timezone.ParseDate(strings.TrimSpace(q.Get("to")))
	if err != nil {
		return studentpresence.StatisticsFilters{}, fmt.Errorf("invalid to date: %w", err)
	}
	filters := studentpresence.StatisticsFilters{From: from, To: to}
	sections, err := parseReportSections(q["section"])
	if err != nil {
		return studentpresence.StatisticsFilters{}, err
	}
	filters.Sections = sections
	for _, raw := range q["group_id"] {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil || id < 0 {
				return studentpresence.StatisticsFilters{}, fmt.Errorf("invalid group_id %q", part)
			}
			filters.GroupIDs = append(filters.GroupIDs, id)
		}
	}
	return filters, nil
}

const (
	sectionAttendance     = "attendance"
	sectionRooms          = "rooms"
	sectionCourses        = "courses"
	sectionCourseStudents = "course-students"
)

// parseSection picks the export document: the child/group table (default),
// the room utilization table, or one of the two course tables. Each has its
// own column grid and is therefore its own document.
func parseSection(r *http.Request) (string, error) {
	section := strings.TrimSpace(r.URL.Query().Get("section"))
	switch section {
	case "", sectionAttendance:
		return sectionAttendance, nil
	case sectionRooms, sectionCourses, sectionCourseStudents:
		return section, nil
	default:
		return "", fmt.Errorf("unsupported export section %q", section)
	}
}

// parseReportSections limits which sections the report computes. Repeatable
// and comma-separated; absent means the whole report, which is what the
// screen asks for so switching tabs costs no request.
func parseReportSections(raw []string) ([]studentpresence.StatisticsSection, error) {
	var sections []studentpresence.StatisticsSection
	for _, value := range raw {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			switch part {
			case sectionAttendance:
				sections = append(sections, studentpresence.StatisticsSectionAttendance)
			case sectionRooms:
				sections = append(sections, studentpresence.StatisticsSectionRooms)
			case sectionCourses, sectionCourseStudents:
				sections = append(sections, studentpresence.StatisticsSectionCourses)
			default:
				return nil, fmt.Errorf("unsupported section %q", part)
			}
		}
	}
	return sections, nil
}

func parseFormat(r *http.Request) (listexport.Format, error) {
	format := listexport.Format(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = listexport.FormatPDF
	}
	switch format {
	case listexport.FormatPDF, listexport.FormatDOCX, listexport.FormatXLSX:
		return format, nil
	default:
		return format, fmt.Errorf("unsupported export format %q", format)
	}
}
