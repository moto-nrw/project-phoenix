package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy/internal/domain"
)

// traegerStaffReleaseActions maps a release change onto the audit action.
var traegerStaffReleaseActions = map[string]string{
	organizationtenancy.TraegerStaffReleaseGranted:   domain.AuditActionGrant,
	organizationtenancy.TraegerStaffReleaseWithdrawn: domain.AuditActionRevoke,
}

// traegerStaffRepresentations are the two statements a Träger confirms
// before the release (ADR 0089 section 2).
var traegerStaffRepresentations = map[string]bool{"agreement": true, "none": true}

// RecordTraegerStaffReleaseChange records the operator audit entry of a
// grant or withdrawal of the release „Personal beim Träger“ (#4705, ADR 0089
// section 2) on the administrative unit, after checking that the Träger
// exists. It joins the change's administrative transaction, so the entry
// commits with the change or not at all. A grant records the statement the
// Träger confirmed.
func (p *Provisioning) RecordTraegerStaffReleaseChange(ctx context.Context, change organizationtenancy.TraegerStaffReleaseChange) error {
	action, known := traegerStaffReleaseActions[change.Kind]
	switch {
	case change.OperatorID <= 0:
		return invalidData("operator is required")
	case change.OrganizationID <= 0:
		return invalidData("organization is required")
	case !known:
		return invalidData("change kind is invalid")
	case change.Kind == organizationtenancy.TraegerStaffReleaseGranted && !traegerStaffRepresentations[change.Representation]:
		return invalidData("representation is invalid")
	}
	organizationID := change.OrganizationID
	_, err := adminValue(ctx, p, func(adminCtx context.Context) (struct{}, error) {
		if _, err := p.organizations.FindOrganization(adminCtx, organizationID); err != nil {
			return struct{}{}, mapOrganizationError(err, organizationID)
		}
		return struct{}{}, p.recordAction(adminCtx, change.OperatorID, action, domain.AuditResourceTraegerStaffRelease, &organizationID, change.ClientIP, map[string]any{
			"organization_id": organizationID,
			"kind":            change.Kind,
			"representation":  change.Representation,
		})
	})
	return err
}
