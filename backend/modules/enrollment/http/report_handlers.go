package enrollmenthttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

type careUsageExportRequest struct {
	Format  lists.Format                  `json:"format"`
	Layout  string                        `json:"layout"`
	Filters careUsageExportFiltersRequest `json:"filters"`
}

// Care-usage export layouts (#2215): "detailed" keeps the one-block-per-child
// record output, "compact" renders the class-roster-style table (one row per
// child, weekday pickup columns). XLSX is always tabular regardless of layout.
const (
	careUsageLayoutDetailed = "detailed"
	careUsageLayoutCompact  = "compact"
)

// careUsageExportParams embeds the service filters so existing callers keep
// promoted-field access; Layout is presentation-only and never reaches the
// service.
type careUsageExportParams struct {
	capability.CareUsageFilters
	Layout string
}

// careUsageExportPayload carries the fetched report together with the
// requested layout into the build step of the generic exportReport flow.
type careUsageExportPayload struct {
	report *capability.CareUsageReport
	layout string
}

type classRosterExportRequest struct {
	Format  lists.Format                    `json:"format"`
	Filters classRosterExportFiltersRequest `json:"filters"`
}

type classRosterExportFiltersRequest struct {
	PhaseID     json.RawMessage `json:"phase_id"`
	SchoolClass string          `json:"school_class"`
	AllClasses  bool            `json:"all_classes"`
}

type careUsageExportFiltersRequest struct {
	PhaseID         string   `json:"phase_id"`
	Status          string   `json:"status,omitempty"`
	CareOfferingID  string   `json:"care_offering_id,omitempty"`
	CareOfferingIDs []string `json:"care_offering_ids,omitempty"`
	DayCount        *int     `json:"day_count,omitempty"`
	GradeLevel      *int16   `json:"grade_level,omitempty"`
	Weekday         string   `json:"weekday,omitempty"`
	PickupTime      string   `json:"pickup_time,omitempty"`
	Search          string   `json:"search,omitempty"`
}

type careUsageReportResponse struct {
	Phase         careUsagePhaseResponse          `json:"phase"`
	Filters       careUsageAppliedFiltersResponse `json:"filters"`
	Totals        capability.CareUsageTotals      `json:"totals"`
	ByOffering    []careUsageOfferingStatResponse `json:"by_offering"`
	FilterOptions careUsageFilterOptionsResponse  `json:"filter_options"`
	Rows          []careUsageRowResponse          `json:"rows"`
}

type careUsagePhaseResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type careUsageAppliedFiltersResponse struct {
	PhaseID         string   `json:"phase_id"`
	Status          string   `json:"status"`
	CareOfferingIDs []string `json:"care_offering_ids"`
	DayCount        *int     `json:"day_count,omitempty"`
	GradeLevel      *int16   `json:"grade_level,omitempty"`
	Weekday         string   `json:"weekday,omitempty"`
	PickupTime      string   `json:"pickup_time,omitempty"`
	Search          string   `json:"search,omitempty"`
}

type careUsageOfferingStatResponse struct {
	OfferingID   string         `json:"offering_id"`
	OfferingName string         `json:"offering_name"`
	Children     int            `json:"children"`
	ByDayCount   map[string]int `json:"by_day_count"`
}

type careUsageFilterOptionsResponse struct {
	Offerings   []careUsageOfferingOptionResponse `json:"offerings"`
	GradeLevels []int16                           `json:"grade_levels"`
	PickupTimes []string                          `json:"pickup_times"`
}

type careUsageOfferingOptionResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CountsAsCare bool   `json:"counts_as_care"`
}

