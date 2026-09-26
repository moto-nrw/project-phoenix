package students

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// exportSortMode resolves the ordering a request asks for. Calendar order is
// what makes a birthday list a birthday list, so the preset implies it rather
// than depending on a caller to pass the matching sort; an explicit sort still
// wins for the rare pickup/arrival view of the same list.
func exportSortMode(req studentExportRequest) string {
	if req.Filters.Sort == "" && req.Preset == lists.PresetBirthdayList {
		return "birthday"
	}
	return req.Filters.Sort
}

func sortExportResponses(students []StudentResponse, sortMode string) {
	sort.SliceStable(students, func(i, j int) bool {
		a := students[i]
		b := students[j]
		if sortMode == "pickup" {
			return timeValue(a.PickupTime) < timeValue(b.PickupTime)
		}
		if sortMode == "arrival" {
			return timeValue(a.ArrivalTime) < timeValue(b.ArrivalTime)
		}
		// Same-day children fall through to name collation below.
		if sortMode == "birthday" {
			if ka, kb := birthdaySortKey(a.Birthday), birthdaySortKey(b.Birthday); ka != kb {
				return ka < kb
			}
		}
		return collation.CompareGermanNames(a.LastName, a.FirstName, b.LastName, b.FirstName) < 0
	})
}

// birthdaySortKey orders by the annually recurring day ("MM-DD") rather than
// the birth year, so a birthday list reads as a calendar instead of an age
// ranking. Children without a birthday sort last.
func birthdaySortKey(birthday string) string {
	date, err := timezone.ParseDate(birthday)
	if err != nil {
		return "99-99"
	}
	return fmt.Sprintf("%02d-%02d", int(date.Month()), date.Day())
}

func (rs *Resource) loadWeeklySchedules(r *http.Request, studentIDs []int64, planningDate timezone.Date) (map[int64]weeklySchedule, error) {
	result := make(map[int64]weeklySchedule, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}
	if rs.ArrivalScheduleService == nil || rs.PickupScheduleService == nil {
		return nil, errors.New("student schedule repositories are not configured")
	}
	for _, studentID := range studentIDs {
		result[studentID] = weeklySchedule{
			ArrivalByWeekday:  make(map[int]string),
			CareDaysByWeekday: make(map[int]bool),
			PickupByWeekday:   make(map[int]string),
		}
	}
	pickups, err := rs.PickupScheduleService.GetWeeklySchedulesByStudentIDsForDate(r.Context(), studentIDs, planningDate)
	if err != nil {
		return nil, err
	}
	for _, pickup := range pickups {
		weekly := result[pickup.StudentID]
		weekly.PickupByWeekday[pickup.Weekday] = formatWallClock(pickup.PickupTime)
		result[pickup.StudentID] = weekly
	}
	arrivals, err := rs.ArrivalScheduleService.GetWeeklySchedulesByStudentIDsForDate(r.Context(), studentIDs, planningDate)
	if err != nil {
		return nil, err
	}
	for _, arrival := range arrivals {
		weekly := result[arrival.StudentID]
		weekly.CareDaysByWeekday[arrival.Weekday] = true
		if !arrival.ExpectedArrival.IsZero() {
			weekly.ArrivalByWeekday[arrival.Weekday] = formatWallClock(arrival.ExpectedArrival)
		}
		result[arrival.StudentID] = weekly
	}
	return result, nil
}

func (rs *Resource) loadActiveEnrollmentSummaries(r *http.Request, studentIDs []int64, onDate timezone.Date, columns []lists.Column) (map[int64]string, error) {
	if !columnsContain(columns, lists.ColumnEnrollmentSummary) {
		return map[int64]string{}, nil
	}
	if rs.ActiveEnrollments == nil {
		return nil, errors.New("activity service is not configured")
	}
	groupsByStudent, err := rs.ActiveEnrollments.ActiveEnrollmentGroups(r.Context(), studentIDs, onDate)
	if err != nil {
		return nil, err
	}
	summaries := make(map[int64]string, len(studentIDs))
	for _, studentID := range studentIDs {
		summaries[studentID] = enrollmentSummaryLabel(groupsByStudent[studentID])
	}
	return summaries, nil
}

