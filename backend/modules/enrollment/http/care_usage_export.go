package enrollmenthttp

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// buildCareUsageExport dispatches on the requested layout: "compact" renders
// the class-roster-style table for PDF/DOCX; everything else (including XLSX,
// which is always tabular) takes the existing detailed path.
func buildCareUsageExport(svc lists.DocumentRenderer, payload careUsageExportPayload, format lists.Format) (lists.File, error) {
	if payload.layout == careUsageLayoutCompact && format != lists.FormatXLSX {
		return svc.Render(buildCareUsageCompactTableDocument(payload.report), format, careUsageExportFilename(payload.report)+" kompakt")
	}
	return buildCareUsageExportFile(svc, payload.report, format)
}

func careUsageExportFilename(report *capability.CareUsageReport) string {
	if name := strings.TrimSpace(report.Phase.Name); name != "" {
		return "Anmelde-Auswertung " + name
	}
	return "Anmelde-Auswertung"
}

func buildCareUsageExportFile(svc lists.DocumentRenderer, report *capability.CareUsageReport, format lists.Format) (lists.File, error) {
	filename := careUsageExportFilename(report)
	switch format {
	case lists.FormatDOCX:
		return svc.RenderRecordsDOCX(buildCareUsageRecordDocument(report), filename)
	case lists.FormatPDF:
		return svc.RenderRecords(buildCareUsageRecordDocument(report), filename)
	case lists.FormatXLSX:
		return svc.Render(buildCareUsageTableDocument(report), lists.FormatXLSX, filename)
	default:
		return lists.File{}, fmt.Errorf("unsupported export format %q", format)
	}
}

// buildCareUsageCompactTableDocument renders the filtered care-usage report in
// the compact class-roster layout (#2215): one row per child with weekday
// pickup columns. Rows are sorted by target class; when the result spans more
// than one class, each class gets a heading row (a new page in the PDF),
// mirroring the "Alle Klassen" roster behavior.
func buildCareUsageCompactTableDocument(report *capability.CareUsageReport) lists.Document {
	cols := []lists.Column{
		{ID: lists.ColumnName, Label: "Name"},
		{ID: lists.ColumnSchoolClass, Label: "Zielklasse"},
		{ID: lists.ColumnWeeklyMonday, Label: "Montag"},
		{ID: lists.ColumnWeeklyTuesday, Label: "Dienstag"},
		{ID: lists.ColumnWeeklyWednesday, Label: "Mittwoch"},
		{ID: lists.ColumnWeeklyThursday, Label: "Donnerstag"},
		{ID: lists.ColumnWeeklyFriday, Label: "Freitag"},
		{ID: lists.ColumnGuardianContacts, Label: "Erziehungsberechtigte"},
	}
	sorted := slices.Clone(report.Rows)
	// The service sorts by child name; the stable re-sort keeps that order
	// within each class section.
	sort.SliceStable(sorted, func(i, j int) bool {
		return collation.CompareSchoolClasses(careUsageClassLabel(sorted[i]), careUsageClassLabel(sorted[j])) < 0
	})
	grouped := careUsageSpansMultipleClasses(sorted)
	rows := make([]lists.Row, 0, len(sorted))
	currentClass := ""
	for i, row := range sorted {
		class := careUsageClassLabel(row)
		if grouped && (i == 0 || collation.CompareSchoolClasses(class, currentClass) != 0) {
			currentClass = class
			rows = append(rows, lists.Row{GroupTitle: careUsageGroupTitle(class)})
		}
		rows = append(rows, lists.Row{Values: map[lists.ColumnID]string{
			lists.ColumnName:             strings.TrimSpace(row.ChildFirstName + " " + row.ChildLastName),
			lists.ColumnSchoolClass:      class,
			lists.ColumnWeeklyMonday:     careUsageWeeklyCell(row, "mon"),
			lists.ColumnWeeklyTuesday:    careUsageWeeklyCell(row, "tue"),
			lists.ColumnWeeklyWednesday:  careUsageWeeklyCell(row, "wed"),
			lists.ColumnWeeklyThursday:   careUsageWeeklyCell(row, "thu"),
			lists.ColumnWeeklyFriday:     careUsageWeeklyCell(row, "fri"),
			lists.ColumnGuardianContacts: classRosterGuardianContactsLabel(careUsageGuardians(row)),
		}})
	}
	return lists.Document{
		Title:       careUsageTitle(report),
		Subtitle:    careUsageSubtitle(report),
		GeneratedAt: time.Now(),
		Filters:     careUsageFilterLabels(report),
		Columns:     cols,
		Rows:        rows,
		Footer:      exportConfidentialityNote,
	}
}

func careUsageClassLabel(row capability.CareUsageRow) string {
	return schoolClassLabel(row.TargetSchoolClass, row.TargetGradeLevel)
}

// careUsageGroupTitle avoids "Klasse 1. Klasse" headings: grade-only labels
// from gradeLabel already carry the word "Klasse" as a suffix, which the
// shared prefix-based ClassGroupTitle helper cannot detect.
func careUsageGroupTitle(class string) string {
	if strings.Contains(strings.ToLower(class), "klasse") {
		return class
	}
	return lists.ClassGroupTitle(class)
}

// careUsageSpansMultipleClasses reports whether the class-sorted rows cover
// more than one logical class (comparator equivalence, so "1a"/"1A" count as
// one). Single-class results skip the group headings.
func careUsageSpansMultipleClasses(sorted []capability.CareUsageRow) bool {
	for i := 1; i < len(sorted); i++ {
		if collation.CompareSchoolClasses(careUsageClassLabel(sorted[i]), careUsageClassLabel(sorted[0])) != 0 {
			return true
		}
	}
	return false
}

