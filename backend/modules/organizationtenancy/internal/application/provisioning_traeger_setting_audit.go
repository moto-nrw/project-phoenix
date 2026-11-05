package application

import (
	"context"
	"regexp"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// settingKeyPattern is the shape of a registry key; the audit stores the key,
// so it takes no free text.
var settingKeyPattern = regexp.MustCompile(`^[a-z0-9_.]{1,255}$`)

// RecordTraegerSettingChange records the operator audit entry of a change of
// a Träger setting (#5748) on the administrative unit, after checking that
// the Träger exists. It commits before the change runs: the Träger value and
// its change log live in the Träger transaction, which cannot join an
// administrative one, so the entry is the record of the attempt and a failed
// audit stops the change.
func (p *Provisioning) RecordTraegerSettingChange(ctx context.Context, change organizationtenancy.TraegerSettingChange) error {
	if change.OperatorID <= 0 {
		return invalidData("operator is required")
	}
	if change.OrganizationID <= 0 {
		return invalidData("organization is required")
	}
	if !settingKeyPattern.MatchString(change.SettingKey) {
		return invalidData("setting key is invalid")
	}
	action := domain.AuditActionUpdate
	if change.Reset {
		action = domain.AuditActionDelete
	}
	organizationID := change.OrganizationID
	_, err := adminValue(ctx, p, func(adminCtx context.Context) (struct{}, error) {
		if _, err := p.organizations.FindOrganization(adminCtx, organizationID); err != nil {
			return struct{}{}, mapOrganizationError(err, organizationID)
		}
		return struct{}{}, p.recordAction(adminCtx, change.OperatorID, action, domain.AuditResourceTraegerSetting, &organizationID, change.ClientIP, map[string]any{
			"organization_id": organizationID,
			"setting_key":     change.SettingKey,
		})
	})
	return err
}