// enrollmentSummaryLabel renders the export's "angemeldet" cell for one child:
// the deduplicated, alphabetically sorted list of active activity-group names,
// or "Keine Anmeldung" when the child has no active enrollment.
func enrollmentSummaryLabel(groups []ActiveEnrollmentGroup) string {
	if len(groups) == 0 {
		return "Keine Anmeldung"
	}
	names := make([]string, 0, len(groups))
	seen := make(map[string]bool, len(groups))
	for _, group := range groups {
		name := strings.TrimSpace(group.Name)
		if name == "" {
			name = "Gruppe #" + strconv.FormatInt(group.ID, 10)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "Keine Anmeldung"
	}
	return "Angemeldet: " + strings.Join(names, ", ")
}

func columnsContain(columns []lists.Column, id lists.ColumnID) bool {
	for _, column := range columns {
		if column.ID == id {
			return true
		}
	}
	return false
}

// groupExportResponsesByClass stably re-sorts by class only, preserving the
// prior within-class ordering (name collation or pickup/arrival sort mode).
func groupExportResponsesByClass(students []StudentResponse) {
	sort.SliceStable(students, func(i, j int) bool {
		return collation.CompareSchoolClasses(students[i].SchoolClass, students[j].SchoolClass) < 0
	})
}

// responseRowSources renders every student response into its future document
// row plus the sort keys the class-list-entry merge (#2382) needs. The order
// of `students` (whatever sort mode produced it) is preserved.
func responseRowSources(students []StudentResponse, weekly map[int64]weeklySchedule, enrollmentSummaries map[int64]string, onDate timezone.Date, isToday bool) []exportRowSource {
	sources := make([]exportRowSource, 0, len(students))
	for _, student := range students {
		sources = append(sources, exportRowSource{
			schoolClass: student.SchoolClass,
			lastName:    student.LastName,
			firstName:   student.FirstName,
			row:         buildExportRow(student, weekly[student.ID], enrollmentSummaries, onDate, isToday),
		})
	}
	return sources
}

// birthdayExportCell renders the birth date German-style ("02.09.2018").
// Children without a stored birthday render empty rather than a fake date.
func birthdayExportCell(birthday string) string {
	date, err := timezone.ParseDate(birthday)
	if err != nil {
		return ""
	}
	return date.Format("02.01.2006")
}

// ageExportCell renders the age in completed years as of onDate. A birthday
// later this year has not happened yet, so that year is not counted.
func ageExportCell(birthday string, onDate timezone.Date) string {
	date, err := timezone.ParseDate(birthday)
	if err != nil {
		return ""
	}
	years := onDate.Year() - date.Year()
	if onDate.Month() < date.Month() || (onDate.Month() == date.Month() && onDate.Day() < date.Day()) {
		years--
	}
	if years < 0 {
		return ""
	}
	return strconv.Itoa(years)
}

// buildExportRow renders one child into the generic list document.
//
// It deliberately carries NO health note. The generic export never resolves
// ColumnHealthInfo (see the renderer's column catalog behind lists.ResolveColumns); the one child list that
// prints it, the Gesundheitsliste, fills the cell afterwards together with its
// audit record (finalizeExportSources), so no other preset can reach it.
func buildExportRow(student StudentResponse, plan weeklySchedule, enrollmentSummaries map[int64]string, onDate timezone.Date, isToday bool) lists.Row {
	return lists.Row{Values: map[lists.ColumnID]string{
		lists.ColumnName:              strings.TrimSpace(student.FirstName + " " + student.LastName),
		lists.ColumnSchoolClass:       student.SchoolClass,
		lists.ColumnGroup:             student.GroupName,
		lists.ColumnEnrollmentSummary: enrollmentSummaries[student.ID],
		lists.ColumnCareDays:          careDays(plan),
		lists.ColumnWeeklyMonday:      weeklyCell(plan, weekdayMonday),
		lists.ColumnWeeklyTuesday:     weeklyCell(plan, weekdayTuesday),
		lists.ColumnWeeklyWednesday:   weeklyCell(plan, weekdayWednesday),
		lists.ColumnWeeklyThursday:    weeklyCell(plan, weekdayThursday),
		lists.ColumnWeeklyFriday:      weeklyCell(plan, weekdayFriday),
		lists.ColumnDailyStatus:       dailyStatusExportCell(student, isToday),
		lists.ColumnPlannedArrival:    derefOrEmpty(student.ArrivalTime),
		lists.ColumnPlannedPickup:     derefOrEmpty(student.PickupTime),
		lists.ColumnDeparture:         departureExportCell(student),
		lists.ColumnDailyNotes:        dailyNotes(student),
		lists.ColumnCurrentLocation:   student.Location,
		lists.ColumnBirthday:          birthdayExportCell(student.Birthday),
		lists.ColumnAge:               ageExportCell(student.Birthday, onDate),
	}}
}

func dailyStatusExportCell(student StudentResponse, isToday bool) string {
	switch student.DayPlanningStatus {
	case DayPlanningStatusComesToday:
		return dayLabel("Kommt heute", "Wird erwartet", isToday)
	case DayPlanningStatusNotComingToday:
		switch student.DayPlanningReason {
		case dayPlanningReasonSick:
			return "Krank"
		case dayPlanningReasonExcused:
			return "Entschuldigt"
		case dayPlanningReasonClassTrip:
			return "Klassenfahrt"
		}
		if student.DayPlanningLabel != "" {
			return sentenceCase(student.DayPlanningLabel)
		}
		return dayLabel("Kommt heute nicht", "Wird nicht erwartet", isToday)
	}

	if student.Sick {
		return "Krank"
	}
	if student.ClassTrip {
		return "Klassenfahrt"
	}
	if student.Excused {
		return "Entschuldigt"
	}
	return ""
}

func sentenceCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(value)
	if r == utf8.RuneError && size == 0 {
		return ""
	}
	return string(unicode.ToUpper(r)) + value[size:]
}

