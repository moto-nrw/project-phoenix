package timetracking

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
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

// Retained models/active rows and repository contracts.
type (
	AbsenceRequestFilter             = activeModels.AbsenceRequestFilter
	AbsenceRequestRow                = activeModels.AbsenceRequestRow
	Group                            = activeModels.Group
	GroupSupervisor                  = activeModels.GroupSupervisor
	StaffAbsence                     = activeModels.StaffAbsence
	StaffAbsenceAudit                = activeModels.StaffAbsenceAudit
	StaffAbsenceType                 = activeModels.StaffAbsenceType
	StaffBalanceAdjustment           = activeModels.StaffBalanceAdjustment
	StaffBalanceAdjustmentRepository = activeModels.StaffBalanceAdjustmentRepository
	StaffRoomSupervision             = activeModels.StaffRoomSupervision
	StaffVacationOpening             = activeModels.StaffVacationOpening
	StaffVacationQuota               = activeModels.StaffVacationQuota
	SupervisionBlocker               = activeModels.SupervisionBlocker
	WorkSession                      = activeModels.WorkSession
	WorkSessionBreak                 = activeModels.WorkSessionBreak
)

// Retained models/active enumerations.
const (
	AbsenceStatusApproved  = activeModels.AbsenceStatusApproved
	AbsenceStatusCanceled  = activeModels.AbsenceStatusCanceled
	AbsenceStatusDeclined  = activeModels.AbsenceStatusDeclined
	AbsenceStatusQuestion  = activeModels.AbsenceStatusQuestion
	AbsenceStatusReported  = activeModels.AbsenceStatusReported
	AbsenceStatusRequested = activeModels.AbsenceStatusRequested

	AbsenceTypeCompTime = activeModels.AbsenceTypeCompTime
	AbsenceTypeOther    = activeModels.AbsenceTypeOther
	AbsenceTypeSick     = activeModels.AbsenceTypeSick
	AbsenceTypeTraining = activeModels.AbsenceTypeTraining
	AbsenceTypeVacation = activeModels.AbsenceTypeVacation

	BalanceAdjustmentTypeCompTime = activeModels.BalanceAdjustmentTypeCompTime
	BalanceAdjustmentTypeOpening  = activeModels.BalanceAdjustmentTypeOpening
	BalanceAdjustmentTypePayout   = activeModels.BalanceAdjustmentTypePayout
	BalanceAdjustmentTypeReset    = activeModels.BalanceAdjustmentTypeReset

	WorkSessionSourceApp     = activeModels.WorkSessionSourceApp
	WorkSessionSourceNFC     = activeModels.WorkSessionSourceNFC
	WorkSessionSourceUnknown = activeModels.WorkSessionSourceUnknown

	WorkSessionStatusHomeOffice = activeModels.WorkSessionStatusHomeOffice
	WorkSessionStatusPresent    = activeModels.WorkSessionStatusPresent
)
