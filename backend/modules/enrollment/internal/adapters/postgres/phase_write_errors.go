package postgres

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun/driver/pgdriver"
)

// phaseNameUniqueConstraint is the unique (tenant_id, name) constraint on
// enrollment.phases.
const phaseNameUniqueConstraint = "enrollment_phases_unique_name"

// phaseWriteError keeps the database error's text and marks it with the
// owner sentinel a caller classifies the write by.
type phaseWriteError struct {
	err    error
	reason error
}

func (e *phaseWriteError) Error() string { return e.err.Error() }

func (e *phaseWriteError) Unwrap() []error { return []error{e.err, e.reason} }

// markPhaseWriteError marks a refused phase insert or update: a duplicate
// name as ErrPhaseNameTaken, a missing form schema or calendar period as
// ErrPhaseReferenceMissing. Race-safe: nothing is pre-checked, the database
// decides and the owner names its verdict.
func markPhaseWriteError(err error) error {
	var postgresError pgdriver.Error
	if !errors.As(err, &postgresError) || !postgresError.IntegrityViolation() {
		return err
	}
	switch constraint := postgresError.Field('n'); {
	case constraint == phaseNameUniqueConstraint:
		return &phaseWriteError{err: err, reason: enrollment.ErrPhaseNameTaken}
	case isPhaseReferenceConstraint(constraint):
		return &phaseWriteError{err: err, reason: enrollment.ErrPhaseReferenceMissing}
	}
	return err
}

func isPhaseReferenceConstraint(name string) bool {
	return name == "phases_form_schema_id_fkey" || name == "phases_calendar_period_id_fkey"
}