// exportHasColumn reports whether the resolved column set carries the given
// column.
func exportHasColumn(columns []lists.Column, id lists.ColumnID) bool {
	for _, column := range columns {
		if column.ID == id {
			return true
		}
	}
	return false
}

// departureExportCell renders the per-weekday departure plan and appends the
// coupled "mit wem" detail whenever the plan allows the accompanied ("Mit
// anderem Kind") mode, so offline pickup/weekly lists carry the actionable
// "with whom" information staff need to act on (#1694).
//
// Both sources of that detail are rendered, structured links first: since links
// satisfy the accompanied-requires-a-note rule per weekday, a child that walks
// in a Laufgemeinschaft legitimately has NO note at all, and a cell built from
// the note alone would print "Mit anderem Kind" and leave the paper list — the
// one staff use when the app is not at hand — without a single name.
func departureExportCell(student StudentResponse) string {
	summary := departureSummary(student.AllowedDepartureModes, student.DepartureDays)
	details := make([]string, 0, 2)
	if companions := departure.FormatCompanionLinks(student.DepartureCompanions); companions != "" {
		details = append(details, companions)
	}
	if student.DepartureCompanionNote != "" {
		details = append(details, student.DepartureCompanionNote)
	}
	if len(details) == 0 {
		return summary
	}
	allowed := student.AllowedDepartureModes.Normalize()
	if !allowed.HasAny() {
		allowed = departure.AllowedDepartureModesFromDeparture(student.DepartureDays)
	}
	if !allowed.HasMode(departure.DepartureAccompanied) {
		return summary
	}
	return summary + " (mit: " + strings.Join(details, "; ") + ")"
}

// departureSummary renders the per-weekday departure plan for the export, e.g.
// "Mo: Bus, Mi: Abholung". Alone/unset days are omitted; an all-alone plan
// renders "Geht alleine" (#1610).
func departureSummary(allowed departure.AllowedDepartureModes, fallback departure.DepartureDays) string {
	modeLabels := map[departure.DepartureMode]string{
		departure.DepartureAlone:       "zu Fuß",
		departure.DepartureBus:         "Bus",
		departure.DeparturePickup:      "Abholung",
		departure.DepartureAccompanied: "Mit anderem Kind",
	}
	shortDay := map[string]string{
		departure.PickupDayMonday:    "Mo",
		departure.PickupDayTuesday:   "Di",
		departure.PickupDayWednesday: "Mi",
		departure.PickupDayThursday:  "Do",
		departure.PickupDayFriday:    "Fr",
	}
	allowed = allowed.Normalize()
	if !allowed.HasAny() {
		allowed = departure.AllowedDepartureModesFromDeparture(fallback)
	}
	parts := make([]string, 0, len(departure.PickupDayOrder))
	for _, day := range departure.PickupDayOrder {
		modes := allowed[day]
		if len(modes) == 0 {
			continue
		}
		labels := make([]string, 0, len(modes))
		for _, mode := range modes {
			labels = append(labels, modeLabels[mode])
		}
		parts = append(parts, shortDay[day]+": "+strings.Join(labels, ", "))
	}
	if len(parts) == 0 {
		return "Geht alleine"
	}
	return strings.Join(parts, ", ")
}

func weeklyCell(plan weeklySchedule, weekday int) string {
	arrival := plan.ArrivalByWeekday[weekday]
	pickup := plan.PickupByWeekday[weekday]
	if arrival == "" && pickup == "" && !plan.CareDaysByWeekday[weekday] {
		return "nein"
	}
	if arrival != "" && pickup != "" {
		return "Ankunft: " + arrival + ", Abholung: " + pickup
	}
	if arrival != "" {
		return "Ankunft: " + arrival
	}
	if pickup == "" {
		return "Ankunft: keine Zeit"
	}
	return "Abholung: " + pickup
}

