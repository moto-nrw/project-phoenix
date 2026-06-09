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

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// phaseExportRequest is the (optional) JSON body:
// {"format":"pdf"|"docx"|"xlsx", "child_status":"approved"|…}. Missing/empty
// format defaults to PDF; missing/empty/"all" child_status means no
// status filter (export every child).
type phaseExportRequest struct {
	Format      listexport.Format `json:"format"`
	ChildStatus string            `json:"child_status"`
}

// exportConfidentialityNote is stamped on printed phase exports because
// the files contain full guardian and child PII.
const exportConfidentialityNote = "Vertraulich, nur für berechtigte Personen. Nach Gebrauch sicher vernichten."

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
	phaseID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || phaseID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid phase id")))
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
	var data *enrollmentService.PhaseExport
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		d, e := rs.DecisionService.ExportPhase(ctx, phaseID, actorAccountID, actorRole, string(format), childStatus)
		if e != nil {
			return e
		}
		data = d
		return nil
	})
	if err != nil {
		if errors.Is(err, enrollmentService.ErrPhaseNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		// Phase too large to assemble in one in-memory file: a client-side
		// limit, not a server fault — surface it as a 400 with a clear
		// message rather than a 500.
		if errors.Is(err, enrollmentService.ErrExportTooLarge) {
			common.RenderError(w, r, common.ErrorInvalidRequest(enrollmentService.ErrExportTooLarge))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	file, err := buildPhaseExportFile(rs.ListExportService, data, format, childStatus)
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

// parsePhaseExportRequest reads the format and optional child-status
// filter from the JSON body (falling back to the ?format= / ?child_status=
// query params), reading the body exactly once. Format defaults to PDF
// (pdf/docx/xlsx only); child_status defaults to "" (no filter), with
// "all" also meaning no filter. A present-but-unknown status is rejected
// so a typo can't silently widen the export to everyone.
func parsePhaseExportRequest(r *http.Request) (listexport.Format, string, error) {
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
	format := listexport.Format(strings.ToLower(string(body.Format)))
	if format == "" {
		format = listexport.Format(strings.ToLower(r.URL.Query().Get("format")))
	}
	if format == "" {
		format = listexport.FormatPDF
	}
	switch format {
	case listexport.FormatPDF, listexport.FormatDOCX, listexport.FormatXLSX:
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

func buildPhaseExportFile(svc listexport.Service, data *enrollmentService.PhaseExport, format listexport.Format, childStatus string) (listexport.File, error) {
	// The document heading is the phase name on its own (no separator
	// punctuation); "Anmeldungen" lives in the subtitle counts + the
	// download filename. The filename base stays descriptive regardless.
	heading := phaseExportHeading(data)
	filename := phaseExportFilename(data)
	switch format {
	case listexport.FormatDOCX:
		return svc.RenderRecordsDOCX(buildPhaseExportRecords(data, heading, childStatus), filename)
	case listexport.FormatXLSX:
		return svc.Render(buildPhaseExportTable(data, heading, childStatus), listexport.FormatXLSX, filename)
	case listexport.FormatPDF:
		return svc.RenderRecords(buildPhaseExportRecords(data, heading, childStatus), filename)
	default:
		return listexport.File{}, fmt.Errorf("unsupported export format %q", format)
	}
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

func phaseName(data *enrollmentService.PhaseExport) string {
	if data != nil && data.Phase != nil {
		return strings.TrimSpace(data.Phase.Name)
	}
	return ""
}

func phaseExportHeading(data *enrollmentService.PhaseExport) string {
	if name := phaseName(data); name != "" {
		return name
	}
	return "Anmeldungen"
}

func phaseExportFilename(data *enrollmentService.PhaseExport) string {
	if name := phaseName(data); name != "" {
		return "Anmeldungen " + name
	}
	return "Anmeldungen"
}

func phaseExportSubtitle(data *enrollmentService.PhaseExport) string {
	requests, children := data.Counts()
	return fmt.Sprintf("%d Anmeldungen, %d Kinder", requests, children)
}

// ---------------------------------------------------------------------
// PDF: one block per child (child-first). The child is the subject of
// each block; the guardian/contact details are repeated on every block
// so it is self-contained — the print fallback is used to find a child
// and immediately see who may collect them in a WLAN/system outage.
// ---------------------------------------------------------------------

// exportEntry is one ordered output unit: a single child together with
// its registration, or a childless registration (child == nil). It is
// the shared row/block unit for BOTH the PDF and the XLSX so the two
// formats list registrations in exactly the same order.
type exportEntry struct {
	request   *enrollmentModels.Request
	child     *enrollmentService.ExportChildRow // nil = registration has no children
	sortLast  string                            // lower-cased sort key (child surname, or guardian surname when childless)
	sortFirst string
}

type exportEntryGroup struct {
	title   string
	entries []exportEntry
}

var enrollmentStatusExportGroups = []struct {
	status string
	title  string
}{
	{enrollmentModels.ChildStatusApproved, "Bestätigte Anmeldungen"},
	{enrollmentModels.ChildStatusRejected, "Abgelehnte Anmeldungen"},
	{enrollmentModels.ChildStatusWaitlisted, "Warteliste"},
	{enrollmentModels.ChildStatusSubmitted, "Eingegangene Anmeldungen"},
	{enrollmentModels.ChildStatusUnderReview, "Anmeldungen in Prüfung"},
	{enrollmentModels.ChildStatusPendingAdminReview, "Manuelle Prüfung"},
	{enrollmentModels.ChildStatusPendingRenewal, "Ausstehende Verlängerungen"},
	{enrollmentModels.ChildStatusAutoRenewed, "Automatisch verlängerte Anmeldungen"},
	{enrollmentModels.ChildStatusWithdrawn, "Zurückgezogene Anmeldungen"},
}

// orderedExportEntries flattens the phase into one entry per child (plus
// one per childless registration) and orders them by child surname then
// first name, case-insensitively; childless entries sort by guardian
// name. This is the single source of truth for export ordering — both
// buildPhaseExportRecords (PDF) and buildPhaseExportTable (XLSX) iterate
// the same slice, so PDF blocks and XLSX rows can never drift apart.
func orderedExportEntries(data *enrollmentService.PhaseExport) []exportEntry {
	entries := make([]exportEntry, 0, len(data.Rows))
	for _, row := range data.Rows {
		if len(row.Children) == 0 {
			entries = append(entries, exportEntry{
				request:   row.Request,
				sortLast:  strings.ToLower(strings.TrimSpace(row.Request.GuardianLastName)),
				sortFirst: strings.ToLower(strings.TrimSpace(row.Request.GuardianFirstName)),
			})
			continue
		}
		for i := range row.Children {
			ch := row.Children[i]
			entries = append(entries, exportEntry{
				request:   row.Request,
				child:     &ch,
				sortLast:  strings.ToLower(strings.TrimSpace(ch.Child.LastName)),
				sortFirst: strings.ToLower(strings.TrimSpace(ch.Child.FirstName)),
			})
		}
	}
	// Stable so children with identical names keep their submission order.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].sortLast != entries[j].sortLast {
			return entries[i].sortLast < entries[j].sortLast
		}
		return entries[i].sortFirst < entries[j].sortFirst
	})
	return entries
}

func groupedExportEntries(data *enrollmentService.PhaseExport) []exportEntryGroup {
	entries := orderedExportEntries(data)
	buckets := make(map[string][]exportEntry)
	titles := make(map[string]string)
	for _, e := range entries {
		key, title := exportEntryGroupKey(e)
		buckets[key] = append(buckets[key], e)
		titles[key] = title
	}

	groups := make([]exportEntryGroup, 0, len(buckets))
	seen := make(map[string]bool, len(buckets))
	for _, def := range enrollmentStatusExportGroups {
		if entries := buckets[def.status]; len(entries) > 0 {
			groups = append(groups, exportEntryGroup{title: def.title, entries: entries})
			seen[def.status] = true
		}
	}
	for _, key := range sortedRemainingExportGroupKeys(titles, seen) {
		if entries := buckets[key]; len(entries) > 0 {
			groups = append(groups, exportEntryGroup{title: titles[key], entries: entries})
		}
	}
	return groups
}

func exportEntryGroupKey(e exportEntry) (string, string) {
	if e.child == nil || e.child.Child == nil {
		return "__childless__", "Anmeldungen ohne Kind"
	}
	status := strings.TrimSpace(e.child.Child.Status)
	if status == "" {
		return "__unknown_status__", "Weitere Anmeldungen"
	}
	return status, enrollmentStatusExportTitle(status)
}

func enrollmentStatusExportTitle(status string) string {
	for _, def := range enrollmentStatusExportGroups {
		if def.status == status {
			return def.title
		}
	}
	label := strings.TrimSpace(statusLabelDE(status))
	if label == "" {
		return "Weitere Anmeldungen"
	}
	return label
}

func sortedRemainingExportGroupKeys(titles map[string]string, seen map[string]bool) []string {
	keys := make([]string, 0, len(titles))
	for key := range titles {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if titles[keys[i]] != titles[keys[j]] {
			return titles[keys[i]] < titles[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

func buildPhaseExportRecords(data *enrollmentService.PhaseExport, title, childStatus string) listexport.RecordDocument {
	guardianCustoms, childCustoms := collectCustomFields(data.Schemas)

	// One block per child (child-first), in the shared export order. A
	// childless registration still surfaces as a guardian-only block so
	// no submission is silently dropped.
	records := make([]listexport.Record, 0, len(data.Rows))
	groups := make([]listexport.RecordGroup, 0, len(data.Rows))
	for _, group := range groupedExportEntries(data) {
		groupRecords := make([]listexport.Record, 0, len(group.entries))
		for _, e := range group.entries {
			var record listexport.Record
			if e.child == nil {
				record = guardianOnlyRecord(e.request, guardianCustoms)
			} else {
				record = childRecord(e.request, *e.child, guardianCustoms, childCustoms)
			}
			groupRecords = append(groupRecords, record)
			records = append(records, record)
		}
		if len(groupRecords) > 0 {
			groups = append(groups, listexport.RecordGroup{Title: group.title, Records: groupRecords})
		}
	}
	return listexport.RecordDocument{
		Title:       title,
		Subtitle:    phaseExportSubtitle(data),
		GeneratedAt: time.Now(),
		Footer:      exportConfidentialityNote,
		Filters:     enrollmentExportFilterLabels(childStatus),
		Records:     records,
		Groups:      groups,
	}
}

func guardianBlockFields(req *enrollmentModels.Request, guardianCustoms []enrollmentModels.FormField) []listexport.Field {
	fields := []listexport.Field{
		{Label: "E-Mail", Value: req.GuardianEmail},
		{Label: "Telefon", Value: strOrEmpty(req.GuardianPhone)},
		{Label: "Eingereicht am", Value: req.SubmittedAt.Format("02.01.2006 15:04")},
	}
	if req.WithdrawnAt != nil {
		fields = append(fields, listexport.Field{Label: "Zurückgezogen am", Value: timeOrEmpty(req.WithdrawnAt)})
	}
	fields = append(fields, listexport.Field{Label: "Zustimmungen", Value: consentSummary(req.ConsentFlags)})
	for _, cf := range guardianCustoms {
		if v, ok := req.CustomData[cf.Key]; ok {
			fields = append(fields, listexport.Field{Label: cf.Label, Value: formatCustomValue(cf, v)})
		}
	}
	return fields
}

// childRecord renders one child as a standalone block: the child's own
// fields first (the child is the subject), then the guardian/contact
// details led by the parent name, repeated on every child so a single
// block holds everything a supervisor needs offline.
func childRecord(req *enrollmentModels.Request, ch enrollmentService.ExportChildRow, guardianCustoms, childCustoms []enrollmentModels.FormField) listexport.Record {
	rec := listexport.Record{
		Title:  childFullName(ch.Child),
		Fields: childFields(ch, childCustoms),
	}
	rec.Fields = append(rec.Fields, listexport.Field{Label: "Eltern", Value: guardianFullName(req)})
	rec.Fields = append(rec.Fields, guardianBlockFields(req, guardianCustoms)...)
	return rec
}

// guardianOnlyRecord is the fallback block for a registration with no
// child rows — keeps the guardian (and their answers) in the export
// rather than dropping the submission entirely.
func guardianOnlyRecord(req *enrollmentModels.Request, guardianCustoms []enrollmentModels.FormField) listexport.Record {
	return listexport.Record{
		Title:  guardianFullName(req),
		Fields: guardianBlockFields(req, guardianCustoms),
	}
}

// childFields builds the child's own label/value lines: identity, target
// class, status, activation, offerings, then per-child custom answers.
func childFields(ch enrollmentService.ExportChildRow, childCustoms []enrollmentModels.FormField) []listexport.Field {
	c := ch.Child
	fields := []listexport.Field{
		{Label: "Geburtsdatum", Value: c.DateOfBirth.Format("02.01.2006")},
		{Label: "Zielklasse", Value: gradeLabel(c.TargetGradeLevel)},
		{Label: "Status", Value: statusLabelDE(c.Status)},
	}
	if c.StatusReason != nil && strings.TrimSpace(*c.StatusReason) != "" {
		fields = append(fields, listexport.Field{Label: "Status-Grund", Value: *c.StatusReason})
	}
	fields = append(fields,
		listexport.Field{Label: "Aktivierung", Value: activationSummary(c)},
		listexport.Field{Label: "Betreuungsangebote", Value: formatOfferings(ch.Offerings)},
	)
	for _, cf := range childCustoms {
		if v, ok := c.CustomData[cf.Key]; ok {
			fields = append(fields, listexport.Field{Label: cf.Label, Value: formatCustomValue(cf, v)})
		}
	}
	return fields
}

// childFullName is the child's display heading for its block.
func childFullName(c *enrollmentModels.RequestChild) string {
	name := strings.TrimSpace(c.FirstName + " " + c.LastName)
	if name == "" {
		return "Kind"
	}
	return name
}

// ---------------------------------------------------------------------
// XLSX: one flat row per child, every field its own column.
// ---------------------------------------------------------------------

func buildPhaseExportTable(data *enrollmentService.PhaseExport, title, childStatus string) listexport.Document {
	guardianCustoms, childCustoms := collectCustomFields(data.Schemas)

	// Child-first column order, mirroring the PDF block: every child
	// column up front (the child is the subject), then the Eltern/contact
	// block. Row values are keyed by column ID, so only this slice's order
	// changes — guardianRowValues/childRowValues are untouched.
	cols := []listexport.Column{
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
		cols = append(cols, listexport.Column{ID: listexport.ColumnID("custom_c_" + cf.Key), Label: cf.Label})
	}
	// Eltern / contact block, after the child — same field order as the
	// PDF guardian block (name, contact, dates, custom answers, consents).
	cols = append(cols,
		listexport.Column{ID: "guardian_last_name", Label: "Eltern Nachname"},
		listexport.Column{ID: "guardian_first_name", Label: "Eltern Vorname"},
		listexport.Column{ID: "guardian_email", Label: "E-Mail"},
		listexport.Column{ID: "guardian_phone", Label: "Telefon"},
		listexport.Column{ID: "submitted_at", Label: "Eingereicht am"},
		listexport.Column{ID: "withdrawn_at", Label: "Zurückgezogen am"},
	)
	for _, cf := range guardianCustoms {
		cols = append(cols, listexport.Column{ID: listexport.ColumnID("custom_g_" + cf.Key), Label: cf.Label})
	}
	cols = append(cols,
		listexport.Column{ID: "consent_agb", Label: "Zustimmung AGB"},
		listexport.Column{ID: "consent_data_processing", Label: "Zustimmung Datenverarbeitung"},
		listexport.Column{ID: "consent_email_contact", Label: "Zustimmung E-Mail-Kontakt"},
		listexport.Column{ID: "consent_photo", Label: "Zustimmung Foto"},
	)

	// Same ordered entries as the PDF, so the XLSX rows appear in exactly
	// the same order as the PDF blocks (child surname A–Z). One row per
	// child; a childless registration emits a guardian-only row.
	rows := make([]listexport.Row, 0, len(data.Rows))
	for _, group := range groupedExportEntries(data) {
		rows = append(rows, listexport.Row{GroupTitle: group.title})
		for _, e := range group.entries {
			guardianValues := guardianRowValues(e.request, guardianCustoms)
			if e.child == nil {
				rows = append(rows, listexport.Row{Values: guardianValues})
				continue
			}
			rows = append(rows, listexport.Row{Values: childRowValues(guardianValues, *e.child, childCustoms)})
		}
	}

	return listexport.Document{
		Title:       title,
		Subtitle:    phaseExportSubtitle(data),
		GeneratedAt: time.Now(),
		Filters:     enrollmentExportFilterLabels(childStatus),
		Columns:     cols,
		Rows:        rows,
		Footer:      exportConfidentialityNote,
	}
}

func guardianRowValues(req *enrollmentModels.Request, guardianCustoms []enrollmentModels.FormField) map[listexport.ColumnID]string {
	values := map[listexport.ColumnID]string{
		"guardian_last_name":      req.GuardianLastName,
		"guardian_first_name":     req.GuardianFirstName,
		"guardian_email":          req.GuardianEmail,
		"guardian_phone":          strOrEmpty(req.GuardianPhone),
		"submitted_at":            req.SubmittedAt.Format("02.01.2006 15:04"),
		"withdrawn_at":            timeOrEmpty(req.WithdrawnAt),
		"consent_agb":             consentLabel(req.ConsentFlags, enrollmentModels.ConsentKeyAGB),
		"consent_data_processing": consentLabel(req.ConsentFlags, enrollmentModels.ConsentKeyDataProcessing),
		"consent_email_contact":   consentLabel(req.ConsentFlags, enrollmentModels.ConsentKeyEmailContact),
		"consent_photo":           consentLabel(req.ConsentFlags, enrollmentModels.ConsentKeyPhoto),
	}
	for _, cf := range guardianCustoms {
		if v, ok := req.CustomData[cf.Key]; ok {
			values[listexport.ColumnID("custom_g_"+cf.Key)] = formatCustomValue(cf, v)
		}
	}
	return values
}

func childRowValues(guardianValues map[listexport.ColumnID]string, ch enrollmentService.ExportChildRow, childCustoms []enrollmentModels.FormField) map[listexport.ColumnID]string {
	c := ch.Child
	values := cloneValues(guardianValues)
	values["child_first_name"] = c.FirstName
	values["child_last_name"] = c.LastName
	values["child_dob"] = c.DateOfBirth.Format("02.01.2006")
	values["child_grade"] = gradeLabel(c.TargetGradeLevel)
	values["child_status"] = statusLabelDE(c.Status)
	values["child_status_reason"] = strOrEmpty(c.StatusReason)
	values["child_activation"] = activationSummary(c)
	values["child_offerings"] = formatOfferings(ch.Offerings)
	for _, cf := range childCustoms {
		if v, ok := c.CustomData[cf.Key]; ok {
			values[listexport.ColumnID("custom_c_"+cf.Key)] = formatCustomValue(cf, v)
		}
	}
	return values
}

// collectCustomFields merges the custom fields across every pinned
// schema version of the phase, deduped by key and split into
// guardian-level vs child-level, each sorted by SortOrder. Reserved
// targets are still included — they live in custom_data too and the
// admin wants every answer in the export.
func collectCustomFields(schemas map[int64]*enrollmentModels.FormSchema) (guardian, child []enrollmentModels.FormField) {
	seenG := map[string]bool{}
	seenC := map[string]bool{}
	// Iterate schema versions newest-first (descending schema_id) so that
	// for a key defined in more than one version the CURRENT version's
	// label/type/sort-order wins — an admin who renames a field sees the
	// new label, not a stale one from an old pinned version. Descending is
	// as deterministic as ascending (a Go map range would pick a random
	// winner and produce differently-labelled columns between two exports
	// of the same phase); it just picks the more useful winner.
	schemaIDs := make([]int64, 0, len(schemas))
	for id := range schemas {
		schemaIDs = append(schemaIDs, id)
	}
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

func guardianFullName(req *enrollmentModels.Request) string {
	name := strings.TrimSpace(req.GuardianFirstName + " " + req.GuardianLastName)
	if name == "" {
		return "Anmeldung"
	}
	return name
}