type careUsageRowResponse struct {
	RequestID         string                         `json:"request_id"`
	ChildID           string                         `json:"child_id"`
	ChildFirstName    string                         `json:"child_first_name"`
	ChildLastName     string                         `json:"child_last_name"`
	DateOfBirth       string                         `json:"date_of_birth"`
	TargetGradeLevel  *int16                         `json:"target_grade_level,omitempty"`
	TargetSchoolClass *string                        `json:"target_school_class,omitempty"`
	Status            string                         `json:"status"`
	Offerings         []careUsageRowOfferingResponse `json:"offerings"`
	EffectiveDays     []string                       `json:"effective_days"`
	DayCount          int                            `json:"day_count"`
	PickupByDay       map[string]string              `json:"pickup_by_day"`
	GuardianFirstName string                         `json:"guardian_first_name"`
	GuardianLastName  string                         `json:"guardian_last_name"`
	GuardianEmail     string                         `json:"guardian_email"`
	GuardianPhone     *string                        `json:"guardian_phone,omitempty"`
	SubmittedAt       time.Time                      `json:"submitted_at"`
}

type careUsageRowOfferingResponse struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Days                  []string `json:"days"`
	DaysSource            string   `json:"days_source"`
	DaysOfWeekMode        string   `json:"days_of_week_mode"`
	ManualSelectedDays    []string `json:"manual_selected_days,omitempty"`
	AutomaticSelectedDays []string `json:"automatic_selected_days,omitempty"`
}

func (rs *Resource) getCareUsageReport(w http.ResponseWriter, r *http.Request) {
	if rs.ReportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("report service not configured")))
		return
	}
	filters, err := parseCareUsageFiltersFromQuery(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var report *capability.CareUsageReport
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		out, e := rs.ReportService.CareUsage(ctx, filters)
		if e != nil {
			return e
		}
		report = out
		return nil
	})
	if err != nil {
		if errors.Is(err, capability.ErrReportPhaseNotFound) {
			common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentPhaseNotFound))
			return
		}
		if errors.Is(err, capability.ErrReportExportTooLarge) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		if errors.Is(err, capability.ErrReportInvalidFilter) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toCareUsageReportResponse(report), "Care usage report retrieved")
}

// exportReport is the shared body of the report-export handlers: parse the
// export request, fetch the report inside a tenant transaction, build the
// export file, and stream it as an attachment.
func exportReport[F, R any](rs *Resource, w http.ResponseWriter, r *http.Request,
	parse func(*http.Request) (lists.Format, F, error),
	fetch func(ctx context.Context, filters F, actorAccountID int64, actorRole, format string) (R, error),
	build func(svc lists.DocumentRenderer, report R, format lists.Format) (lists.File, error),
) {
	if rs.ReportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("report service not configured")))
		return
	}
	if rs.ListExportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("list export service not configured")))
		return
	}
	format, filters, err := parse(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	actorAccountID := int64(claims.ID)
	actorRole := strings.Join(claims.Roles, ",")

	var report R
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		out, e := fetch(ctx, filters, actorAccountID, actorRole, string(format))
		if e != nil {
			return e
		}
		report = out
		return nil
	})
	if err != nil {
		if errors.Is(err, capability.ErrReportPhaseNotFound) {
			common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentPhaseNotFound))
			return
		}
		if errors.Is(err, capability.ErrReportExportTooLarge) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		if errors.Is(err, capability.ErrReportInvalidFilter) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	file, err := build(rs.ListExportService, report, format)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	writeExportFile(w, file)
}

func (rs *Resource) exportCareUsageReport(w http.ResponseWriter, r *http.Request) {
	exportReport(rs, w, r, parseCareUsageExportRequest,
		func(ctx context.Context, params careUsageExportParams, actorAccountID int64, actorRole, format string) (careUsageExportPayload, error) {
			compact := params.Layout == careUsageLayoutCompact && format != string(lists.FormatXLSX)
			report, err := rs.ReportService.ExportCareUsage(ctx, params.CareUsageFilters, actorAccountID, actorRole, format, compact)
			if err != nil {
				return careUsageExportPayload{}, err
			}
			return careUsageExportPayload{report: report, layout: params.Layout}, nil
		}, buildCareUsageExport)
}

func (rs *Resource) exportClassRosterReport(w http.ResponseWriter, r *http.Request) {
	exportReport(rs, w, r, parseClassRosterExportRequest,
		func(ctx context.Context, filters capability.ClassRosterFilters, actorAccountID int64, actorRole, format string) (*capability.ClassRosterReport, error) {
			return rs.ReportService.ExportClassRoster(ctx, filters, actorAccountID, actorRole, format)
		}, buildClassRosterExportFile)
}

