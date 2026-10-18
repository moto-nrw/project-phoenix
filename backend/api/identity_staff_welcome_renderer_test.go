package api

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffWelcomeRendererWaitsForInvitationDelivery(t *testing.T) {
	t.Parallel()
	delivered := false
	payload := map[string]any{
		identityaccess.StaffWelcomePayloadRecipientEmail: "ada@example.test",
		identityaccess.StaffWelcomePayloadHelpURL:        "https://moto.example.test/help/einladung-annehmen-und-konto-einrichten?role=caregiver",
		identityaccess.StaffWelcomePayloadFirstName:      "Ada",
		identityaccess.StaffWelcomePayloadSchoolName:     "OGS Am Berg",
	}

	_, err := renderStaffWelcome(context.Background(), 7, 3, payload, StaffWelcomeRendererConfig{
		InvitationDeliverySent: func(context.Context, int64) (bool, error) { return false, nil },
	}, nil)
	require.Error(t, err, "the welcome must not overtake its invitation")

	delivered = true
	message, err := renderStaffWelcome(context.Background(), 7, 3, payload, StaffWelcomeRendererConfig{
		InvitationDeliverySent: func(context.Context, int64) (bool, error) { return delivered, nil },
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "staff-welcome.html", message.Template)
	assert.Equal(t, "Willkommen bei moto – OGS Am Berg", message.Subject)
	assert.Equal(t, "ada@example.test", message.To.Address)
}
