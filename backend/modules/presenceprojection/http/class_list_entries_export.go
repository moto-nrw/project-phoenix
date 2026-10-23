package presenceprojectionhttp

import (
	http "net/http"
	slices "slices"
	sort "sort"
	strings "strings"

	lists "github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	departure "github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// classListEntryExportEligible reports whether the export may carry list
// entries at all. Entries only exist as name + class, so any active filter on
// a property they don't have (group, room, lifecycle status, bus, photo
// consent, pickup, day status, times, birthday months) excludes every entry
// semantically — the merged list would claim they matched the filter. The
// enum filters share the student side's isActiveFilterValue semantics: an
// explicit "all" filters nothing there, so it must not drop the entries here.
func classListEntryExportEligible(preset lists.Preset, f studentExportFilters) bool {
	if preset != lists.PresetClassRoster {
		return false
	}
	return f.GroupID == "" && f.RoomID == "" &&
		!isActiveFilterValue(f.Status) && !isActiveFilterValue(f.Bus) &&
		!isActiveFilterValue(f.PhotoConsent) && !isActiveFilterValue(f.PickupStatus) &&
		!isActiveFilterValue(f.DayStatus) &&
		f.PickupTime == "" && f.ArrivalTime == "" && len(f.Months) == 0 &&
		len(f.StudentIDs) == 0
}

// classListEntriesForExport loads and filters the entries for the export:
// the search matches names, the class filter (comma-separated, #2218) and
// the year filter match the class string — the same dimensions the student
// side filters by.
func (rs *Resource) classListEntriesForExport(r *http.Request, filters studentExportFilters) ([]peopleModule.StudentClassListEntry, error) {
	if rs.ClassListEntries == nil {
		return nil, nil
	}
	entries, err := rs.ClassListEntries.ListClassListEntriesInDisplayOrder(r.Context())
	if err != nil {
		return nil, err
	}
	classFilter := map[string]bool{}
	for _, class := range strings.Split(filters.SchoolClass, ",") {
		if key := schoolClassKey(class); key != "" {
			classFilter[key] = true
		}
	}
	kept := make([]peopleModule.StudentClassListEntry, 0, len(entries))
	for _, entry := range entries {
		if len(classFilter) > 0 && !classFilter[schoolClassKey(entry.SchoolClass)] {
			continue
		}
		if !matchesExportYearFilter(entry.SchoolClass, filters.Year) {
			continue
		}
		if !classListEntryMatchesSearch(entry, filters.Search) {
			continue
		}
		kept = append(kept, entry)
	}
	return kept, nil
}

func classListEntryMatchesSearch(entry peopleModule.StudentClassListEntry, search string) bool {
	if search == "" {
		return true
	}
	fullName := entry.FirstName + " " + entry.LastName
	return containsFold(entry.FirstName, search) ||
		containsFold(entry.LastName, search) ||
		containsFold(fullName, search)
}

// classListEntryExportRow renders one entry through the shared class-list
// table: name with the "Keine Betreuung" marker, class, and "—" in the
// weekday cells like a non-care day of a student row. Every other column
// stays empty — the entry has no data there, and an invented value is
// exactly what #2382 forbids.
func classListEntryExportRow(entry peopleModule.StudentClassListEntry) lists.Row {
	return lists.ClassRosterRow(lists.ClassRosterChild{
		Name:        entry.FirstName + " " + entry.LastName,
		SchoolClass: entry.SchoolClass,
		NoCare:      true,
		Extra:       map[lists.ColumnID]string{lists.ColumnEnrollmentSummary: lists.ClassRosterNoCareLabel},
	})
}

// classRosterWeekdays are the plan weekdays of the class-list cells, Monday
// first like departure.PickupDayOrder.
var classRosterWeekdays = [5]int{weekdayMonday, weekdayTuesday, weekdayWednesday, weekdayThursday, weekdayFriday}

// applyClassRosterRows re-renders the student rows of the "Klassenliste"
// preset through the shared class-list table (#4749): the weekday cells carry
// the Kind-Gehzeit and the day's Geh-/Abholregelung ("14:30 Uhr, wird
// abgeholt") like the Klassenliste of an enrollment phase. The other columns
// keep the values buildExportRow rendered. sources and students are parallel
// (responseRowSources keeps the order).
func applyClassRosterRows(sources []exportRowSource, students []StudentResponse, weekly map[int64]weeklySchedule) {
	for i, student := range students {
		sources[i].row = classRosterExportRow(student, weekly[student.ID], sources[i].row)
	}
}

func classRosterExportRow(student StudentResponse, plan weeklySchedule, base lists.Row) lists.Row {
	rules := departure.DayRules(student.AllowedDepartureModes, student.DepartureDays, student.DepartureCompanionNote, student.DepartureCompanions)
	child := lists.ClassRosterChild{
		Name:        student.FirstName + " " + student.LastName,
		SchoolClass: student.SchoolClass,
		Extra:       base.Values,
	}
	for i, weekday := range classRosterWeekdays {
		pickup := plan.PickupByWeekday[weekday]
		child.Days[i] = lists.ClassRosterDay{
			// A care day as careDays counts it: a planned arrival or pickup
			// makes the day a care day even without its own marker.
			Care:      plan.CareDaysByWeekday[weekday] || plan.ArrivalByWeekday[weekday] != "" || pickup != "",
			Pickup:    pickup,
			Departure: rules[departure.PickupDayOrder[i]],
		}
	}
	return lists.ClassRosterRow(child)
}

// exportPrintsDepartureRules reports whether the export prints a departure
// rule: the departure column, or a weekday cell of the "Klassenliste".
func exportPrintsDepartureRules(preset lists.Preset, columns []lists.Column) bool {
	if exportHasColumn(columns, lists.ColumnDeparture) {
		return true
	}
	if preset != lists.PresetClassRoster {
		return false
	}
	for _, column := range columns {
		if slices.Contains(lists.ClassRosterWeekdayColumns[:], column.ID) {
			return true
		}
	}
	return false
}

// exportRowSource is one future document row with the sort keys the merge
// needs (rows themselves don't carry name/class separately).
type exportRowSource struct {
	schoolClass string
	lastName    string
	firstName   string
	// entryTie breaks ties between a student and an entry deterministically
	// (students first) without ever comparing IDs across the two kinds.
	entryTie bool
	row      lists.Row
}

// mergeClassListEntrySources loads, filters and interleaves the class-list
// entries into the student row sources when the export is eligible; a
// non-eligible export passes through unchanged.
func (rs *Resource) mergeClassListEntrySources(r *http.Request, req studentExportRequest, sources []exportRowSource) ([]exportRowSource, error) {
	if !classListEntryExportEligible(req.Preset, req.Filters) {
		return sources, nil
	}
	entries, err := rs.classListEntriesForExport(r, req.Filters)
	if err != nil {
		return nil, err
	}
	entrySources := make([]exportRowSource, 0, len(entries))
	for _, entry := range entries {
		entrySources = append(entrySources, exportRowSource{
			schoolClass: entry.SchoolClass,
			lastName:    entry.LastName,
			firstName:   entry.FirstName,
			entryTie:    true,
			row:         classListEntryExportRow(entry),
		})
	}
	return rs.mergeClassListEntryRowSources(sources, entrySources, exportSortMode(req) == "", req.Filters.GroupByClass), nil
}

// mergeClassListEntryRowSources interleaves entry rows into the
// already-sorted student rows. Class ordering is applied ONLY where the
// document shape requires it — a grouped export, whose class headings depend
// on class-contiguous rows. An ungrouped export keeps the order the requested
// sort mode produced: name-sorted documents interleave the entries by German
// name collation, and a time/birthday sort (which an entry has no value for)
// keeps the student order untouched and appends the entries at the end,
// class-then-name sorted among themselves.
func (rs *Resource) mergeClassListEntryRowSources(students []exportRowSource, entries []exportRowSource, nameSorted, grouped bool) []exportRowSource {
	if len(entries) == 0 {
		return students
	}
	if grouped {
		merged := make([]exportRowSource, 0, len(students)+len(entries))
		merged = append(merged, students...)
		merged = append(merged, entries...)
		// Stable: within a class the existing student order (whatever sort
		// mode produced it) survives; entries slot in by name or sink to the
		// block end.
		rs.stableSortRowSources(merged, nameSorted)
		return merged
	}
	if nameSorted {
		merged := make([]exportRowSource, 0, len(students)+len(entries))
		merged = append(merged, students...)
		merged = append(merged, entries...)
		rs.sortRowSourcesByName(merged)
		return merged
	}
	rs.stableSortRowSources(entries, true)
	return append(students, entries...)
}

// sortRowSourcesByName orders by German name collation only — the ungrouped,
// name-sorted document shape. Students win ties against entries.
func (rs *Resource) sortRowSourcesByName(sources []exportRowSource) {
	sort.SliceStable(sources, func(i, j int) bool {
		a, b := sources[i], sources[j]
		if r := rs.CompareGermanNames(a.lastName, a.firstName, b.lastName, b.firstName); r != 0 {
			return r < 0
		}
		if a.entryTie != b.entryTie {
			return !a.entryTie
		}
		return false
	})
}

func (rs *Resource) stableSortRowSources(sources []exportRowSource, nameSorted bool) {
	sort.SliceStable(sources, func(i, j int) bool {
		a, b := sources[i], sources[j]
		if r := rs.CompareSchoolClasses(a.schoolClass, b.schoolClass); r != 0 {
			return r < 0
		}
		if nameSorted {
			if r := rs.CompareGermanNames(a.lastName, a.firstName, b.lastName, b.firstName); r != 0 {
				return r < 0
			}
		}
		if a.entryTie != b.entryTie {
			return !a.entryTie
		}
		return false
	})
}

// buildExportRowSources renders the final rows (grouped or flat) from the
// merged sources with the class sections of the shared class-list table
// (#4749), so a grouped child list and both class lists open each class the
// same way.
func (rs *Resource) buildExportRowSources(sources []exportRowSource, grouped bool) []lists.Row {
	rows := make([]lists.ClassSectionRow, 0, len(sources))
	for _, source := range sources {
		rows = append(rows, lists.ClassSectionRow{SchoolClass: source.schoolClass, Row: source.row})
	}
	return lists.ClassSectionRows(rows, grouped)
}
