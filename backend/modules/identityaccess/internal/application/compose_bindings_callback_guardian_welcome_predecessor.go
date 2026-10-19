package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	identityBindingPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func NewGuardianWelcomePredecessor(mails identityBindingPorts.PrecedingMails) func(ctx context.Context, outboxID int64) (bool, bool, error) {
	return func(ctx context.Context, outboxID int64) (bool, bool, error) {
		if mails == nil {
			return false, false, fmt.Errorf("guardian welcome predecessor: delivery state is not composed")
		}
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return false, false, fmt.Errorf("guardian welcome predecessor: tenant is required: %w", err)
		}
		state, found, err := mails.PrecedingMailState(ctx, tenantID.Int64(), outboxID)
		if err != nil {
			return false, false, err
		}
		sent, terminal := domain.GuardianWelcomeOrder(domain.PrecedingMailState(state), found)
		return sent, terminal, nil
	}
}
