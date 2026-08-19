package students

import (
	"net/http"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// This file merges the class-list-only entries (#2382) into the student
// database "Klassenliste" export: the same children the phase class roster
// shows must appear here too, or the two exports both called "Klassenliste"
// disagree about the Klassenverband.

// classListEntryExportEligible reports whether the export may carry list
// entries at all. Entries only exist as name + class, so any active filter on
// a property they don't have (group, room, lifecycle status, bus, photo
// consent, pickup, day status, times, birthday months) excludes every entry
// semantically — the merged list would claim they matched the filter. The
// enum filters share the student side's isActiveFilterValue semantics: an
// explicit "all" filters nothing there, so it must not drop the entries here.
func classListEntryExportEligible(preset listexport.Preset, f studentExportFilters) bool {
	if preset != listexport.PresetClassRoster {
		return false
	}
	return f.GroupID == "" && f.RoomID == "" &&
		!isActiveFilterValue(f.Status) && !isActiveFilterValue(f.Bus) &&
		!isActiveFilterValue(f.PhotoConsent) && !isActiveFilterValue(f.PickupStatus) &&
		!isActiveFilterValue(f.DayStatus) &&
		f.PickupTime == "" && f.ArrivalTime == "" && len(f.Months) == 0
}

// classListEntriesForExport loads and filters the entries for the export:
// the search matches names, the class filter (comma-separated, #2218) and
// the year filter match the class string — the same dimensions the student
// side filters by.
func (rs *Resource) classListEntriesForExport(r *http.Request, filters studentExportFilters) ([]*userModels.ClassListEntry, error) {
	if rs.ClassListEntryService == nil {
		return nil, nil
	}
	entries, err := rs.ClassListEntryService.ListAll(r.Context())
	if err != nil {
		return nil, err
	}
	classFilter := map[string]bool{}
	for _, class := range strings.Split(filters.SchoolClass, ",") {
		if key := schoolclass.Normalize(class); key != "" {
			classFilter[key] = true
		}
	}
	kept := make([]*userModels.ClassListEntry, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if len(classFilter) > 0 && !classFilter[schoolclass.Normalize(entry.SchoolClass)] {
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

func classListEntryMatchesSearch(entry *userModels.ClassListEntry, search string) bool {
	if search == "" {
		return true
	}
	fullName := entry.FirstName + " " + entry.LastName
	return strutil.ContainsFold(entry.FirstName, search) ||
		strutil.ContainsFold(entry.LastName, search) ||
		strutil.ContainsFold(fullName, search)
}

// classListEntryExportRow renders one entry: name with the "Keine Betreuung"
// marker, class, and "—" in the weekday cells like a non-care day of a
// student row. Every other column stays empty — the entry has no data there,
// and an invented value is exactly what #2382 forbids.
func classListEntryExportRow(entry *userModels.ClassListEntry) listexport.Row {
	return listexport.Row{Values: map[listexport.ColumnID]string{
		listexport.ColumnName:              strings.TrimSpace(entry.FirstName+" "+entry.LastName) + " (" + enrollmentService.ClassListEntryNoCareLabel + ")",
		listexport.ColumnSchoolClass:       entry.SchoolClass,
		listexport.ColumnEnrollmentSummary: enrollmentService.ClassListEntryNoCareLabel,
		listexport.ColumnWeeklyMonday:      "—",
		listexport.ColumnWeeklyTuesday:     "—",
		listexport.ColumnWeeklyWednesday:   "—",
		listexport.ColumnWeeklyThursday:    "—",
		listexport.ColumnWeeklyFriday:      "—",
	}}
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
	row      listexport.Row
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
	return mergeClassListEntryRowSources(sources, entrySources, exportSortMode(req) == "", req.Filters.GroupByClass), nil
}

// mergeClassListEntryRowSources interleaves entry rows into the
// already-sorted student rows. Class ordering is applied ONLY where the
// document shape requires it — a grouped export, whose class headings depend
// on class-contiguous rows. An ungrouped export keeps the order the requested
// sort mode produced: name-sorted documents interleave the entries by German
// name collation, and a time/birthday sort (which an entry has no value for)
// keeps the student order untouched and appends the entries at the end,
// class-then-name sorted among themselves.
func mergeClassListEntryRowSources(students []exportRowSource, entries []exportRowSource, nameSorted, grouped bool) []exportRowSource {
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
		stableSortRowSources(merged, nameSorted)
		return merged
	}
	if nameSorted {
		merged := make([]exportRowSource, 0, len(students)+len(entries))
		merged = append(merged, students...)
		merged = append(merged, entries...)
		sortRowSourcesByName(merged)
		return merged
	}
	stableSortRowSources(entries, true)
	return append(students, entries...)
}

// sortRowSourcesByName orders by German name collation only — the ungrouped,
// name-sorted document shape. Students win ties against entries.
func sortRowSourcesByName(sources []exportRowSource) {
	sort.SliceStable(sources, func(i, j int) bool {
		a, b := sources[i], sources[j]
		if r := collation.CompareGermanNames(a.lastName, a.firstName, b.lastName, b.firstName); r != 0 {
			return r < 0
		}
		if a.entryTie != b.entryTie {
			return !a.entryTie
		}
		return false
	})
}

func stableSortRowSources(sources []exportRowSource, nameSorted bool) {
	sort.SliceStable(sources, func(i, j int) bool {
		a, b := sources[i], sources[j]
		if r := collation.CompareSchoolClasses(a.schoolClass, b.schoolClass); r != 0 {
			return r < 0
		}
		if nameSorted {
			if r := collation.CompareGermanNames(a.lastName, a.firstName, b.lastName, b.firstName); r != 0 {
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
// merged sources, reusing the exact heading semantics of
// buildGroupedExportRows.
func buildExportRowSources(sources []exportRowSource, grouped bool) []listexport.Row {
	rows := make([]listexport.Row, 0, len(sources))
	currentClass := ""
	for i, source := range sources {
		if class := strings.TrimSpace(source.schoolClass); grouped && (i == 0 || collation.CompareSchoolClasses(class, currentClass) != 0) {
			currentClass = class
			rows = append(rows, listexport.Row{GroupTitle: listexport.ClassGroupTitle(class)})
		}
		rows = append(rows, source.row)
	}
	return rows
}
