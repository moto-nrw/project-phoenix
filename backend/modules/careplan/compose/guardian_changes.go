package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// GuardianAbsenceRecords and GuardianPickupRecords are satisfied by
// *careplan.Module.
type GuardianAbsenceRecords = ports.GuardianAbsenceRecords
type GuardianPickupRecords = ports.GuardianPickupRecords

func NewGuardianAbsences(records GuardianAbsenceRecords) (careplan.GuardianAbsenceReports, error) {
	if records == nil {
		return nil, errors.New("guardian absences: records are required")
	}
	return &application.GuardianAbsences{Records: records}, nil
}

// NewGuardianPickupExceptions accepts a nil excusal; the pickup leg is then
// written without deriving or releasing block excusals.
func NewGuardianPickupExceptions(records GuardianPickupRecords, excusal careplan.PickupAutoExcusal) (careplan.GuardianPickupExceptions, error) {
	if records == nil {
		return nil, errors.New("guardian pickup exceptions: records are required")
	}
	// A nil interface converts to a nil port, which skips the coupling.
	return &application.GuardianPickupExceptions{Records: records, Excusal: excusal, IsUniqueViolation: postgres.IsPickupExceptionConflict}, nil
}
