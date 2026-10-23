package records

import (
	"strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/collation"
)

// The class list exists twice (#4749): the Kinderlisten-Vorlage
// "Klassenliste" reads the live master data, the Klassenliste of an
// enrollment phase reads the enrollments for the coming school year. The
// data sources stay separate; the table below is shared, so both lists print
// the same columns, weekday cells and class sections.

// ClassRosterNoCareLabel marks a class-list-only entry (#2382): a child of the
// Klassenverband with no OGS record at all.
const ClassRosterNoCareLabel = "Keine Betreuung"

// ClassRosterWeekdayColumns are the weekday columns of a class list, Monday
// first.
var ClassRosterWeekdayColumns = [5]ColumnID{
	ColumnWeeklyMonday,
	ColumnWeeklyTuesday,
	ColumnWeeklyWednesday,
	ColumnWeeklyThursday,
	ColumnWeeklyFriday,
}

// ClassRosterColumnIDs are the columns both class lists print, in their
// printed order. A source with more data appends its own columns after them
// (the enrollment list its Erziehungsberechtigte).
func ClassRosterColumnIDs() []ColumnID {
	ids := []ColumnID{ColumnName, ColumnSchoolClass}
	return append(ids, ClassRosterWeekdayColumns[:]...)
}

// ClassRosterDay is one weekday of one child on a class list.
type ClassRosterDay struct {
	// Care marks a care day; every other field is ignored without it.
	Care bool
	// Pickup is the wall-clock pickup time ("14:30"), empty when unknown.
	Pickup string
	// Offerings name the care window for schools whose form encodes it in
	// the offering name ("Betreuung bis 14:30 Uhr"); the fallback when no
	// pickup time is known.
	Offerings []string
	// Departure is the day's Geh-/Abholregelung ("wird abgeholt"), see
	// departure.DayRules.
	Departure string
}

// ClassRosterDayCell renders one weekday cell: "—" outside care days,
// otherwise the pickup time and the day's rule ("14:30 Uhr, wird
// abgeholt"). Without a pickup time the offering names stand in, and
// without those "Keine Abholzeit".
func ClassRosterDayCell(day ClassRosterDay) string {
	if !day.Care {
		return "—"
	}
	cell := "Keine Abholzeit"
	if pickup := strings.TrimSpace(day.Pickup); pickup != "" {
		cell = pickup + " Uhr"
	} else if len(day.Offerings) > 0 {
		cell = strings.Join(day.Offerings, "; ")
	}
	if rule := strings.TrimSpace(day.Departure); rule != "" {
		cell += ", " + rule
	}
	return cell
}

// ClassRosterChild is one child of a class list.
type ClassRosterChild struct {
	Name        string
	SchoolClass string
	// NoCare marks a class-list-only entry (#2382). The marker sits in the
	// name cell because the shared table has no status column; its weekday
	// cells stay "—".
	NoCare bool
	Days   [5]ClassRosterDay
	// Extra carries the values of columns only one source has. The shared
	// columns above win over a key repeated here.
	Extra map[ColumnID]string
}

// ClassRosterRow renders one child into a row of the shared table.
func ClassRosterRow(child ClassRosterChild) Row {
	values := make(map[ColumnID]string, len(child.Extra)+len(ClassRosterWeekdayColumns)+2)
	for id, value := range child.Extra {
		values[id] = value
	}
	name := strings.TrimSpace(child.Name)
	if child.NoCare {
		name += " (" + ClassRosterNoCareLabel + ")"
	}
	values[ColumnName] = name
	values[ColumnSchoolClass] = child.SchoolClass
	for i, id := range ClassRosterWeekdayColumns {
		day := child.Days[i]
		if child.NoCare {
			day = ClassRosterDay{}
		}
		values[id] = ClassRosterDayCell(day)
	}
	return Row{Values: values}
}

// ClassRosterRows renders class-sorted children into the table rows; grouped
// opens a section per class (see ClassSectionRows).
func ClassRosterRows(children []ClassRosterChild, grouped bool) []Row {
	rows := make([]ClassSectionRow, 0, len(children))
	for _, child := range children {
		rows = append(rows, ClassSectionRow{SchoolClass: child.SchoolClass, Row: ClassRosterRow(child)})
	}
	return ClassSectionRows(rows, grouped)
}

// ClassSectionRow is one table row with the class it belongs to.
type ClassSectionRow struct {
	SchoolClass string
	Row         Row
}

// ClassSectionRows returns the rows, and when grouped puts a heading row
// before each class (a new page in the PDF). The rows must arrive
// class-sorted. Boundaries use the sort comparator's equivalence, so label
// variants like "1a"/"1A" share one heading under the first-seen label.
func ClassSectionRows(rows []ClassSectionRow, grouped bool) []Row {
	out := make([]Row, 0, len(rows))
	currentClass := ""
	for i, row := range rows {
		if class := strings.TrimSpace(row.SchoolClass); grouped && (i == 0 || collation.CompareSchoolClasses(class, currentClass) != 0) {
			currentClass = class
			out = append(out, Row{GroupTitle: ClassGroupTitle(class)})
		}
		out = append(out, row.Row)
	}
	return out
}
