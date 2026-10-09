package enrollmenthttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// phaseExportRequest is the (optional) JSON body:
// {"format":"pdf"|"docx"|"xlsx", "child_status":"approved"|…}. Missing/empty
// format defaults to PDF; missing/empty/"all" child_status means no
// status filter (export every child).
type phaseExportRequest struct {
	Format      lists.Format `json:"format"`
	ChildStatus string       `json:"child_status"`
}

// exportConfidentialityNote is stamped on enrollment exports because the
// files contain child data and, for phase exports, guardian contact data.
const exportConfidentialityNote = "Enthält personenbezogene Daten. Bitte nur intern verwenden."

// exportPhaseRegistrations streams a compact export of every
// registration in the phase. config:manage gated because one call bundles
// every guardian and child's full PII into a file that leaves the
// RLS-protected system. PDF and DOCX use readable child blocks grouped by
// status; XLSX keeps one table row per child with status group rows.
func (rs *Resource) exportPhaseRegistrations(w http.ResponseWriter, r *http.Request) {
	if rs.ListExportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("list export service not configured")))
		return
	}
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	phaseID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid phase id")
	if !ok {
		return
	}
	format, childStatus, err := parsePhaseExportRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Extract the actor for the GDPR audit row. A bulk PII export is a
	// disclosure event: ExportPhase loads the data AND records it on the
	// same tenant tx, and a failed audit write refuses the export (no
	// trail, no disclosure).
	claims := jwt.ClaimsFromCtx(r.Context())
	actorAccountID := int64(claims.ID)
	actorRole := strings.Join(claims.Roles, ",")

	// Load the PII + write the audit row inside the tenant tx so both
	// land under the correct tenant (RLS) and stay coupled. Rendering is
	// deliberately kept OUT of the tx: it is pure in-memory work, and a
	// large export should not hold a DB connection open while the file is
	// built. The "no disclosure without a trail" invariant survives the
	// move — a render failure after the audit row commits only
	// over-reports (the bytes never leave this handler on error), which
	// is the safe direction; it can never under-report an actual leak.
	var data *PhaseExport
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		d, e := rs.DecisionService.ExportPhase(ctx, phaseID, actorAccountID, actorRole, string(format), childStatus)
		if e != nil {
			return e
		}
		data = d
		return nil
	})
	if err != nil {
		renderPhaseExportError(w, r, err)
		return
	}

	file, err := buildPhaseExportFile(rs.ListExportService, data, format, childStatus)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	writeExportFile(w, file)
}

// renderPhaseExportError answers a failed phase export load.
func renderPhaseExportError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, capability.ErrPhaseNotFound) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentPhaseNotFound))
		return
	}
	// Phase too large to assemble in one in-memory file: a client-side
	// limit, not a server fault — surface it as a 400 with a clear
	// message rather than a 500.
	if errors.Is(err, capability.ErrExportTooLarge) {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(capability.ErrExportTooLarge, common.CodeEnrollmentExportTooLarge))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
}

// writeExportFile sends a rendered export as a download.
func writeExportFile(w http.ResponseWriter, file lists.File) {
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

// parsePhaseExportRequest reads the format and optional child-status
// filter from the JSON body (falling back to the ?format= / ?child_status=
// query params), reading the body exactly once. Format defaults to PDF
// (pdf/docx/xlsx only); child_status defaults to "" (no filter), with
// "all" also meaning no filter. A present-but-unknown status is rejected
// so a typo can't silently widen the export to everyone.
func parsePhaseExportRequest(r *http.Request) (lists.Format, string, error) {
	var body phaseExportRequest
	if r.Body != nil {
		if raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16)); len(raw) > 0 {
			// A present-but-malformed body is a client bug — surface it
			// rather than silently defaulting. An empty body is still fine
			// (falls through to the defaults below).
			if err := json.Unmarshal(raw, &body); err != nil {
				return "", "", fmt.Errorf("invalid export request body: %w", err)
			}
		}
	}
	// Normalise both sources to lower case so "PDF"/"XLSX" from either
	// the JSON body or the ?format= query param resolve identically.
	format := lists.Format(strings.ToLower(string(body.Format)))
	if format == "" {
		format = lists.Format(strings.ToLower(r.URL.Query().Get("format")))
	}
	if format == "" {
		format = lists.FormatPDF
	}
	switch format {
	case lists.FormatPDF, lists.FormatDOCX, lists.FormatXLSX:
		// ok
	default:
		return "", "", fmt.Errorf("unsupported export format %q (use pdf, docx or xlsx)", format)
	}

	childStatus := strings.ToLower(strings.TrimSpace(body.ChildStatus))
	if childStatus == "" {
		childStatus = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("child_status")))
	}
	if childStatus == "all" {
		childStatus = "" // explicit "all" == no filter
	}
	if childStatus != "" {
		// Validate against the known child statuses (the German-label map
		// is the authoritative key set) so an unknown value is a 400, not a
		// silent full export.
		if _, ok := statusLabelsDE[childStatus]; !ok {
			return "", "", fmt.Errorf("unsupported child_status %q", childStatus)
		}
	}
	return format, childStatus, nil
}

