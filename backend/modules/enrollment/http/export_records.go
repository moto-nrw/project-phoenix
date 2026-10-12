package enrollmenthttp

import (
	"sort"
	"strings"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
)

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
	request   *Request
	child     *ExportChildRow               // nil = registration has no children
	guardians []*capability.RequestGuardian // co-guardians of this submission (nil when none)
	sortLast  string                        // lower-cased sort key (child surname, or guardian surname when childless)
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
	{capability.ChildStatusApproved, "Bestätigte Anmeldungen"},
	{capability.ChildStatusRejected, "Abgelehnte Anmeldungen"},
	{capability.ChildStatusWaitlisted, "Warteliste"},
	{capability.ChildStatusSubmitted, "Eingegangene Anmeldungen"},
	{capability.ChildStatusUnderReview, "Anmeldungen in Prüfung"},
	{capability.ChildStatusPendingAdminReview, "Manuelle Prüfung"},
	{capability.ChildStatusPendingRenewal, "Ausstehende Verlängerungen"},
	{capability.ChildStatusAutoRenewed, "Automatisch verlängerte Anmeldungen"},
	{capability.ChildStatusWithdrawn, "Zurückgezogene Anmeldungen"},
}

// orderedExportEntries flattens the phase into one entry per child (plus
// one per childless registration) and orders them by child surname then
// first name, case-insensitively; childless entries sort by guardian
// name. This is the single source of truth for export ordering — both
// buildPhaseExportRecords (PDF) and buildPhaseExportTable (XLSX) iterate
// the same slice, so PDF blocks and XLSX rows can never drift apart.
func orderedExportEntries(data *PhaseExport) []exportEntry {
	entries := make([]exportEntry, 0, len(data.Rows))
	for _, row := range data.Rows {
		if len(row.Children) == 0 {
			entries = append(entries, exportEntry{
				request:   row.Request,
				guardians: row.Guardians,
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
				guardians: row.Guardians,
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

func groupedExportEntries(data *PhaseExport) []exportEntryGroup {
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

func buildPhaseExportRecords(data *PhaseExport, title, childStatus string) lists.RecordDocument {
	guardianCustoms, childCustoms := collectCustomFields(data.Schemas)

	// One block per child (child-first), in the shared export order. A
	// childless registration still surfaces as a guardian-only block so
	// no submission is silently dropped.
	records := make([]lists.Record, 0, len(data.Rows))
	groups := make([]lists.RecordGroup, 0, len(data.Rows))
	for _, group := range groupedExportEntries(data) {
		groupRecords := make([]lists.Record, 0, len(group.entries))
		for _, e := range group.entries {
			var record lists.Record
			if e.child == nil {
				record = guardianOnlyRecord(e.request, e.guardians, guardianCustoms)
			} else {
				record = childRecord(e.request, *e.child, e.guardians, guardianCustoms, childCustoms)
			}
			groupRecords = append(groupRecords, record)
			records = append(records, record)
		}
		if len(groupRecords) > 0 {
			groups = append(groups, lists.RecordGroup{Title: group.title, Records: groupRecords})
		}
	}
	return lists.RecordDocument{
		Title:       title,
		Subtitle:    phaseExportSubtitle(data),
		GeneratedAt: time.Now(),
		Footer:      exportConfidentialityNote,
		Filters:     enrollmentExportFilterLabels(childStatus),
		Records:     records,
		Groups:      groups,
	}
}

func guardianBlockFields(req *Request, guardianCustoms []capability.FormField) []lists.Field {
	fields := []lists.Field{
		{Label: "E-Mail", Value: req.GuardianEmail},
		{Label: "Telefon", Value: deref(req.GuardianPhone)},
		{Label: "Eingereicht am", Value: req.SubmittedAt.Format("02.01.2006 15:04")},
	}
	if req.WithdrawnAt != nil {
		fields = append(fields, lists.Field{Label: "Zurückgezogen am", Value: timeOrEmpty(req.WithdrawnAt)})
	}
	fields = append(fields, lists.Field{Label: "Zustimmungen", Value: consentSummary(req.ConsentFlags)})
	for _, cf := range guardianCustoms {
		if v, ok := req.CustomData[cf.Key]; ok {
			fields = append(fields, lists.Field{Label: cf.Label, Value: formatCustomValue(cf, v)})
		}
	}
	return fields
}

// childRecord renders one child as a standalone block: the child's own
// fields first (the child is the subject), then the guardian/contact
// details led by the parent name, repeated on every child so a single
// block holds everything a supervisor needs offline.
func childRecord(req *Request, ch ExportChildRow, guardians []*capability.RequestGuardian, guardianCustoms, childCustoms []capability.FormField) lists.Record {
	rec := lists.Record{
		Title:  childFullName(ch.Child),
		Fields: childFields(ch, childCustoms),
	}
	rec.Fields = append(rec.Fields, lists.Field{Label: "Eltern", Value: guardianFullName(req)})
	rec.Fields = append(rec.Fields, guardianBlockFields(req, guardianCustoms)...)
	rec.Fields = append(rec.Fields, additionalGuardianFields(guardians)...)
	return rec
}

// guardianOnlyRecord is the fallback block for a registration with no
// child rows — keeps the guardian (and their answers) in the export
// rather than dropping the submission entirely.
func guardianOnlyRecord(req *Request, guardians []*capability.RequestGuardian, guardianCustoms []capability.FormField) lists.Record {
	rec := lists.Record{
		Title:  guardianFullName(req),
		Fields: guardianBlockFields(req, guardianCustoms),
	}
	rec.Fields = append(rec.Fields, additionalGuardianFields(guardians)...)
	return rec
}

// additionalGuardianFields renders one label/value line per co-guardian for
// the PDF/DOCX record, each "Vorname Nachname (E-Mail, Telefon)" with empty
// contact parts omitted. Empty when the submission had no co-guardians.
func additionalGuardianFields(guardians []*capability.RequestGuardian) []lists.Field {
	fields := make([]lists.Field, 0, len(guardians))
	for _, g := range guardians {
		fields = append(fields, lists.Field{
			Label: "Weitere Erziehungsberechtigte",
			Value: formatGuardianContact(g),
		})
	}
	return fields
}

// formatGuardianContact renders one co-guardian as "Vorname Nachname
// (E-Mail, Telefon)", dropping whichever contact parts are absent (a
// co-guardian may be a name+phone-only or name+email-only contact).
func formatGuardianContact(g *capability.RequestGuardian) string {
	name := strings.TrimSpace(g.FirstName + " " + g.LastName)
	contact := make([]string, 0, 2)
	if g.Email != nil && strings.TrimSpace(*g.Email) != "" {
		contact = append(contact, strings.TrimSpace(*g.Email))
	}
	if g.Phone != nil && strings.TrimSpace(*g.Phone) != "" {
		contact = append(contact, strings.TrimSpace(*g.Phone))
	}
	if len(contact) > 0 {
		return name + " (" + strings.Join(contact, ", ") + ")"
	}
	return name
}

// formatAdditionalGuardians joins every co-guardian into one cell for the
// XLSX export (one column can't expand per row), separated by "; ".
func formatAdditionalGuardians(guardians []*capability.RequestGuardian) string {
	if len(guardians) == 0 {
		return ""
	}
	parts := make([]string, 0, len(guardians))
	for _, g := range guardians {
		parts = append(parts, formatGuardianContact(g))
	}
	return strings.Join(parts, "; ")
}

// childFields builds the child's own label/value lines: identity, target
// class, status, activation, offerings, then per-child custom answers.
func childFields(ch ExportChildRow, childCustoms []capability.FormField) []lists.Field {
	c := ch.Child
	fields := []lists.Field{
		{Label: "Geburtsdatum", Value: calendar.Date(c.DateOfBirth).Format("02.01.2006")},
		{Label: "Zielklasse", Value: schoolClassLabel(c.TargetSchoolClass, c.TargetGradeLevel)},
		{Label: "Status", Value: statusLabelDE(c.Status)},
	}
	if c.StatusReason != nil && strings.TrimSpace(*c.StatusReason) != "" {
		fields = append(fields, lists.Field{Label: "Status-Grund", Value: *c.StatusReason})
	}
	fields = append(fields,
		lists.Field{Label: "Aktivierung", Value: activationSummary(c)},
		lists.Field{Label: "Betreuungsangebote", Value: formatOfferings(ch.Offerings)},
	)
	for _, cf := range childCustoms {
		if v, ok := c.CustomData[cf.Key]; ok {
			fields = append(fields, lists.Field{Label: cf.Label, Value: formatCustomValue(cf, v)})
		}
	}
	if note := childCompanionNote(c); note != "" {
		fields = append(fields, lists.Field{Label: departureCompanionLabelDE, Value: note})
	}
	return fields
}

// departureCompanionLabelDE labels the coupled "mit wem" note (#1694) in staff
// exports. Matches the "Mit welchem Kind?" wording shown in the student detail
// and admin review UIs.
const departureCompanionLabelDE = "Mit welchem Kind?"

// childCompanionNote extracts the coupled accompanied-mode "mit wem" note from a
// child's custom_data. It lives on a reserved key alongside
// allowed_departure_modes (not a schema field), so the field-iterating export
// loops never emit it — it must be pulled out explicitly. Returns "" when
// absent or blank.
func childCompanionNote(c *RequestChild) string {
	if c == nil || c.CustomData == nil {
		return ""
	}
	return strings.TrimSpace(stringifyValue(c.CustomData[capability.TargetStudentDepartureCompanionNote]))
}

// childFullName is the child's display heading for its block.
func childFullName(c *RequestChild) string {
	name := strings.TrimSpace(c.FirstName + " " + c.LastName)
	if name == "" {
		return "Kind"
	}
	return name
}