func parseCareUsageFiltersFromQuery(r *http.Request) (capability.CareUsageFilters, error) {
	q := r.URL.Query()
	var filters capability.CareUsageFilters
	phaseID, err := strconv.ParseInt(q.Get("phase_id"), 10, 64)
	if err != nil || phaseID <= 0 {
		return filters, errors.New("phase_id is required")
	}
	filters.PhaseID = phaseID
	filters.Status = q.Get("status")
	filters.Weekday = q.Get("weekday")
	filters.PickupTime = q.Get("pickup_time")
	filters.Search = q.Get("search")

	offeringIDs, offeringIDsSet, err := resolveCareOfferingIDs(q)
	if err != nil {
		return filters, err
	}
	filters.CareOfferingIDs = offeringIDs
	filters.CareOfferingIDsSet = offeringIDsSet

	dayCount, err := parseOptionalDayCount(q)
	if err != nil {
		return filters, err
	}
	filters.DayCount = dayCount

	gradeLevel, err := parseOptionalGradeLevel(q)
	if err != nil {
		return filters, err
	}
	filters.GradeLevel = gradeLevel
	return filters, nil
}

// resolveCareOfferingIDs merges the repeated/comma-joined care_offering_ids
// param with the legacy single care_offering_id param. The bool reports
// whether either param was present, so an explicit empty selection is
// distinguished from "not filtered".
func resolveCareOfferingIDs(q url.Values) ([]int64, bool, error) {
	offeringIDs, err := parseCareOfferingIDsFromQuery(q["care_offering_ids"])
	if err != nil {
		return nil, false, err
	}
	_, set := q["care_offering_ids"]
	if raw := q.Get("care_offering_id"); raw != "" {
		id, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || id <= 0 {
			return nil, false, errors.New("care_offering_id must be positive")
		}
		offeringIDs = append(offeringIDs, id)
		set = true
	}
	return offeringIDs, set, nil
}

// parseOptionalDayCount parses the optional day_count filter (0–7). An
// absent param yields (nil, nil).
func parseOptionalDayCount(q url.Values) (*int, error) {
	raw := q.Get("day_count")
	if raw == "" {
		return nil, nil
	}
	count, err := strconv.Atoi(raw)
	if err != nil || count < 0 || count > 7 {
		return nil, errors.New("day_count must be between 0 and 7")
	}
	return &count, nil
}

// parseOptionalGradeLevel parses the optional grade_level filter. An absent
// param yields (nil, nil).
func parseOptionalGradeLevel(q url.Values) (*int16, error) {
	raw := q.Get("grade_level")
	if raw == "" {
		return nil, nil
	}
	grade, err := strconv.ParseInt(raw, 10, 16)
	if err != nil || grade <= 0 {
		return nil, errors.New("grade_level must be positive")
	}
	g := int16(grade)
	return &g, nil
}

func parseCareOfferingIDsFromQuery(values []string) ([]int64, error) {
	if len(values) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			raw := strings.TrimSpace(part)
			if raw == "" {
				continue
			}
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id <= 0 {
				return nil, errors.New("care_offering_ids must contain positive ids")
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func parseCareUsageExportRequest(r *http.Request) (lists.Format, careUsageExportParams, error) {
	var body careUsageExportRequest
	if r.Body != nil {
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				return "", careUsageExportParams{}, fmt.Errorf("invalid export request body: %w", err)
			}
		}
	}
	format := lists.Format(strings.ToLower(string(body.Format)))
	if format == "" {
		format = lists.FormatXLSX
	}
	switch format {
	case lists.FormatPDF, lists.FormatDOCX, lists.FormatXLSX:
	default:
		return "", careUsageExportParams{}, fmt.Errorf("unsupported export format %q (use pdf, docx or xlsx)", format)
	}
	layout := strings.ToLower(strings.TrimSpace(body.Layout))
	if layout == "" {
		layout = careUsageLayoutDetailed
	}
	switch layout {
	case careUsageLayoutDetailed, careUsageLayoutCompact:
	default:
		return "", careUsageExportParams{}, fmt.Errorf("unsupported export layout %q (use detailed or compact)", layout)
	}
	filters, err := body.Filters.toServiceFilters()
	if err != nil {
		return "", careUsageExportParams{}, err
	}
	if filters.PhaseID <= 0 {
		return "", careUsageExportParams{}, errors.New("filters.phase_id is required")
	}
	return format, careUsageExportParams{CareUsageFilters: filters, Layout: layout}, nil
}

