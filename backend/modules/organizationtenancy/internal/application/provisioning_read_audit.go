package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// RecordOperatorRead must commit before the HTTP adapter releases any data.
// Failed reads are attempts, not evidence that a response reached its recipient.
func (p *Provisioning) RecordOperatorRead(ctx context.Context, access organizationtenancy.OperatorReadAccess) error {
	if access.OperatorID <= 0 {
		return invalidData("operator is required")
	}
	resource := domain.AuditResourceAccount
	switch access.View {
	case organizationtenancy.ReadSchoolPersons, organizationtenancy.ReadOrganizationPersons:
		resource = domain.AuditResourcePerson
	case organizationtenancy.ReadAllAccounts, organizationtenancy.ReadSchoolAccounts,
		organizationtenancy.ReadOrganizationAccounts, organizationtenancy.ReadSchoolAccountCaregiver,
		organizationtenancy.ReadSchoolAccountMFA, organizationtenancy.ReadAccountMFA, organizationtenancy.ReadAccountTenants:
	default:
		return invalidData("unknown operator read view")
	}
	changes := map[string]any{"view": access.View}
	if access.SchoolID > 0 {
		changes["school_id"] = access.SchoolID
	}
	if access.OrganizationID > 0 {
		changes["organization_id"] = access.OrganizationID
	}
	var resourceID *int64
	if access.AccountID > 0 {
		resourceID = &access.AccountID
	}
	return p.inAdmin(ctx, func(adminCtx context.Context) error {
		return p.recordAction(adminCtx, access.OperatorID, domain.AuditActionView, resource, resourceID, access.ClientIP, changes)
	})
}
