package services

import (
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// NewAttendanceActivityCategories exposes only the dashboard's category total.
func NewAttendanceActivityCategories(records timetableCompose.CategoryCountRecords) presenceservice.AttendanceActivityCategories {
	return timetableCompose.NewCategoryCount(records)
}