func careUsageWeeklyCell(row capability.CareUsageRow, day string) string {
	careDays := row.CareDays
	if careDays == nil {
		careDays = row.EffectiveDays
	}
	return weeklyPickupCell(
		containsReportDay(careDays, day),
		row.SchedulePickupByDay[day],
		row.PickupByDay[day],
		careUsageDailyOfferingNames(row, day),
	)
}

func careUsageDailyOfferingNames(row capability.CareUsageRow, day string) []string {
	names := make([]string, 0, len(row.Offerings))
	for _, offering := range row.Offerings {
		if containsReportDay(offering.Days, day) {
			names = append(names, offering.Name)
		}
	}
	return normalizedClassRosterOfferingNames(names)
}

// careUsageGuardians prefers the enriched full contact list; rows built
// without enrichment (older callers, tests) fall back to the submitting
// guardian carried on the flat fields.
func careUsageGuardians(row capability.CareUsageRow) []capability.ClassRosterGuardian {
	if len(row.Guardians) > 0 {
		return row.Guardians
	}
	return []capability.ClassRosterGuardian{{
		Name:  strings.TrimSpace(row.GuardianFirstName + " " + row.GuardianLastName),
		Email: row.GuardianEmail,
		Phone: deref(row.GuardianPhone),
	}}
}

func buildCareUsageTableDocument(report *capability.CareUsageReport) lists.Document {
	cols := []lists.Column{
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
	rows := make([]lists.Row, 0, len(report.Rows))
	for _, row := range report.Rows {
		rows = append(rows, lists.Row{Values: map[lists.ColumnID]string{
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
			"guardian_phone":   deref(row.GuardianPhone),
		}})
	}
	return lists.Document{
		Title:       careUsageTitle(report),
		Subtitle:    careUsageSubtitle(report),
		GeneratedAt: time.Now(),
		Filters:     careUsageFilterLabels(report),
		Columns:     cols,
		Rows:        rows,
		Footer:      exportConfidentialityNote,
	}
}

func buildCareUsageRecordDocument(report *capability.CareUsageReport) lists.RecordDocument {
	records := make([]lists.Record, 0, len(report.Rows)+1)
	records = append(records, lists.Record{
		Title:  "Einsatzplanung nach Gehzeit",
		Fields: careUsagePickupPlanningFields(report),
	})
	for _, row := range report.Rows {
		records = append(records, lists.Record{
			Title: strings.TrimSpace(row.ChildFirstName + " " + row.ChildLastName),
			Fields: []lists.Field{
				{Label: "Zielklasse", Value: schoolClassLabel(row.TargetSchoolClass, row.TargetGradeLevel)},
				{Label: "Status", Value: statusLabelDE(row.Status)},
				{Label: "Betreuungsangebote", Value: careUsageOfferingDayDetails(row.Offerings)},
				{Label: "Effektive Betreuungstage", Value: formatDayCodes(row.EffectiveDays)},
				{Label: "Anzahl Tage", Value: strconv.Itoa(row.DayCount)},
				{Label: "Gehzeiten", Value: careUsagePickupDayDetails(row.PickupByDay)},
				{Label: "Eltern", Value: strings.TrimSpace(row.GuardianFirstName + " " + row.GuardianLastName)},
				{Label: "E-Mail", Value: row.GuardianEmail},
				{Label: "Telefon", Value: deref(row.GuardianPhone)},
			},
		})
	}
	return lists.RecordDocument{
		Title:       careUsageTitle(report),
		Subtitle:    careUsageSubtitle(report),
		GeneratedAt: time.Now(),
		Footer:      exportConfidentialityNote,
		Filters:     careUsageFilterLabels(report),
		Records:     records,
	}
}

func careUsageTitle(report *capability.CareUsageReport) string {
	if report != nil && strings.TrimSpace(report.Phase.Name) != "" {
		return "Auswertung " + strings.TrimSpace(report.Phase.Name)
	}
	return "Anmelde-Auswertung"
}

func careUsageSubtitle(report *capability.CareUsageReport) string {
	if report == nil {
		return "0 Kinder"
	}
	if report.Totals.Children == 1 {
		return "1 Kind"
	}
	return fmt.Sprintf("%d Kinder", report.Totals.Children)
}

func careUsageFilterLabels(report *capability.CareUsageReport) []string {
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

func careUsageOfferingNameByID(report *capability.CareUsageReport, id int64) string {
	for _, option := range report.FilterOptions.Offerings {
		if option.ID == id {
			return option.Name
		}
	}
	return "Angebot #" + strconv.FormatInt(id, 10)
}

func careUsageOfferingNames(offerings []capability.CareUsageRowOffering) string {
	parts := make([]string, 0, len(offerings))
	for _, offering := range offerings {
		parts = append(parts, offering.Name)
	}
	return strings.Join(parts, "; ")
}

func careUsageOfferingDayDetails(offerings []capability.CareUsageRowOffering) string {
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

func careUsageOfferingDayDetail(offering capability.CareUsageRowOffering) string {
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

func careUsagePickupPlanningFields(report *capability.CareUsageReport) []lists.Field {
	pickupTimes := careUsagePickupPlanningTimes(report)
	fields := make([]lists.Field, 0, len(weekdayOrder)*len(pickupTimes))
	for _, day := range weekdayOrder {
		for _, pickupTime := range pickupTimes {
			count := 0
			if report != nil && report.Totals.ByWeekdayPickupTime[day] != nil {
				count = report.Totals.ByWeekdayPickupTime[day][pickupTime]
			}
			fields = append(fields, lists.Field{
				Label: dayLabelsDE[day] + " bis " + pickupTime,
				Value: strconv.Itoa(count),
			})
		}
	}
	return fields
}

func careUsagePickupPlanningTimes(report *capability.CareUsageReport) []string {
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
