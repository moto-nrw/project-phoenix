package postgres

import (
	"errors"
	"strings"
)

// Native persistence preserves the driver error through wrapping. Inspect its
// SQLSTATE class and constraint fields rather than relying on message text.
type requestConstraintError interface {
	error
	Field(byte) string
}

func IsPickupExceptionConflict(err error) bool {
	return isUniqueViolation(err)
}

// IsPendingCareScheduleConflict distinguishes the two pending-request unique
// indexes from unrelated storage failures, including wrapped driver errors.
func IsPendingCareScheduleConflict(err error) bool {
	var detail requestConstraintError
	if !errors.As(err, &detail) || !strings.HasPrefix(detail.Field('C'), "23") {
		return false
	}
	switch detail.Field('n') {
	case "uniq_care_schedule_change_requests_pending", "uniq_care_schedule_change_requests_pending_pickup_date":
		return true
	default:
		return false
	}
}
