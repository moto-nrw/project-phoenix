package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

type careUsageExportRequest struct {
	Format  listexport.Format                  `json:"format"`
	Filters enrollmentService.CareUsageFilters `json:"filters"`
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
	common.Respond(w, r, http.StatusOK, report, "Care usage report retrieved")
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
	if raw := q.Get("care_offering_id"); raw != "" {
		id, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || id <= 0 {
			return filters, errors.New("care_offering_id must be positive")
		}
		filters.CareOfferingID = id
	}
	if raw := q.Get("day_count"); raw != "" {
		count, parseErr := strconv.Atoi(raw)
		if parseErr != nil || count <= 0 {
			return filters, errors.New("day_count must be positive")
		}
		filters.DayCount = count
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
	case listexport.FormatPDF, listexport.FormatXLSX:
	default:
		return "", enrollmentService.CareUsageFilters{}, fmt.Errorf("unsupported export format %q (use pdf or xlsx)", format)
	}
	if body.Filters.PhaseID <= 0 {
		return "", enrollmentService.CareUsageFilters{}, errors.New("filters.phase_id is required")
	}
	return format, body.Filters, nil
}

func buildCareUsageExportFile(svc listexport.Service, report *enrollmentService.CareUsageReport, format listexport.Format) (listexport.File, error) {
	filename := "Anmelde-Auswertung " + strings.TrimSpace(report.Phase.Name)
	if strings.TrimSpace(report.Phase.Name) == "" {
		filename = "Anmelde-Auswertung"
	}
	switch format {
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
		Title: "Statistik",
		Fields: []listexport.Field{
			{Label: "Kinder", Value: strconv.Itoa(report.Totals.Children)},
			{Label: "1 Tag", Value: strconv.Itoa(report.Totals.ByDayCount["1"])},
			{Label: "2 Tage", Value: strconv.Itoa(report.Totals.ByDayCount["2"])},
			{Label: "3 Tage", Value: strconv.Itoa(report.Totals.ByDayCount["3"])},
			{Label: "4 Tage", Value: strconv.Itoa(report.Totals.ByDayCount["4"])},
			{Label: "5 Tage", Value: strconv.Itoa(report.Totals.ByDayCount["5"])},
		},
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
	if report.Filters.CareOfferingID > 0 {
		labels = append(labels, "Betreuungsangebot: "+careUsageOfferingNameByID(report, report.Filters.CareOfferingID))
	}
	if report.Filters.DayCount > 0 {
		labels = append(labels, fmt.Sprintf("Tage: %d", report.Filters.DayCount))
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