func parseClassRosterExportRequest(r *http.Request) (lists.Format, capability.ClassRosterFilters, error) {
	var body classRosterExportRequest
	if r.Body != nil {
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				return "", capability.ClassRosterFilters{}, fmt.Errorf("invalid export request body: %w", err)
			}
		}
	}
	format := lists.Format(strings.ToLower(string(body.Format)))
	if format == "" {
		format = lists.FormatPDF
	}
	switch format {
	case lists.FormatPDF, lists.FormatDOCX, lists.FormatXLSX:
	default:
		return "", capability.ClassRosterFilters{}, fmt.Errorf("unsupported export format %q (use pdf, docx or xlsx)", format)
	}
	phaseID, err := parseClassRosterPhaseID(body.Filters.PhaseID)
	if err != nil {
		return "", capability.ClassRosterFilters{}, err
	}
	filters := capability.ClassRosterFilters{
		PhaseID:     phaseID,
		SchoolClass: strings.TrimSpace(body.Filters.SchoolClass),
		AllClasses:  body.Filters.AllClasses,
	}
	if filters.AllClasses && filters.SchoolClass != "" {
		return "", capability.ClassRosterFilters{}, errors.New("school_class and all_classes are mutually exclusive")
	}
	if !filters.AllClasses && filters.SchoolClass == "" {
		return "", capability.ClassRosterFilters{}, errors.New("school_class is required")
	}
	return format, filters, nil
}

func parseClassRosterPhaseID(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, errors.New("phase_id is required")
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		phaseID, parseErr := strconv.ParseInt(strings.TrimSpace(asString), 10, 64)
		if parseErr != nil || phaseID <= 0 {
			return 0, errors.New("phase_id must be positive")
		}
		return phaseID, nil
	}
	var asNumber int64
	if err := json.Unmarshal(raw, &asNumber); err == nil && asNumber > 0 {
		return asNumber, nil
	}
	return 0, errors.New("phase_id must be positive")
}

func (req careUsageExportFiltersRequest) toServiceFilters() (capability.CareUsageFilters, error) {
	phaseID, err := parseRequiredPositiveInt64(req.PhaseID, "filters.phase_id")
	if err != nil {
		return capability.CareUsageFilters{}, err
	}
	filters := capability.CareUsageFilters{
		PhaseID:    phaseID,
		Status:     req.Status,
		DayCount:   req.DayCount,
		GradeLevel: req.GradeLevel,
		Weekday:    req.Weekday,
		PickupTime: req.PickupTime,
		Search:     req.Search,
	}
	if strings.TrimSpace(req.CareOfferingID) != "" {
		careOfferingID, err := parseRequiredPositiveInt64(req.CareOfferingID, "filters.care_offering_id")
		if err != nil {
			return capability.CareUsageFilters{}, err
		}
		filters.CareOfferingIDs = append(filters.CareOfferingIDs, careOfferingID)
		filters.CareOfferingIDsSet = true
	}
	if req.CareOfferingIDs != nil {
		filters.CareOfferingIDsSet = true
	}
	for _, raw := range req.CareOfferingIDs {
		careOfferingID, err := parseRequiredPositiveInt64(raw, "filters.care_offering_ids")
		if err != nil {
			return capability.CareUsageFilters{}, err
		}
		filters.CareOfferingIDs = append(filters.CareOfferingIDs, careOfferingID)
	}
	return filters, nil
}

func parseRequiredPositiveInt64(raw, field string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("%s is required", field)
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%s must be positive", field)
	}
	return id, nil
}

