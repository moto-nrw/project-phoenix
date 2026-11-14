package application

import (
	"testing"

	appointmentcap "github.com/moto-ogs/project-phoenix/modules/appointments"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuardianRecipientStatusRequiresBothGuardianAndVisibleStudent(t *testing.T) {
	t.Parallel()
	guardianID, otherGuardianID := int64(41), int64(42)
	recipients := []*appointmentcap.AppointmentRecipient{
		{ID: 10, GuardianProfileID: &otherGuardianID, StudentIDs: []int64{51}, Status: appointmentcap.ResponseStatusDeclined},
		{ID: 11, GuardianProfileID: &guardianID, StudentIDs: []int64{52}, Status: appointmentcap.ResponseStatusDeclined},
		{ID: 12, GuardianProfileID: &guardianID, StudentIDs: []int64{52, 51}, Status: appointmentcap.ResponseStatusAccepted},
		{ID: 13, GuardianProfileID: &guardianID, StudentIDs: []int64{51}, Status: appointmentcap.ResponseStatusPending},
	}
	recipient := guardianRecipientForStudents(recipients, []int64{guardianID}, []int64{51})
	require.NotNil(t, recipient)
	assert.Equal(t, int64(12), recipient.ID, "the first matching recipient keeps its RSVP")
	assert.Equal(t, appointmentcap.ResponseStatusAccepted, recipient.Status)
}
