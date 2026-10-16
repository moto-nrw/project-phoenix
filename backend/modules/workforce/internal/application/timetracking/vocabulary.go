package timetracking

import (
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The retained vocabulary these services still speak. The package names the
// calendar-date, persistence-model and shared-base contracts it depends on so
// its own tests and fixtures build them without reaching past the package;
// every entry goes with the retained service that uses it.

// Calendar dates and wall-clock helpers.
type Date = calendar.Date

var (
	Berlin             = calendar.Berlin
	NewDate            = calendar.NewDate
	DateFromTime       = calendar.DateFromTime
	TodayDate          = calendar.TodayDate
	NormalizeWallClock = calendar.NormalizeWallClock
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

// The Student Presence supervision rows. The Workforce time records and their
// repository contracts are declared in this package (*_record.go,
// time_record_repositories.go).
type (
	Group                     = studentpresence.LiveGroup
	GroupSupervisor           = studentpresence.GroupSupervision
	GroupRepository           = studentpresence.SessionRecords
	GroupSupervisorRepository = studentpresence.SupervisionRecords
	StaffRoomSupervision      = studentpresence.StaffRoomSupervision
	SupervisionBlocker        = studentpresence.SupervisionBlocker
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