func toCareUsageReportResponse(report *capability.CareUsageReport) *careUsageReportResponse {
	if report == nil {
		return nil
	}
	out := &careUsageReportResponse{
		Phase: careUsagePhaseResponse{
			ID:   strconv.FormatInt(report.Phase.ID, 10),
			Name: report.Phase.Name,
		},
		Filters: careUsageAppliedFiltersResponse{
			PhaseID:    strconv.FormatInt(report.Filters.PhaseID, 10),
			Status:     report.Filters.Status,
			DayCount:   report.Filters.DayCount,
			GradeLevel: report.Filters.GradeLevel,
			Weekday:    report.Filters.Weekday,
			PickupTime: report.Filters.PickupTime,
			Search:     report.Filters.Search,
		},
		Totals: report.Totals,
		FilterOptions: careUsageFilterOptionsResponse{
			GradeLevels: report.FilterOptions.GradeLevels,
			PickupTimes: report.FilterOptions.PickupTimes,
		},
		Rows: make([]careUsageRowResponse, 0, len(report.Rows)),
	}
	out.Filters.CareOfferingIDs = make([]string, 0, len(report.Filters.CareOfferingIDs))
	for _, id := range report.Filters.CareOfferingIDs {
		out.Filters.CareOfferingIDs = append(out.Filters.CareOfferingIDs, strconv.FormatInt(id, 10))
	}
	out.ByOffering = make([]careUsageOfferingStatResponse, 0, len(report.ByOffering))
	for _, stat := range report.ByOffering {
		out.ByOffering = append(out.ByOffering, careUsageOfferingStatResponse{
			OfferingID:   strconv.FormatInt(stat.OfferingID, 10),
			OfferingName: stat.OfferingName,
			Children:     stat.Children,
			ByDayCount:   stat.ByDayCount,
		})
	}
	out.FilterOptions.Offerings = make([]careUsageOfferingOptionResponse, 0, len(report.FilterOptions.Offerings))
	for _, option := range report.FilterOptions.Offerings {
		out.FilterOptions.Offerings = append(out.FilterOptions.Offerings, careUsageOfferingOptionResponse{
			ID:           strconv.FormatInt(option.ID, 10),
			Name:         option.Name,
			CountsAsCare: option.CountsAsCare,
		})
	}
	for _, row := range report.Rows {
		out.Rows = append(out.Rows, toCareUsageRowResponse(row))
	}
	return out
}

func toCareUsageRowResponse(row capability.CareUsageRow) careUsageRowResponse {
	rowOut := careUsageRowResponse{
		RequestID:         strconv.FormatInt(row.RequestID, 10),
		ChildID:           strconv.FormatInt(row.ChildID, 10),
		ChildFirstName:    row.ChildFirstName,
		ChildLastName:     row.ChildLastName,
		DateOfBirth:       row.DateOfBirth,
		TargetGradeLevel:  row.TargetGradeLevel,
		TargetSchoolClass: row.TargetSchoolClass,
		Status:            row.Status,
		EffectiveDays:     nonNilStringSlice(row.EffectiveDays),
		DayCount:          row.DayCount,
		PickupByDay:       nonNilStringMap(row.PickupByDay),
		GuardianFirstName: row.GuardianFirstName,
		GuardianLastName:  row.GuardianLastName,
		GuardianEmail:     row.GuardianEmail,
		GuardianPhone:     row.GuardianPhone,
		SubmittedAt:       row.SubmittedAt,
		Offerings:         make([]careUsageRowOfferingResponse, 0, len(row.Offerings)),
	}
	for _, offering := range row.Offerings {
		rowOut.Offerings = append(rowOut.Offerings, careUsageRowOfferingResponse{
			ID:                    strconv.FormatInt(offering.ID, 10),
			Name:                  offering.Name,
			Days:                  nonNilStringSlice(offering.Days),
			DaysSource:            offering.DaysSource,
			DaysOfWeekMode:        offering.DaysOfWeekMode,
			ManualSelectedDays:    offering.ManualSelectedDays,
			AutomaticSelectedDays: offering.AutomaticSelectedDays,
		})
	}
	return rowOut
}

func nonNilStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonNilStringMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	return values
}
