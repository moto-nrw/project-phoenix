package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// CreateInvitation stores the invitation and its mail in the same transaction.
// The worker sends only committed entries: staff import may create invitations
// mid-transaction, and a rolled-back link must never reach an inbox.
func (s *SchoolInvitation) CreateInvitation(ctx context.Context, request domain.SchoolInvitationRequest) (result domain.SchoolInvitation, err error) {
	tenantID := request.TenantID
	if tenantID == 0 {
		tenantID = s.runtime.TenantID(ctx)
	}
	if tenantID <= 0 {
		return result, failed(opCreateInvitation, domain.ErrTenantRequired)
	}
	err = s.runtime.RunInTx(s.runtime.WithTenantID(ctx, tenantID), func(txCtx context.Context) error {
		result, err = s.createInvitation(txCtx, request)
		return err
	})
	return result, err
}
