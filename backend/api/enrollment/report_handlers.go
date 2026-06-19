package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

type careUsageExportRequest struct {
	Format  listexport.Format             `json:"format"`
	Filters careUsageExportFiltersRequest `json:"filters"`
}

type careUsageExportFiltersRequest struct {
	PhaseID         string   `json:"phase_id"`
	Status          string   `json:"status,omitempty"`
	CareOfferingID  string   `json:"care_offering_id,omitempty"`
	CareOfferingIDs []string `json:"care_offering_ids,omitempty"`
	DayCount        *int     `json:"day_count,omitempty"`
	GradeLevel      *int16   `json:"grade_level,omitempty"`
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
	CareOfferingIDs []string `json:"care_offering_ids,omitempty"`
	DayCount        *int     `json:"day_count,omitempty"`
	GradeLevel      *int16   `json:"grade_level,omitempty"`
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
	Status            string                         `json:"status"`
	Offerings         []careUsageRowOfferingResponse `json:"offerings"`
	EffectiveDays     []string                       `json:"effective_days"`
	DayCount          int                            `json:"day_count"`
	GuardianFirstName string                         `json:"guardian_first_name"`
	GuardianLastName  string                         `json:"guardian_last_name"`
	GuardianEmail     string                         `json:"guardian_email"`
	GuardianPhone     *string                        `json:"guardian_phone,omitempty"`
	SubmittedAt       time.Time                      `json:"submitted_at"`
}

type careUsageRowOfferingResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Days           []string `json:"days"`
	DaysSource     string   `json:"days_source"`
	DaysOfWeekMode string   `json:"days_of_week_mode"`
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

func (rs *Resource) exportCareUsageReport(w http.ResponseWriter, r *http.Request) {
	if rs.ReportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("report service not configured")))
		return
	}
	if rs.ListExportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("list export service not configured")))
		return
	}
	format, filters, err := parseCareUsageExportRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	actorAccountID := int64(claims.ID)
	actorRole := strings.Join(claims.Roles, ",")

	var report *enrollmentService.CareUsageReport
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		out, e := rs.ReportService.ExportCareUsage(ctx, filters, actorAccountID, actorRole, string(format))
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

	file, err := buildCareUsageExportFile(rs.ListExportService, report, format)
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

func parseCareUsageFiltersFromQuery(r *http.Request) (enrollmentService.CareUsageFilters, error) {
	q := r.URL.Query()
	var filters enrollmentService.CareUsageFilters
	phaseID, err := strconv.ParseInt(q.Get("phase_id"), 10, 64)
	if err != nil || phaseID <= 0 {
		return filters, errors.New("phase_id is required")
	}
	filters.PhaseID = phaseID
	filters.Status = q.Get("status")
	filters.Search = q.Get("search")
	offeringIDs, err := parseCareOfferingIDsFromQuery(q["care_offering_ids"])
	if err != nil {
		return filters, err
	}
	if raw := q.Get("care_offering_id"); raw != "" {
		id, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || id <= 0 {
			return filters, errors.New("care_offering_id must be positive")
		}
		offeringIDs = append(offeringIDs, id)
	}
	filters.CareOfferingIDs = offeringIDs
	if raw := q.Get("day_count"); raw != "" {
		count, parseErr := strconv.Atoi(raw)
		if parseErr != nil || count < 0 || count > 7 {
			return filters, errors.New("day_count must be between 0 and 7")
		}
		filters.DayCount = &count
	}
	if raw := q.Get("grade_level"); raw != "" {
		grade, parseErr := strconv.ParseInt(raw, 10, 16)
		if parseErr != nil || grade <= 0 {
			return filters, errors.New("grade_level must be positive")
		}
		g := int16(grade)
		filters.GradeLevel = &g
	}
	return filters, nil
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
		Search:     req.Search,
	}
	if strings.TrimSpace(req.CareOfferingID) != "" {
		careOfferingID, err := parseRequiredPositiveInt64(req.CareOfferingID, "filters.care_offering_id")
		if err != nil {
			return enrollmentService.CareUsageFilters{}, err
		}
		filters.CareOfferingIDs = append(filters.CareOfferingIDs, careOfferingID)
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
			Search:     report.Filters.Search,
		},
		Totals: report.Totals,
		FilterOptions: careUsageFilterOptionsResponse{
			GradeLevels: report.FilterOptions.GradeLevels,
		},
		Rows: make([]careUsageRowResponse, 0, len(report.Rows)),
	}
	if len(report.Filters.CareOfferingIDs) > 0 {
		out.Filters.CareOfferingIDs = make([]string, 0, len(report.Filters.CareOfferingIDs))
		for _, id := range report.Filters.CareOfferingIDs {
			out.Filters.CareOfferingIDs = append(out.Filters.CareOfferingIDs, strconv.FormatInt(id, 10))
		}
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
			Status:            row.Status,
			EffectiveDays:     nonNilStringSlice(row.EffectiveDays),
			DayCount:          row.DayCount,
			GuardianFirstName: row.GuardianFirstName,
			GuardianLastName:  row.GuardianLastName,
			GuardianEmail:     row.GuardianEmail,
			GuardianPhone:     row.GuardianPhone,
			SubmittedAt:       row.SubmittedAt,
			Offerings:         make([]careUsageRowOfferingResponse, 0, len(row.Offerings)),
		}
		for _, offering := range row.Offerings {
			rowOut.Offerings = append(rowOut.Offerings, careUsageRowOfferingResponse{
				ID:             strconv.FormatInt(offering.ID, 10),
				Name:           offering.Name,
				Days:           nonNilStringSlice(offering.Days),
				DaysSource:     offering.DaysSource,
				DaysOfWeekMode: offering.DaysOfWeekMode,
			})
		}
		out.Rows = append(out.Rows, rowOut)
	}
	return out
}