func careDays(plan weeklySchedule) string {
	labels := []string{}
	for _, day := range []struct {
		weekday int
		label   string
	}{
		{weekdayMonday, "Mo"},
		{weekdayTuesday, "Di"},
		{weekdayWednesday, "Mi"},
		{weekdayThursday, "Do"},
		{weekdayFriday, "Fr"},
	} {
		if plan.CareDaysByWeekday[day.weekday] || plan.ArrivalByWeekday[day.weekday] != "" || plan.PickupByWeekday[day.weekday] != "" {
			labels = append(labels, day.label)
		}
	}
	if len(labels) == 0 {
		return "keine"
	}
	return strings.Join(labels, ", ")
}

func dailyNotes(student StudentResponse) string {
	notes := []string{}
	if student.ArrivalNotes != "" {
		notes = append(notes, "Ankunft: "+student.ArrivalNotes)
	}
	if student.PickupNotes != "" {
		notes = append(notes, "Abholung: "+student.PickupNotes)
	}
	return strings.Join(notes, "; ")
}

func collectResponseIDs(students []StudentResponse) []int64 {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		if student.HasFullAccess {
			ids = append(ids, student.ID)
		}
	}
	return ids
}

func (rs *Resource) exportSubtitle(r *http.Request, count int) string {
	name := "Alle Kinder"
	if tenantID := tenant.FromContext(r.Context()); tenantID > 0 && rs.SchoolService != nil {
		if school, err := rs.SchoolService.GetSchoolByID(r.Context(), tenantID); err == nil && school != nil && school.Name != "" {
			name = school.Name
		}
	}
	return fmt.Sprintf("%s - %d Kinder", name, count)
}

func exportTitle(req studentExportRequest) string {
	title := strings.TrimSpace(req.Title)
	if title != "" {
		return title
	}
	switch req.Preset {
	case lists.PresetOGSCompact:
		return "OGS Kompaktliste"
	case lists.PresetClassRoster:
		return "Klassenliste"
	case lists.PresetDailyPlanning:
		// "Tagesplanung", nicht "Tagesliste": der Name kollidierte mit den
		// slot-basierten Tageslisten aus dem Betreuungsplan (#1565).
		return "Tagesplanung"
	case lists.PresetAttendanceSnapshot:
		return "Anwesenheitsliste"
	case lists.PresetPickupList:
		return "Abholliste"
	case lists.PresetBlankChecklist:
		return "Checkliste"
	case lists.PresetBirthdayList:
		return "Geburtstagsliste"
	case lists.PresetHealthList:
		return "Gesundheitsliste"
	default:
		return "OGS Wochenliste"
	}
}

func exportFilterLabelsForDate(filters studentExportFilters, planningDate timezone.Date, isToday bool) []string {
	labels := exportIdentityFilterLabels(filters, planningDate, isToday)
	return append(labels, exportAttributeFilterLabels(filters, isToday)...)
}

// exportIdentityFilterLabels names the "who / which" filters for the printed
// header: the planning date, free-text search, group, school year, class, and
// the momentary status snapshot.
func exportIdentityFilterLabels(filters studentExportFilters, planningDate timezone.Date, isToday bool) []string {
	labels := []string{}
	if !isToday {
		labels = append(labels, "Datum: "+planningDate.Format("02.01.2006"))
	}
	if filters.Search != "" {
		labels = append(labels, "Suche: "+filters.Search)
	}
	if filters.GroupID != "" {
		labels = append(labels, "Gruppe gefiltert")
	}
	if years := exportYearFilterValues(filters.Year); len(years) > 0 {
		labels = append(labels, "Stufe: "+strings.Join(years, ", "))
	}
	if classes := parseMultiValueParam([]string{filters.SchoolClass}); len(classes) > 0 {
		labels = append(labels, "Klasse: "+strings.Join(classes, ", "))
	}
	if filters.Status != "" && filters.Status != "all" {
		// Only the location-derived buckets are a snapshot of right now; on a
		// dated export the remaining ones (krank/klassenfahrt/entschuldigt) come
		// from that day's status days, so calling them a Momentaufnahme would
		// mislabel a plan (#1939).
		labels = append(labels, dayLabel("Momentaufnahme: ", "Geplanter Status: ", isToday)+exportStatusLabel(filters.Status))
	}
	return labels
}

