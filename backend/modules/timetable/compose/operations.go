package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
)

// The operational day, the timetable data reads and the attendance
// corrections live in internal/application (#4084). The roots build them
// through the constructors and dependency types below; the composition binds
// the caller port to the request's session (operationCaller).

type (
	AdvisoryLocks                    = application.AdvisoryLocks
	AttendanceAnnouncer              = application.AttendanceAnnouncer
	AttendanceCorrectionDependencies = application.AttendanceCorrectionDependencies
	AttendanceCorrectionRecord       = application.AttendanceCorrectionRecord
	AttendanceCorrectionTrail        = application.AttendanceCorrectionTrail
	AttendanceHistory                = application.AttendanceHistory
	AttendanceHistoryRecords         = application.AttendanceHistoryRecords
	AttendanceLocks                  = application.AttendanceLocks
	BlockDisplayMetadata             = application.BlockDisplayMetadata
	ClassArrivalNotice               = application.ClassArrivalNotice
	CorrectionPeople                 = application.CorrectionPeople
	DataArrivalExceptions            = application.DataArrivalExceptions
	DataCategories                   = application.DataCategories
	DataGroups                       = application.DataGroups
	DataInstances                    = application.DataInstances
	DataParticipants                 = application.DataParticipants
	DataPickupExceptions             = application.DataPickupExceptions
	DataTemplates                    = application.DataTemplates
	DeviationEventReader             = application.DeviationEventReader
	EducationGroupNames              = application.EducationGroupNames
	ExpectedArrival                  = application.ExpectedArrival
	OperationAttendance              = application.OperationAttendance
	OperationDependencies            = application.OperationDependencies
	OperationInstanceStaff           = application.OperationInstanceStaff
	OperationInstances               = application.OperationInstances
	OperationLifecycle               = application.OperationLifecycle
	OperationParticipants            = application.OperationParticipants
	OperationPeople                  = application.OperationPeople
	OperationSessions                = application.OperationSessions
	OperationStudents                = application.OperationStudents
	OperationSupervisions            = application.OperationSupervisions
	OperationTemplates               = application.OperationTemplates
	OperationVisits                  = application.OperationVisits
	PickupBaselineProjector          = application.PickupBaselineProjector
	PickupBaselines                  = application.PickupBaselines
	RoomNames                        = application.RoomNames
	RoomOccupancy                    = application.RoomOccupancy
	ScopedAction                     = application.ScopedAction
	TimetableDataDependencies        = application.TimetableDataDependencies
)

// The operational actions with their own school scope setting.
const (
	ScopedAttendance    = application.ScopedAttendance
	ScopedBlockStart    = application.ScopedBlockStart
	ScopedBlockComplete = application.ScopedBlockComplete
)

// NewOperations builds the operational day with the caller bound to the
// request's session token.
func NewOperations(deps OperationDependencies) (timetable.OperationCapability, error) {
	return application.NewOperations(deps, operationCaller{})
}

func NewTimetableData(deps TimetableDataDependencies) (timetable.TimetableDataCapability, error) {
	return application.NewTimetableData(deps)
}

func NewAttendanceCorrections(deps AttendanceCorrectionDependencies) (timetable.AttendanceCorrections, error) {
	return application.NewAttendanceCorrections(deps)
}

func NewAttendanceHistory(records AttendanceHistoryRecords) *AttendanceHistory {
	return application.NewAttendanceHistory(records)
}
