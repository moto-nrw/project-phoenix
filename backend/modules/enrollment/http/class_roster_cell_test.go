package enrollmenthttp

import (
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// classRosterWeeklyCell renders one weekday cell of an enrollment row the way
// buildClassRosterTableDocument does, so the cell tests read one day at a time.
// The day goes through the shared row in the Monday column.
func classRosterWeeklyCell(row capability.ClassRosterRow, day string) string {
	child := lists.ClassRosterChild{}
	child.Days[0] = classRosterDay(row, day)
	return lists.ClassRosterRow(child).Values[lists.ColumnWeeklyMonday]
}
