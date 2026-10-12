package enrollmenthttp

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func buildClassRosterExportFile(svc lists.DocumentRenderer, report *capability.ClassRosterReport, format lists.Format) (lists.File, error) {
	filename := "Klassenliste " + strings.TrimSpace(report.Filters.SchoolClass)
	if report.Filters.AllClasses {
		filename = "Klassenlisten"
	}
	if phaseName := strings.TrimSpace(report.Phase.Name); phaseName != "" {
		filename += " " + phaseName
	}
	return svc.Render(buildClassRosterTableDocument(report), format, filename)
}

func buildClassRosterTableDocument(report *capability.ClassRosterReport) lists.Document {
	cols := []lists.Column{
		{ID: lists.ColumnName, Label: "Name"},
		{ID: lists.ColumnSchoolClass, Label: "Klasse"},
		{ID: lists.ColumnWeeklyMonday, Label: "Montag"},
		{ID: lists.ColumnWeeklyTuesday, Label: "Dienstag"},
		{ID: lists.ColumnWeeklyWednesday, Label: "Mittwoch"},
		{ID: lists.ColumnWeeklyThursday, Label: "Donnerstag"},
		{ID: lists.ColumnWeeklyFriday, Label: "Freitag"},
		{ID: lists.ColumnGuardianContacts, Label: "Erziehungsberechtigte"},
	}
	rows := make([]lists.Row, 0, len(report.Rows))
	currentClass := ""
	for i, row := range report.Rows {
		// All-classes rosters arrive class-sorted from the service; a
		// heading row starts each class section (new page in the PDF).
		// Boundaries use the sort comparator's equivalence so label
		// variants like "1a"/"1A" share one heading (first-seen label).
		if class := strings.TrimSpace(row.SchoolClass); report.Filters.AllClasses && (i == 0 || collation.CompareSchoolClasses(class, currentClass) != 0) {
			currentClass = class
			rows = append(rows, lists.Row{GroupTitle: lists.ClassGroupTitle(class)})
		}
		name := strings.TrimSpace(row.FirstName + " " + row.LastName)
		if row.ListEntry {
			// Class-list-only entry (#2382): the child has no OGS record at
			// all. The marker sits in the name cell because the roster table
			// has no status column — every weekday cell stays "—".
			name += " (" + capability.ClassListEntryNoCareLabel + ")"
		}
		rows = append(rows, lists.Row{Values: map[lists.ColumnID]string{
			lists.ColumnName:             name,
			lists.ColumnSchoolClass:      row.SchoolClass,
			lists.ColumnWeeklyMonday:     classRosterWeeklyCell(row, "mon"),
			lists.ColumnWeeklyTuesday:    classRosterWeeklyCell(row, "tue"),
			lists.ColumnWeeklyWednesday:  classRosterWeeklyCell(row, "wed"),
			lists.ColumnWeeklyThursday:   classRosterWeeklyCell(row, "thu"),
			lists.ColumnWeeklyFriday:     classRosterWeeklyCell(row, "fri"),
			lists.ColumnGuardianContacts: classRosterGuardianContactsLabel(row.Guardians),
		}})
	}
	return lists.Document{
		Title:       classRosterTitle(report),
		Subtitle:    classRosterSubtitle(report),
		GeneratedAt: time.Now(),
		Filters:     classRosterFilterLabels(report),
		Columns:     cols,
		Rows:        rows,
		Footer:      exportConfidentialityNote,
	}
}

func classRosterTitle(report *capability.ClassRosterReport) string {
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

func classRosterSubtitle(report *capability.ClassRosterReport) string {
	if report == nil {
		return "0 Kinder"
	}
	subtitle := fmt.Sprintf("%d Kinder, %d angemeldet", report.Totals.Students, report.Totals.Registered)
	if report.Totals.ListEntries > 0 {
		subtitle += fmt.Sprintf(", %d ohne Betreuung", report.Totals.ListEntries)
	}
	return subtitle
}

func classRosterFilterLabels(report *capability.ClassRosterReport) []string {
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

// classRosterWeeklyCell renders one weekday cell: the pickup time plus the
// day's own Geh-/Abholregelung ("14:30 Uhr, wird abgeholt"). The former
// summarized "Geh-/Abholweise" week column is folded in here per day (#2254);
// days without care stay "—" and carry no departure text.
func classRosterWeeklyCell(row capability.ClassRosterRow, day string) string {
	isCareDay := containsReportDay(row.CareDays, day)
	cell := weeklyPickupCell(
		isCareDay,
		row.SchedulePickupByDay[day],
		row.PickupByDay[day],
		classRosterDailyOfferings(row, day),
	)
	if !isCareDay {
		return cell
	}
	if rule := strings.TrimSpace(row.DepartureByDay[day]); rule != "" {
		cell += ", " + rule
	}
	return cell
}

// weeklyPickupCell renders one weekday cell of the roster-style tables
// (class roster and compact care-usage export): "—" outside care days; the
// maintained Kind-Gehzeit (manual rows and rolled-out Angebots-Gehzeiten)
// outranks the enrollment-form snapshot (#2290); offering names are the
// fallback for schools whose form encodes the care window in the offering
// name ("Betreuung bis 14:30 Uhr") instead of a pickup-time field.
func weeklyPickupCell(isCareDay bool, schedulePickup, snapshotPickup string, offeringNames []string) string {
	if !isCareDay {
		return "—"
	}
	if pickup := strings.TrimSpace(schedulePickup); pickup != "" {
		return pickup + " Uhr"
	}
	if pickup := strings.TrimSpace(snapshotPickup); pickup != "" {
		return pickup + " Uhr"
	}
	if len(offeringNames) > 0 {
		return strings.Join(offeringNames, "; ")
	}
	return "Keine Abholzeit"
}

func classRosterDailyOfferings(row capability.ClassRosterRow, day string) []string {
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

func classRosterGuardianContactsLabel(guardians []capability.ClassRosterGuardian) string {
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

func containsReportDay(days []string, needle string) bool {
	for _, day := range days {
		if strings.EqualFold(strings.TrimSpace(day), needle) {
			return true
		}
	}
	return false
}
