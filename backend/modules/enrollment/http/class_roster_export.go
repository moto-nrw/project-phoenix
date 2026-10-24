package enrollmenthttp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
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
	children := make([]lists.ClassRosterChild, 0, len(report.Rows))
	for _, row := range report.Rows {
		// A class-list-only entry (#2382) has no OGS record at all: the
		// shared table marks its name and leaves every weekday "—".
		child := lists.ClassRosterChild{
			Name:        row.FirstName + " " + row.LastName,
			SchoolClass: row.SchoolClass,
			NoCare:      row.ListEntry,
			Extra: map[lists.ColumnID]string{
				lists.ColumnGuardianContacts: classRosterGuardianContactsLabel(row.Guardians),
			},
		}
		for i, day := range departure.PickupDayOrder {
			child.Days[i] = classRosterDay(row, day)
		}
		children = append(children, child)
	}
	head := enrollmentExportHead{
		Title:    classRosterTitle(report),
		Subtitle: classRosterSubtitle(report),
		Filters:  classRosterFilterLabels(report),
	}
	// All-classes rosters arrive class-sorted from the service.
	return enrollmentClassRosterDocument(head, "Klasse", children, report.Filters.AllClasses)
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

// classRosterDay reads one weekday of an enrollment row for the shared
// class-list cell: the maintained Kind-Gehzeit (manual rows and rolled-out
// Angebots-Gehzeiten) outranks the enrollment-form snapshot (#2290), and the
// day's own Geh-/Abholregelung follows it (#2254).
func classRosterDay(row capability.ClassRosterRow, day string) lists.ClassRosterDay {
	pickup := strings.TrimSpace(row.SchedulePickupByDay[day])
	if pickup == "" {
		pickup = strings.TrimSpace(row.PickupByDay[day])
	}
	return lists.ClassRosterDay{
		Care:      containsReportDay(row.CareDays, day),
		Pickup:    pickup,
		Offerings: classRosterDailyOfferings(row, day),
		Departure: row.DepartureByDay[day],
	}
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
