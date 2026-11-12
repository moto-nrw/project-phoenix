package application

import (
	"context"
	"regexp"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy/internal/domain"
)

// belegTypePattern is the shape of a Beleg type; the audit stores it, so it
// takes no free text.
var belegTypePattern = regexp.MustCompile(`^[a-z_]{1,32}$`)

// belegRetentionActions maps a kind of change onto the audit action.
var belegRetentionActions = map[string]string{
	organizationtenancy.BelegRetentionInstruction:    domain.AuditActionCreate,
	organizationtenancy.BelegRetentionSuspension:     domain.AuditActionCreate,
	organizationtenancy.BelegRetentionSuspensionLift: domain.AuditActionDelete,
	organizationtenancy.BelegClarificationReview:     domain.AuditActionCreate,
}

// RecordBelegRetentionChange records the operator audit entry of a change of
// the Beleg retention of a Träger (#4720, ADR 0184) on the administrative
// unit, after checking that the Träger exists. It joins the change's
// administrative transaction, so the entry commits with the change or not
// at all.
func (p *Provisioning) RecordBelegRetentionChange(ctx context.Context, change organizationtenancy.BelegRetentionChange) error {
	action, known := belegRetentionActions[change.Kind]
	switch {
	case change.OperatorID <= 0:
		return invalidData("operator is required")
	case change.OrganizationID <= 0:
		return invalidData("organization is required")
	case !known:
		return invalidData("change kind is invalid")
	case !belegTypePattern.MatchString(change.BelegType):
		return invalidData("beleg type is invalid")
	}
	organizationID := change.OrganizationID
	_, err := adminValue(ctx, p, func(adminCtx context.Context) (struct{}, error) {
		if _, err := p.organizations.FindOrganization(adminCtx, organizationID); err != nil {
			return struct{}{}, mapOrganizationError(err, organizationID)
		}
		return struct{}{}, p.recordAction(adminCtx, change.OperatorID, action, domain.AuditResourceBelegRetention, &organizationID, change.ClientIP, map[string]any{
			"organization_id": organizationID,
			"kind":            change.Kind,
			"beleg_type":      change.BelegType,
		})
	})
	return err
}
