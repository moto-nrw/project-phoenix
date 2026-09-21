package postgres

import (
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

const pendingStudentDataFieldIndex = "uniq_student_data_change_requests_pending_field"

// studentDataRequestCreateError marks a duplicate pending request for the same
// field and keeps the raw database error in the chain.
func studentDataRequestCreateError(err error) error {
	wrapped := requestDBError("create student data change request", err)
	if isConstraintViolation(err, pendingStudentDataFieldIndex) {
		return fmt.Errorf("%w: %w", careplan.ErrStudentDataRequestFieldPending, wrapped)
	}
	return wrapped
}

// isConstraintViolation reports an integrity violation of one named
// constraint or index, read from the driver fields like
// IsPendingCareScheduleConflict.
func isConstraintViolation(err error, constraint string) bool {
	var detail requestConstraintError
	return errors.As(err, &detail) && strings.HasPrefix(detail.Field('C'), "23") && detail.Field('n') == constraint
}
