package timetracking

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The retained vocabulary these services still speak. The package names the
// calendar-date contract it depends on and declares its own record base, so
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

// Model is the identity and timestamps every time record carries.
type Model struct {
	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

func (m *Model) GetID() any              { return m.ID }
func (m *Model) GetCreatedAt() time.Time { return m.CreatedAt }
func (m *Model) GetUpdatedAt() time.Time { return m.UpdatedAt }

// TenantModel is the school a time record belongs to.
type TenantModel struct {
	TenantID int64 `bun:"tenant_id,notnull" json:"tenant_id"`
}

func (t *TenantModel) GetTenantID() int64   { return t.TenantID }
func (t *TenantModel) SetTenantID(id int64) { t.TenantID = id }

// ErrNotFound is the missing-row result of the time record repositories. It
// carries the repository not-found marker, the shape the row adapters'
// missing rows have too, so isRecordNotFound classifies both alike.
var ErrNotFound error = recordNotFoundError{}

type recordNotFoundError struct{}

func (recordNotFoundError) Error() string { return "repository: not found" }

// RepositoryNotFound marks the not-found result by shape.
func (recordNotFoundError) RepositoryNotFound() {}

// isRecordNotFound reports whether err carries the repository not-found
// marker anywhere in its chain.
func isRecordNotFound(err error) bool {
	var marker interface{ RepositoryNotFound() }
	return errors.As(err, &marker)
}

// driverError is the shape of a PostgreSQL driver error: its message fields
// by their protocol code and its integrity-constraint class.
type driverError interface {
	error
	Field(byte) string
	IntegrityViolation() bool
}

// isUniqueViolationOn reports whether PostgreSQL rejected the write on the
// named unique index or constraint; the row adapters keep the driver error in
// the chain. A unique index or constraint raises only a unique_violation, so
// the integrity class together with its name identifies it.
func isUniqueViolationOn(err error, constraint string) bool {
	var pgErr driverError
	return constraint != "" && errors.As(err, &pgErr) && pgErr.IntegrityViolation() && pgErr.Field('n') == constraint
}

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
