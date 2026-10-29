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
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Resource is the statistics API resource.
type Resource struct {
	Service studentpresence.StatisticsReports
	Runtime Runtime

	Logger *slog.Logger
}

// NewResource creates the statistics resource.
func NewResource(service studentpresence.StatisticsReports, runtime Runtime, logger *slog.Logger) *Resource {
	return &Resource{Service: service, Runtime: runtime, Logger: logger}
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
func (rs *Resource) Router(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	protectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {
		guard := rs.Runtime.ReportGuard
		r.With(guard, withTx).Get("/report", rs.getReport)
		r.With(guard, withTx).Get("/export", rs.exportReport)
	})
	return r
}

var renderError = common.RulesRenderer([]common.ErrorRule{
	{Target: studentpresence.ErrInvalidStatisticsRange, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodePresenceStatisticsRangeInvalid)
	}},
}, func(err error) render.Renderer {
	return common.ErrorInternalServerWrap("statistics failed", err)
})

func (rs *Resource) getReport(w http.ResponseWriter, r *http.Request) {
	filters, err := parseFilters(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	report, err := rs.Service.Report(r.Context(), filters, rs.Runtime.Actor(r))
	if err != nil {
		rs.logFailure("statistics report failed", err)
		common.RenderError(w, r, renderError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toReportResponse(report), "")
}

func (rs *Resource) exportReport(w http.ResponseWriter, r *http.Request) {
	if rs.Runtime.Export == nil {
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
	report, err := rs.Service.ReportForExport(r.Context(), filters, rs.Runtime.Actor(r), string(format)+"/"+section)
	if err != nil {
		rs.logFailure("statistics export failed", err)
		common.RenderError(w, r, renderError(err))
		return
	}
	doc, name := buildSectionDocument(report, section)
	filename := name + "-" + report.From.String() + "-" + report.To.String()
	file, err := rs.Runtime.Export(doc, format, filename)
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

// parseFilters reads from, to (YYYY-MM-DD, required), group_id and
// school_class (both repeatable).
func parseFilters(r *http.Request) (studentpresence.StatisticsFilters, error) {
	q := r.URL.Query()
	from, err := calendar.ParseDate(strings.TrimSpace(q.Get("from")))
	if err != nil {
		return studentpresence.StatisticsFilters{}, fmt.Errorf("invalid from date: %w", err)
	}
	to, err := calendar.ParseDate(strings.TrimSpace(q.Get("to")))
	if err != nil {
		return studentpresence.StatisticsFilters{}, fmt.Errorf("invalid to date: %w", err)
	}
	filters := studentpresence.StatisticsFilters{From: from, To: to}
	sections, err := parseReportSections(q["section"])
	if err != nil {
		return studentpresence.StatisticsFilters{}, err
	}
	filters.Sections = sections
	if filters.GroupIDs, err = parseGroupIDs(q["group_id"]); err != nil {
		return studentpresence.StatisticsFilters{}, err
	}
	filters.SchoolClasses = parseSchoolClasses(q["school_class"])
	return filters, nil
}

// parseGroupIDs reads group_id, repeatable and comma-separated; 0 is the
// pseudo group of children without a group.
func parseGroupIDs(raw []string) ([]int64, error) {
	var ids []int64
	for _, value := range raw {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil || id < 0 {
				return nil, fmt.Errorf("invalid group_id %q", part)
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// parseSchoolClasses reads school_class. Class names may contain commas in
// theory, so the parameter is only repeatable, never comma-separated.
func parseSchoolClasses(raw []string) []string {
	var classes []string
	for _, value := range raw {
		if class := strings.TrimSpace(value); class != "" {
			classes = append(classes, class)
		}
	}
	return classes
}

const (
	sectionAttendance     = "attendance"
	sectionRooms          = "rooms"
	sectionCourses        = "courses"
	sectionCourseStudents = "course-students"
	sectionOccupancy      = "occupancy"
)

// parseSection picks the export document: the child/group table (default),
// the room utilization table, or one of the two course tables. Each has its
// own column grid and is therefore its own document.
func parseSection(r *http.Request) (string, error) {
	section := strings.TrimSpace(r.URL.Query().Get("section"))
	switch section {
	case "", sectionAttendance:
		return sectionAttendance, nil
	case sectionRooms, sectionCourses, sectionCourseStudents, sectionOccupancy:
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
			case sectionOccupancy:
				sections = append(sections, studentpresence.StatisticsSectionOccupancy)
			default:
				return nil, fmt.Errorf("unsupported section %q", part)
			}
		}
	}
	return sections, nil
}

func parseFormat(r *http.Request) (ExportFormat, error) {
	format := ExportFormat(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = formatPDF
	}
	switch format {
	case formatPDF, formatDOCX, formatXLSX:
		return format, nil
	default:
		return format, fmt.Errorf("unsupported export format %q", format)
	}
}