// exportAttributeFilterLabels names the per-child attribute filters for the
// printed header: bus, photo consent, pickup rule, day planning, class grouping,
// and birthday months.
func exportAttributeFilterLabels(filters studentExportFilters, isToday bool) []string {
	labels := []string{}
	if label := binaryFilterLabel(filters.Bus, "Buskind", "Kein Buskind"); label != "" {
		labels = append(labels, label)
	}
	if label := binaryFilterLabel(filters.PhotoConsent, "Fotoerlaubnis liegt vor", "Keine Fotoerlaubnis"); label != "" {
		labels = append(labels, label)
	}
	if filters.PickupStatus != "" && filters.PickupStatus != "all" {
		labels = append(labels, "Abholregelung: "+exportPickupStatusLabel(filters.PickupStatus))
	}
	if filters.DayStatus != "" && filters.DayStatus != DayPlanningStatusAll {
		labels = append(labels, "Tagesplanung: "+dayStatusExportLabel(filters.DayStatus, isToday))
	}
	if filters.GroupByClass {
		labels = append(labels, "Nach Klassen getrennt")
	}
	if label := birthdayMonthFilterLabel(filters.Months); label != "" {
		labels = append(labels, label)
	}
	return labels
}

// binaryFilterLabel names a yes/no filter for the printed header, or "" when
// the filter is inactive.
func binaryFilterLabel(value, yesLabel, noLabel string) string {
	if value == "" || value == "all" {
		return ""
	}
	if value == "yes" {
		return yesLabel
	}
	return noLabel
}

var germanMonthNames = [12]string{
	"Januar", "Februar", "März", "April", "Mai", "Juni",
	"Juli", "August", "September", "Oktober", "November", "Dezember",
}

// birthdayMonthFilterLabel names the selected birth months chronologically,
// independent of the order they arrived in, so the printed header matches the
// order of the list below it.
func birthdayMonthFilterLabel(values []string) string {
	months, err := parseExportMonths(values)
	if err != nil || len(months) == 0 {
		return ""
	}
	names := make([]string, 0, len(months))
	for month := time.January; month <= time.December; month++ {
		if months[month] {
			names = append(names, germanMonthNames[month-1])
		}
	}
	if len(names) == 1 {
		return "Geburtsmonat: " + names[0]
	}
	return "Geburtsmonate: " + strings.Join(names, ", ")
}

func exportPickupStatusLabel(status string) string {
	switch status {
	case "self":
		return "Geht alleine nach Hause"
	case "pickedUp":
		return "Wird abgeholt"
	case "none":
		return "Keine Abholregelung"
	default:
		return "Sonstige"
	}
}

func exportStatusLabel(status string) string {
	switch status {
	case "krank":
		return "Krank"
	case "klassenfahrt":
		return "Klassenfahrt"
	case "entschuldigt":
		return "Entschuldigt"
	case "abwesend":
		return "Abwesend"
	case "schule":
		return "Schule"
	case "unterwegs":
		return "Unterwegs"
	case "schulhof":
		return "Schulhof"
	case "anwesend":
		return "Anwesend"
	default:
		return status
	}
}

func dayStatusExportLabel(status string, isToday bool) string {
	switch status {
	case DayPlanningStatusComesToday:
		return dayLabel("Kommt heute", "Wird erwartet", isToday)
	case DayPlanningStatusNotComingToday:
		return dayLabel("Kommt heute nicht", "Wird nicht erwartet", isToday)
	default:
		return status
	}
}

func schoolYear(schoolClass string) string {
	for _, r := range schoolClass {
		if r >= '1' && r <= '9' {
			return string(r)
		}
	}
	return ""
}

func exportStatus(student StudentResponse) string {
	if student.ClassTrip {
		return "klassenfahrt"
	}
	if student.Sick {
		return "krank"
	}
	if student.Excused {
		return "entschuldigt"
	}
	switch student.Location {
	// The resolver emits "Abwesend"; "Zuhause" and "" cover dated exports and
	// older callers. Without the "Abwesend" case every child at home fell
	// through to "anwesend".
	case common.AbsentLocationLabel, "Zuhause", "":
		return "abwesend"
	case common.AtSchoolLocationLabel:
		return "schule"
	case "Unterwegs":
		return "unterwegs"
	case "Schulhof":
		return "schulhof"
	default:
		return "anwesend"
	}
}

func timeValue(value *string) string {
	if value == nil || *value == "" {
		return "99:99"
	}
	return *value
}

func formatWallClock(value time.Time) string {
	return timezone.NormalizeWallClock(value).Format("15:04")
}
