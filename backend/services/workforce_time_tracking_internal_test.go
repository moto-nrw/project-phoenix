package services

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	"github.com/stretchr/testify/assert"
)

func TestMapTimeTrackingFailure_AllowanceBookingOverlap(t *testing.T) {
	t.Parallel()

	err := mapTimeTrackingFailure(timetracking.ErrAllowanceBookingOverlap)

	assert.ErrorIs(t, err, workforce.ErrAllowanceBookingOverlap)
	assert.Equal(t, timetracking.ErrAllowanceBookingOverlap.Error(), err.Error())
	assert.Equal(t, "conflict", workforce.ErrorCode(err))
}
