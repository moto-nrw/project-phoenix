package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// The operational day, the timetable data reads and the attendance
// corrections live in internal/application (#4084). The roots build them
// through the constructors below, with their dependency types from
// modules/timetable/ports; the composition binds the caller port to the
// request's session the root binds (operationCaller).

// NewOperations builds the operational day with the caller bound to the
// request's session.
func NewOperations(deps ports.OperationDependencies) (timetable.OperationCapability, error) {
	return application.NewOperations(deps, operationCaller{RequestSession: deps.Session})
}

func NewTimetableData(deps ports.TimetableDataDependencies) (timetable.TimetableDataCapability, error) {
	return application.NewTimetableData(deps)
}

func NewAttendanceCorrections(deps ports.AttendanceCorrectionDependencies) (timetable.AttendanceCorrections, error) {
	return application.NewAttendanceCorrections(deps)
}

func NewAttendanceHistory(records ports.AttendanceHistoryRecords) ports.AttendanceHistory {
	return application.NewAttendanceHistory(records)
}
