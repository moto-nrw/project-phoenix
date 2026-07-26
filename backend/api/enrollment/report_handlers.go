package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/models/base"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

type careUsageExportRequest struct {
	Format  listexport.Format             `json:"format"`
	Filters careUsageExportFiltersRequest `json:"filters"`
}

type classRosterExportRequest struct {
	Format  listexport.Format               `json:"format"`
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
	Phase         careUsagePhaseResponse            `json:"phase"`
	Filters       careUsageAppliedFiltersResponse   `json:"filters"`
	Totals        enrollmentService.CareUsageTotals `json:"totals"`
	ByOffering    []careUsageOfferingStatResponse   `json:"by_offering"`
	FilterOptions careUsageFilterOptionsResponse    `json:"filter_options"`
	Rows          []careUsageRowResponse            `json:"rows"`
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
	var report *enrollmentService.CareUsageReport
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		out, e := rs.ReportService.CareUsage(ctx, filters)
		if e != nil {
			return e
		}
		report = out
		return nil
	})
	if err != nil {
		if errors.Is(err, enrollmentService.ErrReportPhaseNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		if errors.Is(err, enrollmentService.ErrReportExportTooLarge) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		if errors.Is(err, enrollmentService.ErrReportInvalidFilter) {
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
	parse func(*http.Request) (listexport.Format, F, error),
	fetch func(ctx context.Context, filters F, actorAccountID int64, actorRole, format string) (R, error),
	build func(svc *listexport.RendererService, report R, format listexport.Format) (listexport.File, error),
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
		if errors.Is(err, enrollmentService.ErrReportPhaseNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		if errors.Is(err, enrollmentService.ErrReportExportTooLarge) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		if errors.Is(err, enrollmentService.ErrReportInvalidFilter) {
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
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func (rs *Resource) exportCareUsageReport(w http.ResponseWriter, r *http.Request) {
	exportReport(rs, w, r, parseCareUsageExportRequest,
		func(ctx context.Context, filters enrollmentService.CareUsageFilters, actorAccountID int64, actorRole, format string) (*enrollmentService.CareUsageReport, error) {
			return rs.ReportService.ExportCareUsage(ctx, filters, actorAccountID, actorRole, format)
		}, buildCareUsageExportFile)
}

func (rs *Resource) exportClassRosterReport(w http.ResponseWriter, r *http.Request) {
	exportReport(rs, w, r, parseClassRosterExportRequest,
		func(ctx context.Context, filters enrollmentService.ClassRosterFilters, actorAccountID int64, actorRole, format string) (*enrollmentService.ClassRosterReport, error) {
			return rs.ReportService.ExportClassRoster(ctx, filters, actorAccountID, actorRole, format)
		}, buildClassRosterExportFile)
}

func parseCareUsageFiltersFromQuery(r *http.Request) (enrollmentService.CareUsageFilters, error) {
	q := r.URL.Query()
	var filters enrollmentService.CareUsageFilters
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

func parseCareUsageExportRequest(r *http.Request) (listexport.Format, enrollmentService.CareUsageFilters, error) {
	var body careUsageExportRequest
	if r.Body != nil {
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				return "", enrollmentService.CareUsageFilters{}, fmt.Errorf("invalid export request body: %w", err)
			}
		}
	}
	format := listexport.Format(strings.ToLower(string(body.Format)))
	if format == "" {
		format = listexport.FormatXLSX
	}
	switch format {
	case listexport.FormatPDF, listexport.FormatDOCX, listexport.FormatXLSX:
	default:
		return "", enrollmentService.CareUsageFilters{}, fmt.Errorf("unsupported export format %q (use pdf, docx or xlsx)", format)
	}
	filters, err := body.Filters.toServiceFilters()
	if err != nil {
		return "", enrollmentService.CareUsageFilters{}, err
	}
	if filters.PhaseID <= 0 {
		return "", enrollmentService.CareUsageFilters{}, errors.New("filters.phase_id is required")
	}
	return format, filters, nil
}

func parseClassRosterExportRequest(r *http.Request) (listexport.Format, enrollmentService.ClassRosterFilters, error) {
	var body classRosterExportRequest
	if r.Body != nil {
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				return "", enrollmentService.ClassRosterFilters{}, fmt.Errorf("invalid export request body: %w", err)
			}
		}
	}
	format := listexport.Format(strings.ToLower(string(body.Format)))
	if format == "" {
		format = listexport.FormatPDF
	}
	switch format {
	case listexport.FormatPDF, listexport.FormatDOCX, listexport.FormatXLSX:
	default:
		return "", enrollmentService.ClassRosterFilters{}, fmt.Errorf("unsupported export format %q (use pdf, docx or xlsx)", format)
	}
	phaseID, err := parseClassRosterPhaseID(body.Filters.PhaseID)
	if err != nil {
		return "", enrollmentService.ClassRosterFilters{}, err
	}
	filters := enrollmentService.ClassRosterFilters{
		PhaseID:     phaseID,
		SchoolClass: strings.TrimSpace(body.Filters.SchoolClass),
		AllClasses:  body.Filters.AllClasses,
	}
	if filters.AllClasses && filters.SchoolClass != "" {
		return "", enrollmentService.ClassRosterFilters{}, errors.New("school_class and all_classes are mutually exclusive")
	}
	if !filters.AllClasses && filters.SchoolClass == "" {
		return "", enrollmentService.ClassRosterFilters{}, errors.New("school_class is required")
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

func (req careUsageExportFiltersRequest) toServiceFilters() (enrollmentService.CareUsageFilters, error) {
	phaseID, err := parseRequiredPositiveInt64(req.PhaseID, "filters.phase_id")
	if err != nil {
		return enrollmentService.CareUsageFilters{}, err
	}
	filters := enrollmentService.CareUsageFilters{
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
			return enrollmentService.CareUsageFilters{}, err
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
			return enrollmentService.CareUsageFilters{}, err
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

func toCareUsageReportResponse(report *enrollmentService.CareUsageReport) *careUsageReportResponse {
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
		out.Rows = append(out.Rows, rowOut)
	}
	return out
}

func buildCareUsageExportFile(svc *listexport.RendererService, report *enrollmentService.CareUsageReport, format listexport.Format) (listexport.File, error) {
	filename := "Anmelde-Auswertung " + strings.TrimSpace(report.Phase.Name)
	if strings.TrimSpace(report.Phase.Name) == "" {
		filename = "Anmelde-Auswertung"
	}
	switch format {
	case listexport.FormatDOCX:
		return svc.RenderRecordsDOCX(buildCareUsageRecordDocument(report), filename)
	case listexport.FormatPDF:
		return svc.RenderRecords(buildCareUsageRecordDocument(report), filename)
	case listexport.FormatXLSX:
		return svc.Render(buildCareUsageTableDocument(report), listexport.FormatXLSX, filename)
	default:
		return listexport.File{}, fmt.Errorf("unsupported export format %q", format)
	}
}

func buildClassRosterExportFile(svc *listexport.RendererService, report *enrollmentService.ClassRosterReport, format listexport.Format) (listexport.File, error) {
	filename := "Klassenliste " + strings.TrimSpace(report.Filters.SchoolClass)
	if report.Filters.AllClasses {
		filename = "Klassenlisten"
	}
	if phaseName := strings.TrimSpace(report.Phase.Name); phaseName != "" {
		filename += " " + phaseName
	}
	return svc.Render(buildClassRosterTableDocument(report), format, filename)
}

func buildClassRosterTableDocument(report *enrollmentService.ClassRosterReport) listexport.Document {
	cols := []listexport.Column{
		{ID: listexport.ColumnName, Label: "Name"},
		{ID: listexport.ColumnSchoolClass, Label: "Klasse"},
		{ID: listexport.ColumnWeeklyMonday, Label: "Montag"},
		{ID: listexport.ColumnWeeklyTuesday, Label: "Dienstag"},
		{ID: listexport.ColumnWeeklyWednesday, Label: "Mittwoch"},
		{ID: listexport.ColumnWeeklyThursday, Label: "Donnerstag"},
		{ID: listexport.ColumnWeeklyFriday, Label: "Freitag"},
		{ID: listexport.ColumnDeparture, Label: "Geh-/Abholweise"},
		{ID: listexport.ColumnGuardianContacts, Label: "Erziehungsberechtigte"},
	}
	rows := make([]listexport.Row, 0, len(report.Rows))
	currentClass := ""
	for i, row := range report.Rows {
		// All-classes rosters arrive class-sorted from the service; a
		// heading row starts each class section (new page in the PDF).
		// Boundaries use the sort comparator's equivalence so label
		// variants like "1a"/"1A" share one heading (first-seen label).
		if class := strings.TrimSpace(row.SchoolClass); report.Filters.AllClasses && (i == 0 || collation.CompareSchoolClasses(class, currentClass) != 0) {
			currentClass = class
			rows = append(rows, listexport.Row{GroupTitle: listexport.ClassGroupTitle(class)})
		}
		rows = append(rows, listexport.Row{Values: map[listexport.ColumnID]string{
			listexport.ColumnName:             strings.TrimSpace(row.FirstName + " " + row.LastName),
			listexport.ColumnSchoolClass:      row.SchoolClass,
			listexport.ColumnWeeklyMonday:     classRosterWeeklyCell(row, "mon"),
			listexport.ColumnWeeklyTuesday:    classRosterWeeklyCell(row, "tue"),
			listexport.ColumnWeeklyWednesday:  classRosterWeeklyCell(row, "wed"),
			listexport.ColumnWeeklyThursday:   classRosterWeeklyCell(row, "thu"),
			listexport.ColumnWeeklyFriday:     classRosterWeeklyCell(row, "fri"),
			listexport.ColumnDeparture:        row.Departure,
			listexport.ColumnGuardianContacts: classRosterGuardianContactsLabel(row.Guardians),
		}})
	}
	return listexport.Document{
		Title:       classRosterTitle(report),
		Subtitle:    classRosterSubtitle(report),
		GeneratedAt: time.Now(),
		Filters:     classRosterFilterLabels(report),
		Columns:     cols,
		Rows:        rows,
		Footer:      exportConfidentialityNote,
	}
}

func classRosterTitle(report *enrollmentService.ClassRosterReport) string {
	title := "Klassenliste"
	if report == nil {
		return title
	}
	if report.Filters.AllClasses {
		title = "Klassenlisten"
	} else if className := strings.TrimSpace(report.Filters.SchoolClass); className != "" {
		title += " " + className
	}
	if phaseName := strings.TrimSpace(report.Phase.Name); phaseName != "" {
		title += " - " + phaseName
	}
	return title
}

func classRosterSubtitle(report *enrollmentService.ClassRosterReport) string {
	if report == nil {
		return "0 Kinder"
	}
	return fmt.Sprintf("%d Kinder, %d angemeldet", report.Totals.Students, report.Totals.Registered)
}

func classRosterFilterLabels(report *enrollmentService.ClassRosterReport) []string {
	if report == nil {
		return nil
	}
	classLabel := strings.TrimSpace(report.Filters.SchoolClass)
	if report.Filters.AllClasses {
		classLabel = "Alle Klassen"
	}
	return []string{
		"Anmeldephase: " + strings.TrimSpace(report.Phase.Name),
		"Klasse: " + classLabel,
		"Status: " + statusLabelDE(report.Filters.Status),
	}
}

func classRosterWeeklyCell(row enrollmentService.ClassRosterRow, day string) string {
	parts := []string{}
	offerings := classRosterDailyOfferings(row, day)
	if len(offerings) > 0 {
		parts = append(parts, strings.Join(offerings, "; "))
	} else if containsReportDay(row.CareDays, day) {
		parts = append(parts, "Betreuung")
	}
	pickup := strings.TrimSpace(row.PickupByDay[day])
	if pickup != "" {
		parts = append(parts, "bis "+pickup)
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	return "nein"
}

func classRosterDailyOfferings(row enrollmentService.ClassRosterRow, day string) []string {
	normalizedDay := strings.ToLower(strings.TrimSpace(day))
	if normalizedDay == "" {
		return nil
	}
	if names := normalizedClassRosterOfferingNames(row.OfferingsByDay[normalizedDay]); len(names) > 0 {
		return names
	}
	if len(row.Offerings) == 0 {
		return nil
	}
	seen := map[string]bool{}
	names := []string{}
	for _, offering := range row.Offerings {
		if !containsReportDay(offering.Days, normalizedDay) {
			continue
		}
		name := strings.TrimSpace(offering.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func classRosterGuardianContactsLabel(guardians []enrollmentService.ClassRosterGuardian) string {
	parts := make([]string, 0, len(guardians))
	for _, guardian := range guardians {
		name := strings.TrimSpace(guardian.Name)
		details := []string{}
		if email := strings.TrimSpace(guardian.Email); email != "" {
			details = append(details, email)
		}
		if phone := strings.TrimSpace(guardian.Phone); phone != "" {
			details = append(details, phone)
		}
		switch {
		case name != "" && len(details) > 0:
			parts = append(parts, name+" ("+strings.Join(details, ", ")+")")
		case name != "":
			parts = append(parts, name)
		case len(details) > 0:
			parts = append(parts, strings.Join(details, ", "))
		}
	}
	return strings.Join(parts, "; ")
}

func normalizedClassRosterOfferingNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func containsReportDay(days []string, needle string) bool {
	for _, day := range days {
		if strings.EqualFold(strings.TrimSpace(day), needle) {
			return true
		}
	}
	return false
}

func buildCareUsageTableDocument(report *enrollmentService.CareUsageReport) listexport.Document {
	cols := []listexport.Column{
		{ID: "child_last_name", Label: "Kind Nachname"},
		{ID: "child_first_name", Label: "Kind Vorname"},
		{ID: "child_grade", Label: "Zielklasse"},
		{ID: "child_status", Label: "Status"},
		{ID: "offerings", Label: "Betreuungsangebote"},
		{ID: "offering_days", Label: "Tage je Angebot"},
		{ID: "effective_days", Label: "Effektive Betreuungstage"},
		{ID: "day_count", Label: "Anzahl Tage"},
		{ID: "pickup_mon", Label: "Mo Gehzeit"},
		{ID: "pickup_tue", Label: "Di Gehzeit"},
		{ID: "pickup_wed", Label: "Mi Gehzeit"},
		{ID: "pickup_thu", Label: "Do Gehzeit"},
		{ID: "pickup_fri", Label: "Fr Gehzeit"},
		{ID: "guardian_name", Label: "Eltern"},
		{ID: "guardian_email", Label: "E-Mail"},
		{ID: "guardian_phone", Label: "Telefon"},
	}
	rows := make([]listexport.Row, 0, len(report.Rows))
	for _, row := range report.Rows {
		rows = append(rows, listexport.Row{Values: map[listexport.ColumnID]string{
			"child_last_name":  row.ChildLastName,
			"child_first_name": row.ChildFirstName,
			"child_grade":      schoolClassLabel(row.TargetSchoolClass, row.TargetGradeLevel),
			"child_status":     statusLabelDE(row.Status),
			"offerings":        careUsageOfferingNames(row.Offerings),
			"offering_days":    careUsageOfferingDayDetails(row.Offerings),
			"effective_days":   formatDayCodes(row.EffectiveDays),
			"day_count":        strconv.Itoa(row.DayCount),
			"pickup_mon":       row.PickupByDay["mon"],
			"pickup_tue":       row.PickupByDay["tue"],
			"pickup_wed":       row.PickupByDay["wed"],
			"pickup_thu":       row.PickupByDay["thu"],
			"pickup_fri":       row.PickupByDay["fri"],
			"guardian_name":    strings.TrimSpace(row.GuardianFirstName + " " + row.GuardianLastName),
			"guardian_email":   row.GuardianEmail,
			"guardian_phone":   base.Deref(row.GuardianPhone),
		}})
	}
	return listexport.Document{
		Title:       careUsageTitle(report),
		Subtitle:    careUsageSubtitle(report),
		GeneratedAt: time.Now(),
		Filters:     careUsageFilterLabels(report),
		Columns:     cols,
		Rows:        rows,
		Footer:      exportConfidentialityNote,
	}
}

func buildCareUsageRecordDocument(report *enrollmentService.CareUsageReport) listexport.RecordDocument {
	records := make([]listexport.Record, 0, len(report.Rows)+1)
	records = append(records, listexport.Record{
		Title:  "Einsatzplanung nach Gehzeit",
		Fields: careUsagePickupPlanningFields(report),
	})
	for _, row := range report.Rows {
		records = append(records, listexport.Record{
			Title: strings.TrimSpace(row.ChildFirstName + " " + row.ChildLastName),
			Fields: []listexport.Field{
				{Label: "Zielklasse", Value: schoolClassLabel(row.TargetSchoolClass, row.TargetGradeLevel)},
				{Label: "Status", Value: statusLabelDE(row.Status)},
				{Label: "Betreuungsangebote", Value: careUsageOfferingDayDetails(row.Offerings)},
				{Label: "Effektive Betreuungstage", Value: formatDayCodes(row.EffectiveDays)},
				{Label: "Anzahl Tage", Value: strconv.Itoa(row.DayCount)},
				{Label: "Gehzeiten", Value: careUsagePickupDayDetails(row.PickupByDay)},
				{Label: "Eltern", Value: strings.TrimSpace(row.GuardianFirstName + " " + row.GuardianLastName)},
				{Label: "E-Mail", Value: row.GuardianEmail},
				{Label: "Telefon", Value: base.Deref(row.GuardianPhone)},
			},
		})
	}
	return listexport.RecordDocument{
		Title:       careUsageTitle(report),
		Subtitle:    careUsageSubtitle(report),
		GeneratedAt: time.Now(),
		Footer:      exportConfidentialityNote,
		Filters:     careUsageFilterLabels(report),
		Records:     records,
	}
}

func careUsageTitle(report *enrollmentService.CareUsageReport) string {
	if report != nil && strings.TrimSpace(report.Phase.Name) != "" {
		return "Auswertung " + strings.TrimSpace(report.Phase.Name)
	}
	return "Anmelde-Auswertung"
}

func careUsageSubtitle(report *enrollmentService.CareUsageReport) string {
	if report == nil {
		return "0 Kinder"
	}
	if report.Totals.Children == 1 {
		return "1 Kind"
	}
	return fmt.Sprintf("%d Kinder", report.Totals.Children)
}

func careUsageFilterLabels(report *enrollmentService.CareUsageReport) []string {
	if report == nil {
		return nil
	}
	statusLabel := statusLabelDE(report.Filters.Status)
	if report.Filters.Status == "all" {
		statusLabel = "Alle"
	}
	labels := []string{"Status: " + statusLabel}
	if len(report.Filters.CareOfferingIDs) > 0 {
		names := make([]string, 0, len(report.Filters.CareOfferingIDs))
		for _, id := range report.Filters.CareOfferingIDs {
			names = append(names, careUsageOfferingNameByID(report, id))
		}
		labels = append(labels, "Betreuungsangebote: "+strings.Join(names, ", "))
	}
	if report.Filters.DayCount != nil {
		labels = append(labels, fmt.Sprintf("Tage: %d", *report.Filters.DayCount))
	}
	if report.Filters.GradeLevel != nil {
		labels = append(labels, fmt.Sprintf("Zielklasse: %d", *report.Filters.GradeLevel))
	}
	if strings.TrimSpace(report.Filters.Weekday) != "" {
		labels = append(labels, "Wochentag: "+formatDayCodes([]string{report.Filters.Weekday}))
	}
	if strings.TrimSpace(report.Filters.PickupTime) != "" {
		labels = append(labels, "Gehzeit: "+strings.TrimSpace(report.Filters.PickupTime))
	}
	if strings.TrimSpace(report.Filters.Search) != "" {
		labels = append(labels, "Suche: "+strings.TrimSpace(report.Filters.Search))
	}
	return labels
}

func careUsageOfferingNameByID(report *enrollmentService.CareUsageReport, id int64) string {
	for _, option := range report.FilterOptions.Offerings {
		if option.ID == id {
			return option.Name
		}
	}
	return "Angebot #" + strconv.FormatInt(id, 10)
}

func careUsageOfferingNames(offerings []enrollmentService.CareUsageRowOffering) string {
	parts := make([]string, 0, len(offerings))
	for _, offering := range offerings {
		parts = append(parts, offering.Name)
	}
	return strings.Join(parts, "; ")
}

func careUsageOfferingDayDetails(offerings []enrollmentService.CareUsageRowOffering) string {
	parts := make([]string, 0, len(offerings))
	for _, offering := range offerings {
		details := careUsageOfferingDayDetail(offering)
		if details == "" {
			parts = append(parts, offering.Name)
			continue
		}
		parts = append(parts, offering.Name+" ("+details+")")
	}
	return strings.Join(parts, "; ")
}

func careUsageOfferingDayDetail(offering enrollmentService.CareUsageRowOffering) string {
	automatic := formatDayCodes(offering.AutomaticSelectedDays)
	manualDays := offering.ManualSelectedDays
	if len(manualDays) == 0 && offering.DaysSource == "selected" && len(offering.AutomaticSelectedDays) == 0 {
		manualDays = offering.Days
	}
	manual := formatDayCodes(manualDays)
	if automatic != "" && manual != "" {
		return automatic + " automatisch; " + manual + " manuell"
	}
	if automatic != "" {
		return automatic + " automatisch"
	}
	if manual != "" {
		return manual + " Elternauswahl"
	}
	return formatDayCodes(offering.Days)
}

func careUsagePickupPlanningFields(report *enrollmentService.CareUsageReport) []listexport.Field {
	pickupTimes := careUsagePickupPlanningTimes(report)
	fields := make([]listexport.Field, 0, len(weekdayOrder)*len(pickupTimes))
	for _, day := range weekdayOrder {
		for _, pickupTime := range pickupTimes {
			count := 0
			if report != nil && report.Totals.ByWeekdayPickupTime[day] != nil {
				count = report.Totals.ByWeekdayPickupTime[day][pickupTime]
			}
			fields = append(fields, listexport.Field{
				Label: dayLabelsDE[day] + " bis " + pickupTime,
				Value: strconv.Itoa(count),
			})
		}
	}
	return fields
}

func careUsagePickupPlanningTimes(report *enrollmentService.CareUsageReport) []string {
	if report == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, pickupTime := range report.FilterOptions.PickupTimes {
		pickupTime = strings.TrimSpace(pickupTime)
		if pickupTime != "" {
			seen[pickupTime] = true
		}
	}
	for _, byPickupTime := range report.Totals.ByWeekdayPickupTime {
		for pickupTime := range byPickupTime {
			pickupTime = strings.TrimSpace(pickupTime)
			if pickupTime != "" {
				seen[pickupTime] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

func careUsagePickupDayDetails(pickupByDay map[string]string) string {
	if len(pickupByDay) == 0 {
		return ""
	}
	parts := make([]string, 0, len(weekdayOrder))
	for _, day := range weekdayOrder {
		pickupTime := strings.TrimSpace(pickupByDay[day])
		if pickupTime == "" {
			continue
		}
		parts = append(parts, dayLabelsDE[day]+" "+pickupTime)
	}
	return strings.Join(parts, "; ")
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
