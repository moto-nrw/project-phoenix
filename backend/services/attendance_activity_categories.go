package services

import (
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// NewAttendanceActivityCategories exposes only the dashboard's category total.
func NewAttendanceActivityCategories(records timetableCompose.CategoryCountRecords) active.AttendanceActivityCategories {
	return timetableCompose.NewCategoryCount(records)
}
