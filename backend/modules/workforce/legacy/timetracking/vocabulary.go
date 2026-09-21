package timetracking

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
)

// The retained vocabulary these services still speak. The package names the
// calendar-date, persistence-model and shared-base contracts it depends on so
// its own tests and fixtures build them without reaching past the package;
// every entry goes with the retained service that uses it.

// Calendar dates and wall-clock helpers.
type Date = timezone.Date

var (
	Berlin             = timezone.Berlin
	NewDate            = timezone.NewDate
	DateFromTime       = timezone.DateFromTime
	TodayDate          = timezone.TodayDate
	NormalizeWallClock = timezone.NormalizeWallClock
)

// Shared persistence base: the embedded model row, list options and the
// repository's not-found sentinel.
type (
	Model        = base.Model
	QueryOptions = base.QueryOptions
)

var (
	ErrNotFound = base.ErrNotFound
	SortDesc    = base.SortDesc
)

// Retained rows and repository contracts: the Workforce time records and the
// Student Presence supervision rows.
type (
	AbsenceRequestFilter             = timerecords.AbsenceRequestFilter
	AbsenceRequestRow                = timerecords.AbsenceRequestRow
	Group                            = studentpresence.LiveGroup
	GroupSupervisor                  = studentpresence.GroupSupervision
	GroupRepository                  = studentpresence.SessionRecords
	GroupSupervisorRepository        = studentpresence.SupervisionRecords
	StaffAbsence                     = timerecords.StaffAbsence
	StaffAbsenceAudit                = timerecords.StaffAbsenceAudit
	StaffAbsenceType                 = timerecords.StaffAbsenceType
	StaffBalanceAdjustment           = timerecords.StaffBalanceAdjustment
	StaffBalanceAdjustmentRepository = timerecords.StaffBalanceAdjustmentRepository
	StaffRoomSupervision             = studentpresence.StaffRoomSupervision
	StaffVacationOpening             = timerecords.StaffVacationOpening
	StaffVacationQuota               = timerecords.StaffVacationQuota
	SupervisionBlocker               = studentpresence.SupervisionBlocker
	WorkSession                      = timerecords.WorkSession
	WorkSessionBreak                 = timerecords.WorkSessionBreak
)

// Canonical Workforce enumerations.
const (
	AbsenceStatusApproved  = workforce.AbsenceStatusApproved
	AbsenceStatusCanceled  = workforce.AbsenceStatusCanceled
	AbsenceStatusDeclined  = workforce.AbsenceStatusDeclined
	AbsenceStatusQuestion  = workforce.AbsenceStatusQuestion
	AbsenceStatusReported  = workforce.AbsenceStatusReported
	AbsenceStatusRequested = workforce.AbsenceStatusRequested

	AbsenceTypeCompTime = workforce.AbsenceTypeCompTime
	AbsenceTypeOther    = workforce.AbsenceTypeOther
	AbsenceTypeSick     = workforce.AbsenceTypeSick
	AbsenceTypeTraining = workforce.AbsenceTypeTraining
	AbsenceTypeVacation = workforce.AbsenceTypeVacation

	BalanceAdjustmentTypeCompTime = workforce.BalanceAdjustmentTypeCompTime
	BalanceAdjustmentTypeOpening  = workforce.BalanceAdjustmentTypeOpening
	BalanceAdjustmentTypePayout   = workforce.BalanceAdjustmentTypePayout
	BalanceAdjustmentTypeReset    = workforce.BalanceAdjustmentTypeReset

	WorkSessionSourceApp     = workforce.WorkSessionSourceApp
	WorkSessionSourceNFC     = workforce.WorkSessionSourceNFC
	WorkSessionSourceUnknown = workforce.WorkSessionSourceUnknown

	WorkSessionStatusHomeOffice = workforce.WorkSessionStatusHomeOffice
	WorkSessionStatusPresent    = workforce.WorkSessionStatusPresent
)
