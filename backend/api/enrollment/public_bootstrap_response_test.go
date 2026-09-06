package enrollment

import (
	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

func TestBuildPublicEnrollmentFormBootstrapResponse_IncludesLateInvitePrefill(t *testing.T) {
	t.Parallel()

	firstName := "Mara"
	lastName := "Muster"
	response := BuildPublicEnrollmentFormBootstrapResponse(
		&enrollmentService.PublicFormBootstrapData{
			Phase: &capability.Phase{
				CareOfferingSelectionMode: capability.PhaseCareOfferingSelectionOptional,
			},
			Offerings: []*enrollmentModels.CareOffering{},
			LateInvite: &capability.LateInvite{
				GuardianEmail:     "invited@example.test",
				GuardianFirstName: &firstName,
				GuardianLastName:  &lastName,
			},
		},
		PublicCaptchaConfigResponse{},
	)

	require.NotNil(t, response.LateInvite)
	assert.Equal(t, "invited@example.test", response.LateInvite.GuardianEmail)
	assert.Equal(t, firstName, *response.LateInvite.GuardianFirstName)
	assert.Equal(t, lastName, *response.LateInvite.GuardianLastName)
}
