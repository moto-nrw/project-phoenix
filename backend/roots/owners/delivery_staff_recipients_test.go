package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// Delivery's staff recipient resolver takes the staff on duty from the
// root's binding, which applies Workforce's rule (#4072).

type fixedWorkPresence struct {
	presence map[int64]string
	err      error
}

func (p fixedWorkPresence) WorkPresenceMap(context.Context) (map[int64]string, error) {
	return p.presence, p.err
}

func TestDeliveryOnDutyStaffAppliesTheWorkforceRule(t *testing.T) {
	t.Parallel()

	onDuty, err := deliveryOnDutyStaff{presence: fixedWorkPresence{presence: map[int64]string{
		7: workforce.WorkSessionStatusPresent,
		8: workforce.WorkSessionStatusHomeOffice,
		9: "checked_out",
	}}}.OnDutyStaffIDs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int64{7, 8}, onDuty)

	failure := errors.New("presence unavailable")
	_, err = deliveryOnDutyStaff{presence: fixedWorkPresence{err: failure}}.OnDutyStaffIDs(context.Background())
	assert.ErrorIs(t, err, failure)
}
