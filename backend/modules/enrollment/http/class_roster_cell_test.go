package enrollmenthttp

import (
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// classRosterWeeklyCell renders one weekday cell of an enrollment row the way
// buildClassRosterTableDocument does, so the cell tests read one day at a time.
func classRosterWeeklyCell(row capability.ClassRosterRow, day string) string {
	return lists.ClassRosterDayCell(classRosterDay(row, day))
}
