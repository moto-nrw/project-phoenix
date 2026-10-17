package compose

import (
	"database/sql"
	"errors"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
)

// The root wraps the absence rows with the School Membership and People
// Directory lookups of the request listing and names them through the
// aliases below.

// The staff absence row contracts the root's decorators and their suites name.
type (
	StaffAbsence                = timetracking.StaffAbsence
	StaffAbsenceRepository      = timetracking.StaffAbsenceRepository
	StaffAbsenceAuditRepository = timetracking.StaffAbsenceAuditRepository
	AbsenceRequestFilter        = timetracking.AbsenceRequestFilter
	AbsenceRequestRow           = timetracking.AbsenceRequestRow
)

// NewStaffAbsenceRepository serves StaffAbsenceRepository from the Workforce
// capability.
func NewStaffAbsenceRepository(capability workforce.Capability) StaffAbsenceRepository {
	return timetracking.NewStaffAbsenceRows(capability, legacyRowErrors())
}

// NewStaffAbsenceAuditRepository serves StaffAbsenceAuditRepository from the
// Workforce capability.
func NewStaffAbsenceAuditRepository(capability workforce.Capability) StaffAbsenceAuditRepository {
	return timetracking.NewStaffAbsenceAuditRows(capability, legacyRowErrors())
}

// legacyRowErrors binds the error shapes of the retained generic
// repositories (models/base) the capability row adapters restore.
func legacyRowErrors() timetracking.RowErrorShapes {
	return timetracking.RowErrorShapes{
		Database: func(op string, err error) error { return &modelBase.DatabaseError{Op: op, Err: err} },
		Missing:  errors.Join(modelBase.ErrNotFound, sql.ErrNoRows),
	}
}

// ErrRowNotFound is the retained repositories' not-found sentinel the row
// adapters wrap (models/base.ErrNotFound).
var ErrRowNotFound = modelBase.ErrNotFound
