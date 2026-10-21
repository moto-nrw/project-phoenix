package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

func (d GuardianInvitationDelivery) EnqueueWelcomeEmail(ctx context.Context, profile domain.GuardianProfile, tenantID int64, schoolName string, precedingOutboxID int64) {
	d.GuardianInvitationDelivery.EnqueueWelcomeEmail(ctx, identityPorts.GuardianProfile(profile), tenantID, schoolName, precedingOutboxID)
}