func buildCareUsageExportFile(svc listexport.Service, report *enrollmentService.CareUsageReport, format listexport.Format) (listexport.File, error) {
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
		{ID: "guardian_name", Label: "Eltern"},
		{ID: "guardian_email", Label: "E-Mail"},
		{ID: "guardian_phone", Label: "Telefon"},
	}
	rows := make([]listexport.Row, 0, len(report.Rows))
	for _, row := range report.Rows {
		rows = append(rows, listexport.Row{Values: map[listexport.ColumnID]string{
			"child_last_name":  row.ChildLastName,
			"child_first_name": row.ChildFirstName,
			"child_grade":      gradeLabel(row.TargetGradeLevel),
			"child_status":     statusLabelDE(row.Status),
			"offerings":        careUsageOfferingNames(row.Offerings),
			"offering_days":    careUsageOfferingDayDetails(row.Offerings),
			"effective_days":   formatDayCodes(row.EffectiveDays),
			"day_count":        strconv.Itoa(row.DayCount),
			"guardian_name":    strings.TrimSpace(row.GuardianFirstName + " " + row.GuardianLastName),
			"guardian_email":   row.GuardianEmail,
			"guardian_phone":   strOrEmpty(row.GuardianPhone),
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
		Title:  "Statistik",
		Fields: careUsageDayCountFields(report),
	})
	for _, row := range report.Rows {
		records = append(records, listexport.Record{
			Title: strings.TrimSpace(row.ChildFirstName + " " + row.ChildLastName),
			Fields: []listexport.Field{
				{Label: "Zielklasse", Value: gradeLabel(row.TargetGradeLevel)},
				{Label: "Status", Value: statusLabelDE(row.Status)},
				{Label: "Betreuungsangebote", Value: careUsageOfferingDayDetails(row.Offerings)},
				{Label: "Effektive Betreuungstage", Value: formatDayCodes(row.EffectiveDays)},
				{Label: "Anzahl Tage", Value: strconv.Itoa(row.DayCount)},
				{Label: "Eltern", Value: strings.TrimSpace(row.GuardianFirstName + " " + row.GuardianLastName)},
				{Label: "E-Mail", Value: row.GuardianEmail},
				{Label: "Telefon", Value: strOrEmpty(row.GuardianPhone)},
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
		days := formatDayCodes(offering.Days)
		if days == "" {
			parts = append(parts, offering.Name)
			continue
		}
		parts = append(parts, offering.Name+" ("+days+")")
	}
	return strings.Join(parts, "; ")
}

func careUsageDayCountFields(report *enrollmentService.CareUsageReport) []listexport.Field {
	fields := []listexport.Field{{Label: "Kinder", Value: strconv.Itoa(report.Totals.Children)}}
	for _, count := range sortedDayCountKeys(report.Totals.ByDayCount) {
		fields = append(fields, listexport.Field{
			Label: dayCountLabelDE(count),
			Value: strconv.Itoa(report.Totals.ByDayCount[strconv.Itoa(count)]),
		})
	}
	return fields
}

func sortedDayCountKeys(counts map[string]int) []int {
	out := make([]int, 0, len(counts))
	seen := make(map[int]bool, len(counts))
	for raw := range counts {
		count, err := strconv.Atoi(raw)
		if err != nil || seen[count] {
			continue
		}
		seen[count] = true
		out = append(out, count)
	}
	sort.Ints(out)
	return out
}

func dayCountLabelDE(count int) string {
	if count == 1 {
		return "1 Tag"
	}
	return fmt.Sprintf("%d Tage", count)
}

func nonNilStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
