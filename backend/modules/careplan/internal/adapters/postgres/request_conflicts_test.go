package postgres

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type requestConstraintFixture struct {
	state, constraint string
}

func (requestConstraintFixture) Error() string { return "localized database error" }
func (e requestConstraintFixture) Field(field byte) string {
	switch field {
	case 'C':
		return e.state
	case 'n':
		return e.constraint
	default:
		return ""
	}
}

func TestRequestConflictClassificationPreservesWrappedDriverFields(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"uniq_care_schedule_change_requests_pending", "uniq_care_schedule_change_requests_pending_pickup_date"} {
		err := fmt.Errorf("request write: %w", requestConstraintFixture{"23505", name})
		assert.True(t, IsPendingCareScheduleConflict(err))
	}
	unrelated := fmt.Errorf("request write: %w", requestConstraintFixture{"23505", "unrelated_unique_index"})
	assert.False(t, IsPendingCareScheduleConflict(unrelated))
	wrongState := requestConstraintFixture{"40001", "uniq_care_schedule_change_requests_pending"}
	assert.False(t, IsPendingCareScheduleConflict(wrongState))
	assert.False(t, IsPickupExceptionConflict(wrongState), "only the driver error establishes a pickup exception conflict")
	assert.False(t, IsPendingCareScheduleConflict(nil))
	assert.False(t, IsPickupExceptionConflict(nil))
	textOnly := errors.New("duplicate key value violates unique constraint \"uniq_care_schedule_change_requests_pending\" (SQLSTATE=23505)")
	assert.False(t, IsPendingCareScheduleConflict(textOnly), "message text cannot establish a native database conflict")
}
