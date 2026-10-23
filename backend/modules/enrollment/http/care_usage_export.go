package enrollmenthttp

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/collation"
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
	return renderEnrollmentExport(svc, format, careUsageExportFilename(report),
		func() lists.RecordDocument { return buildCareUsageRecordDocument(report) },
		func() lists.Document { return buildCareUsageTableDocument(report) },
	)
}

// buildCareUsageCompactTableDocument renders the filtered care-usage report in
// the class-list table of the enrollment Klassenliste (#2215, #4750): one row
// per child with weekday pickup cells. Rows are sorted by target class; when
// the result spans more than one class, each class gets a heading row (a new
// page in the PDF), mirroring the "Alle Klassen" roster behavior.
func buildCareUsageCompactTableDocument(report *capability.CareUsageReport) lists.Document {
	sorted := slices.Clone(report.Rows)
	// The service sorts by child name; the stable re-sort keeps that order
	// within each class section.
	sort.SliceStable(sorted, func(i, j int) bool {
		return collation.CompareSchoolClasses(careUsageClassLabel(sorted[i]), careUsageClassLabel(sorted[j])) < 0
	})
	children := make([]lists.ClassRosterChild, 0, len(sorted))
	for _, row := range sorted {
		child := lists.ClassRosterChild{
			Name:        row.ChildFirstName + " " + row.ChildLastName,
			SchoolClass: careUsageClassLabel(row),
			Extra: map[lists.ColumnID]string{
				lists.ColumnGuardianContacts: classRosterGuardianContactsLabel(careUsageGuardians(row)),
			},
		}
		for i, day := range weekdayOrder {
			child.Days[i] = careUsageRosterDay(row, day)
		}
		children = append(children, child)
	}
	return enrollmentClassRosterDocument(careUsageHead(report), "Zielklasse", children, careUsageSpansMultipleClasses(sorted))
}

func careUsageClassLabel(row capability.CareUsageRow) string {
	return schoolClassLabel(row.TargetSchoolClass, row.TargetGradeLevel)
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

// careUsageRosterDay reads one weekday of a report row for the class-list
// cell. The maintained Kind-Gehzeit outranks the form snapshot, as on the
// Klassenliste; the compact export carries no Geh-/Abholregelung.
func careUsageRosterDay(row capability.CareUsageRow, day string) lists.ClassRosterDay {
	careDays := row.CareDays
	if careDays == nil {
		careDays = row.EffectiveDays
	}
	pickup := strings.TrimSpace(row.SchedulePickupByDay[day])
	if pickup == "" {
		pickup = strings.TrimSpace(row.PickupByDay[day])
	}
	return lists.ClassRosterDay{
		Care:      containsReportDay(careDays, day),
		Pickup:    pickup,
		Offerings: careUsageDailyOfferingNames(row, day),
	}
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
		Name:  careUsageGuardianName(row),
		Email: row.GuardianEmail,
		Phone: deref(row.GuardianPhone),
	}}
}

func buildCareUsageTableDocument(report *capability.CareUsageReport) lists.Document {
	cols := enrollmentColumns("child_last_name", "child_first_name", "child_grade", "child_status",
		"offerings", "offering_days", "effective_days", "day_count",
		"pickup_mon", "pickup_tue", "pickup_wed", "pickup_thu", "pickup_fri",
		"guardian_name", "guardian_email", "guardian_phone")
	rows := make([]lists.Row, 0, len(report.Rows))
	for _, row := range report.Rows {
		rows = append(rows, lists.Row{Values: map[lists.ColumnID]string{
			"child_last_name":  row.ChildLastName,
			"child_first_name": row.ChildFirstName,
			"child_grade":      careUsageClassLabel(row),
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
			"guardian_name":    careUsageGuardianName(row),
			"guardian_email":   row.GuardianEmail,
			"guardian_phone":   deref(row.GuardianPhone),
		}})
	}
	return careUsageHead(report).table(cols, rows)
}

func buildCareUsageRecordDocument(report *capability.CareUsageReport) lists.RecordDocument {
	records := make([]lists.Record, 0, len(report.Rows)+1)
	records = append(records, lists.Record{
		Title:  "Einsatzplanung nach Gehzeit",
		Fields: careUsagePickupPlanningFields(report),
	})
	for _, row := range report.Rows {
		records = append(records, enrollmentChildBlock{
			Name:     strings.TrimSpace(row.ChildFirstName + " " + row.ChildLastName),
			Child:    careUsageChildFields(row),
			Guardian: guardianContactFields(careUsageGuardianName(row), row.GuardianEmail, deref(row.GuardianPhone)),
		}.record())
	}
	return careUsageHead(report).blocks(records, nil)
}

// careUsageChildFields is what the Auswertung reports about one child: target
// class, status, offerings with their days, and the pickup times.
func careUsageChildFields(row capability.CareUsageRow) []lists.Field {
	return []lists.Field{
		{Label: "Zielklasse", Value: careUsageClassLabel(row)},
		{Label: "Status", Value: statusLabelDE(row.Status)},
		{Label: "Betreuungsangebote", Value: careUsageOfferingDayDetails(row.Offerings)},
		{Label: "Effektive Betreuungstage", Value: formatDayCodes(row.EffectiveDays)},
		{Label: "Anzahl Tage", Value: strconv.Itoa(row.DayCount)},
		{Label: "Gehzeiten", Value: careUsagePickupDayDetails(row.PickupByDay)},
	}
}

func careUsageGuardianName(row capability.CareUsageRow) string {
	return strings.TrimSpace(row.GuardianFirstName + " " + row.GuardianLastName)
}

func careUsageHead(report *capability.CareUsageReport) enrollmentExportHead {
	return enrollmentExportHead{
		Title:    careUsageTitle(report),
		Subtitle: careUsageSubtitle(report),
		Filters:  careUsageFilterLabels(report),
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
