package services

import (
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/services/active"
)

// NewAttendanceActivityCategories exposes only the dashboard's category total.
func NewAttendanceActivityCategories(records timetableCompose.CategoryCountRecords) active.AttendanceActivityCategories {
	return timetableCompose.NewCategoryCount(records)
}
