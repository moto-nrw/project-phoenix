package presence

import "github.com/moto-nrw/project-phoenix/modules/studentpresence"

// The records, results and status-day values below are the owner's public
// contract; the application names them locally so the service code reads the
// same as before it left the legacy nest (#3422).
type (
	DatabaseHandle                = studentpresence.DatabaseHandle
	StudentLifecycle              = studentpresence.StudentLifecycle
	StudentRecord                 = studentpresence.StudentRecord
	StatusDayStudents             = studentpresence.StatusDayStudents
	PersonName                    = studentpresence.PersonName
	StatusDayWriteContext         = studentpresence.StatusDayWriteContext
	StatusDayOverviewGroup        = studentpresence.StatusDayOverviewGroup
	StatusDayOverviewEntry        = studentpresence.StatusDayOverviewEntry
	StatusDayOverviewFilters      = studentpresence.StatusDayOverviewFilters
	StatusDayOverview             = studentpresence.StatusDayOverview
	HistorySlot                   = studentpresence.HistorySlot
	HistorySlotInstance           = studentpresence.HistorySlotInstance
	HistorySlotAttendance         = studentpresence.HistorySlotAttendance
	VisitHistoryEntry             = studentpresence.VisitHistoryEntry
	VisitWithRoom                 = studentpresence.VisitWithRoom
	VisitRoomGroup                = studentpresence.VisitRoomGroup
	VisitRoom                     = studentpresence.VisitRoom
	AttendanceSnapshot            = studentpresence.AttendanceSnapshot
	AttendanceSyncer              = studentpresence.AttendanceSyncer
	SchoolCheckinBatchItem        = studentpresence.SchoolCheckinBatchItem
	SchoolCheckinBatchResult      = studentpresence.SchoolCheckinBatchResult
	StudentStatusDayConflictError = studentpresence.StudentStatusDayConflictError
)

// Care lifecycle states of a student record.
const (
	StudentLifecycleOther    = studentpresence.StudentLifecycleOther
	StudentLifecycleActive   = studentpresence.StudentLifecycleActive
	StudentLifecycleInactive = studentpresence.StudentLifecycleInactive
	StudentLifecycleAlumnus  = studentpresence.StudentLifecycleAlumnus
)

// Status-day write sentinels and limits.
var (
	ErrStudentStatusDayReassigned             = studentpresence.ErrStudentStatusDayReassigned
	ErrStudentStatusDayPartialAbsenceConflict = studentpresence.ErrStudentStatusDayPartialAbsenceConflict
)

const MaxStudentStatusDayConflictDetails = studentpresence.MaxStudentStatusDayConflictDetails

// School check-in actions shared by the single and batch endpoints.
const (
	SchoolCheckinActionIn  = studentpresence.SchoolCheckinActionIn
	SchoolCheckinActionOut = studentpresence.SchoolCheckinActionOut
)

// Presence modes resolved from the tenant's presence_mode setting.
const (
	PresenceModeDetailed = studentpresence.PresenceModeDetailed
	PresenceModeBinary   = studentpresence.PresenceModeBinary
)