func buildPhaseExportFile(svc lists.DocumentRenderer, data *PhaseExport, format lists.Format, childStatus string) (lists.File, error) {
	// The document heading is the phase name on its own (no separator
	// punctuation); "Anmeldungen" lives in the subtitle counts + the
	// download filename. The filename base stays descriptive regardless.
	heading := phaseExportHeading(data)
	filename := phaseExportFilename(data)
	switch format {
	case lists.FormatDOCX:
		return svc.RenderRecordsDOCX(buildPhaseExportRecords(data, heading, childStatus), filename)
	case lists.FormatXLSX:
		return svc.Render(buildPhaseExportTable(data, heading, childStatus), lists.FormatXLSX, filename)
	case lists.FormatPDF:
		return svc.RenderRecords(buildPhaseExportRecords(data, heading, childStatus), filename)
	default:
		return lists.File{}, fmt.Errorf("unsupported export format %q", format)
	}
}

func (rs *Resource) exportStudentEnrollmentRequests(w http.ResponseWriter, r *http.Request) {
	if rs.ListExportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("list export service not configured")))
		return
	}
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	studentID, ok := common.ParsePositiveInt64IDWithError(w, r, "studentId", "invalid student id")
	if !ok {
		return
	}
	format, _, err := parsePhaseExportRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	actorAccountID := int64(claims.ID)
	actorRole := strings.Join(claims.Roles, ",")

	var data *StudentEnrollmentExport
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		d, e := rs.DecisionService.ExportStudent(ctx, studentID, actorAccountID, actorRole, string(format))
		if e != nil {
			return e
		}
		data = d
		return nil
	})
	if err != nil {
		if errors.Is(err, capability.ErrDecisionStudentNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		if errors.Is(err, capability.ErrExportTooLarge) {
			common.RenderError(w, r, common.ErrorInvalidRequestWithCode(capability.ErrExportTooLarge, common.CodeEnrollmentExportTooLarge))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	file, err := buildStudentEnrollmentExportFile(rs.ListExportService, data, format)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	writeExportFile(w, file)
}

func buildStudentEnrollmentExportFile(svc lists.DocumentRenderer, data *StudentEnrollmentExport, format lists.Format) (lists.File, error) {
	heading := studentEnrollmentExportHeading(data)
	filename := heading
	switch format {
	case lists.FormatDOCX:
		return svc.RenderRecordsDOCX(buildStudentEnrollmentExportRecords(data, heading), filename)
	case lists.FormatXLSX:
		return svc.Render(buildStudentEnrollmentExportTable(data, heading), lists.FormatXLSX, filename)
	case lists.FormatPDF:
		return svc.RenderRecords(buildStudentEnrollmentExportRecords(data, heading), filename)
	default:
		return lists.File{}, fmt.Errorf("unsupported export format %q", format)
	}
}

func studentEnrollmentExportHeading(data *StudentEnrollmentExport) string {
	if name := studentEnrollmentExportChildName(data); name != "" {
		return "Anmeldungen " + name
	}
	return "Anmeldungen"
}

func studentEnrollmentExportChildName(data *StudentEnrollmentExport) string {
	if data == nil {
		return ""
	}
	for _, row := range data.Rows {
		for _, child := range row.Children {
			if child.Child != nil {
				return childFullName(child.Child)
			}
		}
	}
	return ""
}

func studentEnrollmentExportSubtitle(data *StudentEnrollmentExport) string {
	requests, children := data.Counts()
	if requests == 1 {
		return fmt.Sprintf("%d Anmeldung, %d Kind", requests, children)
	}
	return fmt.Sprintf("%d Anmeldungen, %d Kinder", requests, children)
}

func buildStudentEnrollmentExportRecords(data *StudentEnrollmentExport, title string) lists.RecordDocument {
	guardianCustoms, childCustoms := collectCustomFields(data.Schemas)
	records := make([]lists.Record, 0, len(data.Rows))
	for _, row := range data.Rows {
		for _, child := range row.Children {
			records = append(records, studentEnrollmentRecord(row.Request, data.Phases[row.Request.PhaseID], child, guardianCustoms, childCustoms))
		}
	}
	return lists.RecordDocument{
		Title:       title,
		Subtitle:    studentEnrollmentExportSubtitle(data),
		GeneratedAt: time.Now(),
		Footer:      exportConfidentialityNote,
		Records:     records,
	}
}

func studentEnrollmentRecord(req *Request, phase *capability.Phase, ch ExportChildRow, guardianCustoms, childCustoms []capability.FormField) lists.Record {
	rec := lists.Record{
		Title:  childFullName(ch.Child),
		Fields: childFields(ch, childCustoms),
	}
	rec.Fields = append(rec.Fields,
		lists.Field{Label: "Anmeldung", Value: phaseNameForExport(phase)},
		lists.Field{Label: "Eingereicht am", Value: req.SubmittedAt.Format("02.01.2006 15:04")},
	)
	if req.WithdrawnAt != nil {
		rec.Fields = append(rec.Fields, lists.Field{Label: "Zurückgezogen am", Value: timeOrEmpty(req.WithdrawnAt)})
	}
	rec.Fields = append(rec.Fields, lists.Field{Label: "Zustimmungen", Value: consentSummary(req.ConsentFlags)})
	for _, cf := range guardianCustoms {
		if v, ok := req.CustomData[cf.Key]; ok {
			rec.Fields = append(rec.Fields, lists.Field{Label: cf.Label, Value: formatCustomValue(cf, v)})
		}
	}
	return rec
}

func phaseNameForExport(phase *capability.Phase) string {
	if phase == nil || strings.TrimSpace(phase.Name) == "" {
		return "Nicht zugeordnet"
	}
	return strings.TrimSpace(phase.Name)
}

func buildStudentEnrollmentExportTable(data *StudentEnrollmentExport, title string) lists.Document {
	guardianCustoms, childCustoms := collectCustomFields(data.Schemas)
	cols := []lists.Column{
		{ID: "phase", Label: "Anmeldung"},
		{ID: "submitted_at", Label: "Eingereicht am"},
		{ID: "withdrawn_at", Label: "Zurückgezogen am"},
		{ID: "child_last_name", Label: "Kind Nachname"},
		{ID: "child_first_name", Label: "Kind Vorname"},
		{ID: "child_dob", Label: "Geburtsdatum"},
		{ID: "child_grade", Label: "Zielklasse"},
		{ID: "child_status", Label: "Status"},
		{ID: "child_status_reason", Label: "Status-Grund"},
		{ID: "child_activation", Label: "Aktivierung"},
		{ID: "child_offerings", Label: "Betreuungsangebote"},
	}
	for _, cf := range childCustoms {
		cols = append(cols, lists.Column{ID: lists.ColumnID("custom_c_" + cf.Key), Label: cf.Label})
	}
	// Coupled accompanied-mode note (#1694): a reserved key, not a schema field,
	// so it needs its own dedicated column rather than a custom_c_ one.
	cols = append(cols, lists.Column{ID: "child_companion_note", Label: departureCompanionLabelDE})
	for _, cf := range guardianCustoms {
		cols = append(cols, lists.Column{ID: lists.ColumnID("custom_g_" + cf.Key), Label: cf.Label})
	}
	cols = append(cols,
		lists.Column{ID: "consent_agb", Label: "Zustimmung AGB"},
		lists.Column{ID: "consent_data_processing", Label: "Zustimmung Datenverarbeitung"},
		lists.Column{ID: "consent_email_contact", Label: "Zustimmung E-Mail-Kontakt"},
		lists.Column{ID: "consent_photo", Label: "Zustimmung Foto"},
	)

	rows := make([]lists.Row, 0, len(data.Rows))
	for _, row := range data.Rows {
		for _, child := range row.Children {
			rows = append(rows, lists.Row{
				Values: studentEnrollmentRowValues(row.Request, data.Phases[row.Request.PhaseID], child, guardianCustoms, childCustoms),
			})
		}
	}

	return lists.Document{
		Title:       title,
		Subtitle:    studentEnrollmentExportSubtitle(data),
		GeneratedAt: time.Now(),
		Columns:     cols,
		Rows:        rows,
		Footer:      exportConfidentialityNote,
	}
}

func studentEnrollmentRowValues(req *Request, phase *capability.Phase, ch ExportChildRow, guardianCustoms, childCustoms []capability.FormField) map[lists.ColumnID]string {
	values := map[lists.ColumnID]string{
		"phase":                   phaseNameForExport(phase),
		"submitted_at":            req.SubmittedAt.Format("02.01.2006 15:04"),
		"consent_agb":             consentLabel(req.ConsentFlags, capability.ConsentKeyAGB),
		"consent_data_processing": consentLabel(req.ConsentFlags, capability.ConsentKeyDataProcessing),
		"consent_email_contact":   consentLabel(req.ConsentFlags, capability.ConsentKeyEmailContact),
		"consent_photo":           consentLabel(req.ConsentFlags, capability.ConsentKeyPhoto),
	}
	if req.WithdrawnAt != nil {
		values["withdrawn_at"] = timeOrEmpty(req.WithdrawnAt)
	}
	for _, cf := range guardianCustoms {
		if v, ok := req.CustomData[cf.Key]; ok {
			values[lists.ColumnID("custom_g_"+cf.Key)] = formatCustomValue(cf, v)
		}
	}
	childValues := childRowValues(values, ch, childCustoms)
	return childValues
}

// enrollmentExportFilterLabels turns the applied child-status filter into
// the header label list (e.g. "Status: Angenommen"), mirroring the
// students export's Document.Filters convention. Empty filter → no label.
func enrollmentExportFilterLabels(childStatus string) []string {
	if childStatus == "" {
		return nil
	}
	return []string{"Status: " + statusLabelDE(childStatus)}
}

func phaseName(data *PhaseExport) string {
	if data != nil && data.Phase != nil {
		return strings.TrimSpace(data.Phase.Name)
	}
	return ""
}

func phaseExportHeading(data *PhaseExport) string {
	if name := phaseName(data); name != "" {
		return name
	}
	return "Anmeldungen"
}

func phaseExportFilename(data *PhaseExport) string {
	if name := phaseName(data); name != "" {
		return "Anmeldungen " + name
	}
	return "Anmeldungen"
}

func phaseExportSubtitle(data *PhaseExport) string {
	requests, children := data.Counts()
	return fmt.Sprintf("%d Anmeldungen, %d Kinder", requests, children)
}

// ---------------------------------------------------------------------
// XLSX: one flat row per child, every field its own column.
// ---------------------------------------------------------------------

func buildPhaseExportTable(data *PhaseExport, title, childStatus string) lists.Document {
	guardianCustoms, childCustoms := collectCustomFields(data.Schemas)
	return lists.Document{
		Title:       title,
		Subtitle:    phaseExportSubtitle(data),
		GeneratedAt: time.Now(),
		Filters:     enrollmentExportFilterLabels(childStatus),
		Columns:     phaseExportColumns(guardianCustoms, childCustoms),
		Rows:        phaseExportRows(data, guardianCustoms, childCustoms),
		Footer:      exportConfidentialityNote,
	}
}

// phaseExportColumns lists the XLSX columns of a phase export.
func phaseExportColumns(guardianCustoms, childCustoms []capability.FormField) []lists.Column {
	// Child-first column order, mirroring the PDF block: every child
	// column up front (the child is the subject), then the Eltern/contact
	// block. Row values are keyed by column ID, so only this slice's order
	// changes — guardianRowValues/childRowValues are untouched.
	cols := []lists.Column{
		{ID: "child_last_name", Label: "Kind Nachname"},
		{ID: "child_first_name", Label: "Kind Vorname"},
		{ID: "child_dob", Label: "Geburtsdatum"},
		{ID: "child_grade", Label: "Zielklasse"},
		{ID: "child_status", Label: "Status"},
		{ID: "child_status_reason", Label: "Status-Grund"},
		{ID: "child_activation", Label: "Aktivierung"},
		{ID: "child_offerings", Label: "Betreuungsangebote"},
	}
	for _, cf := range childCustoms {
		cols = append(cols, lists.Column{ID: lists.ColumnID("custom_c_" + cf.Key), Label: cf.Label})
	}
	// Coupled accompanied-mode note (#1694): a reserved key, not a schema field,
	// so it needs its own dedicated column rather than a custom_c_ one.
	cols = append(cols, lists.Column{ID: "child_companion_note", Label: departureCompanionLabelDE})
	// Eltern / contact block, after the child — same field order as the
	// PDF guardian block (name, contact, dates, custom answers, consents).
	cols = append(cols,
		lists.Column{ID: "guardian_last_name", Label: "Eltern Nachname"},
		lists.Column{ID: "guardian_first_name", Label: "Eltern Vorname"},
		lists.Column{ID: "guardian_email", Label: "E-Mail"},
		lists.Column{ID: "guardian_phone", Label: "Telefon"},
		lists.Column{ID: "submitted_at", Label: "Eingereicht am"},
		lists.Column{ID: "withdrawn_at", Label: "Zurückgezogen am"},
	)
	for _, cf := range guardianCustoms {
		cols = append(cols, lists.Column{ID: lists.ColumnID("custom_g_" + cf.Key), Label: cf.Label})
	}
	cols = append(cols,
		lists.Column{ID: "consent_agb", Label: "Zustimmung AGB"},
		lists.Column{ID: "consent_data_processing", Label: "Zustimmung Datenverarbeitung"},
		lists.Column{ID: "consent_email_contact", Label: "Zustimmung E-Mail-Kontakt"},
		lists.Column{ID: "consent_photo", Label: "Zustimmung Foto"},
		lists.Column{ID: "additional_guardians", Label: "Weitere Erziehungsberechtigte"},
	)
	return cols
}

// phaseExportRows lists the XLSX rows of a phase export.
func phaseExportRows(data *PhaseExport, guardianCustoms, childCustoms []capability.FormField) []lists.Row {
	// Same ordered entries as the PDF, so the XLSX rows appear in exactly
	// the same order as the PDF blocks (child surname A–Z). One row per
	// child; a childless registration emits a guardian-only row.
	rows := make([]lists.Row, 0, len(data.Rows))
	for _, group := range groupedExportEntries(data) {
		rows = append(rows, lists.Row{GroupTitle: group.title})
		for _, e := range group.entries {
			guardianValues := guardianRowValues(e.request, e.guardians, guardianCustoms)
			if e.child == nil {
				rows = append(rows, lists.Row{Values: guardianValues})
				continue
			}
			rows = append(rows, lists.Row{Values: childRowValues(guardianValues, *e.child, childCustoms)})
		}
	}
	return rows
}

func guardianRowValues(req *Request, guardians []*capability.RequestGuardian, guardianCustoms []capability.FormField) map[lists.ColumnID]string {
	values := map[lists.ColumnID]string{
		"guardian_last_name":      req.GuardianLastName,
		"guardian_first_name":     req.GuardianFirstName,
		"guardian_email":          req.GuardianEmail,
		"guardian_phone":          deref(req.GuardianPhone),
		"submitted_at":            req.SubmittedAt.Format("02.01.2006 15:04"),
		"withdrawn_at":            timeOrEmpty(req.WithdrawnAt),
		"consent_agb":             consentLabel(req.ConsentFlags, capability.ConsentKeyAGB),
		"consent_data_processing": consentLabel(req.ConsentFlags, capability.ConsentKeyDataProcessing),
		"consent_email_contact":   consentLabel(req.ConsentFlags, capability.ConsentKeyEmailContact),
		"consent_photo":           consentLabel(req.ConsentFlags, capability.ConsentKeyPhoto),
		"additional_guardians":    formatAdditionalGuardians(guardians),
	}
	for _, cf := range guardianCustoms {
		if v, ok := req.CustomData[cf.Key]; ok {
			values[lists.ColumnID("custom_g_"+cf.Key)] = formatCustomValue(cf, v)
		}
	}
	return values
}

func childRowValues(guardianValues map[lists.ColumnID]string, ch ExportChildRow, childCustoms []capability.FormField) map[lists.ColumnID]string {
	c := ch.Child
	values := cloneValues(guardianValues)
	values["child_first_name"] = c.FirstName
	values["child_last_name"] = c.LastName
	values["child_dob"] = calendar.Date(c.DateOfBirth).Format("02.01.2006")
	values["child_grade"] = schoolClassLabel(c.TargetSchoolClass, c.TargetGradeLevel)
	values["child_status"] = statusLabelDE(c.Status)
	values["child_status_reason"] = deref(c.StatusReason)
	values["child_activation"] = activationSummary(c)
	values["child_offerings"] = formatOfferings(ch.Offerings)
	for _, cf := range childCustoms {
		if v, ok := c.CustomData[cf.Key]; ok {
			values[lists.ColumnID("custom_c_"+cf.Key)] = formatCustomValue(cf, v)
		}
	}
	if note := childCompanionNote(c); note != "" {
		values["child_companion_note"] = note
	}
	return values
}

// collectCustomFields merges the custom fields across every pinned
// schema version of the phase, deduped by key and split into
// guardian-level vs child-level, each sorted by SortOrder. Reserved
// targets are still included — they live in custom_data too and the
// admin wants every answer in the export.
func collectCustomFields(schemas map[int64]*capability.FormSchema) (guardian, child []capability.FormField) {
	seenG := map[string]bool{}
	seenC := map[string]bool{}
	// Iterate schema versions newest-first (descending schema_id) so that
	// for a key defined in more than one version the CURRENT version's
	// label/type/sort-order wins — an admin who renames a field sees the
	// new label, not a stale one from an old pinned version. Descending is
	// as deterministic as ascending (a Go map range would pick a random
	// winner and produce differently-labelled columns between two exports
	// of the same phase); it just picks the more useful winner.
	schemaIDs := slices.Collect(maps.Keys(schemas))
	sort.Slice(schemaIDs, func(i, j int) bool { return schemaIDs[i] > schemaIDs[j] })
	for _, id := range schemaIDs {
		fs := schemas[id]
		if fs == nil {
			continue
		}
		for _, f := range fs.Fields {
			if f.Key == "" {
				continue
			}
			switch {
			case f.AppliesToCh && !seenC[f.Key]:
				seenC[f.Key] = true
				child = append(child, f)
			case !f.AppliesToCh && !seenG[f.Key]:
				seenG[f.Key] = true
				guardian = append(guardian, f)
			}
		}
	}
	sort.SliceStable(guardian, func(i, j int) bool { return guardian[i].SortOrder < guardian[j].SortOrder })
	sort.SliceStable(child, func(i, j int) bool { return child[i].SortOrder < child[j].SortOrder })
	return guardian, child
}

func guardianFullName(req *Request) string {
	name := strings.TrimSpace(req.GuardianFirstName + " " + req.GuardianLastName)
	if name == "" {
		return "Anmeldung"
	}
	return name
}
