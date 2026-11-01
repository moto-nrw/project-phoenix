package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// capLegacyLineage keeps the deadline of a refresh family issued before the
// switch link existed (#5422) that may descend from a school switch or a
// Träger Absprung. Its rows carry no switched_from_family_id, so a logout of
// another family of its lineage cannot reach it; renewing it on every
// rotation would keep it alive without bound. Its successor keeps the
// family's current expiry as family_expiry_cap, which every later rotation
// inherits, so such a family ends one refresh lifetime after the deploy at
// the latest. A token the new code issued names its lineage (lineage_id) and
// passes; so does every family that cannot stem from a switch, which keeps
// renewing as before. The branch has no work left once the last token
// issued before #5422 has expired.
func (s *AccountAuthentication) capLegacyLineage(ctx context.Context, claims domain.RefreshClaims, current *domain.AccountSession) error {
	if claims.LineageID != "" || current.SwitchedFromFamilyID != "" {
		return nil
	}
	unlinked, err := s.mayDescendFromSwitch(ctx, *current)
	if err != nil || !unlinked {
		return err
	}
	if current.FamilyExpiryCap == nil || current.Expiry.Before(*current.FamilyExpiryCap) {
		deadline := current.Expiry
		current.FamilyExpiryCap = &deadline
	}
	return nil
}

// mayDescendFromSwitch reports whether a family could have been minted by a
// school switch or an Absprung. A switch needs a mapping at its target and a
// source family at another school or of a Träger, and an Absprung session
// names its Träger: an account with a single school mapping of any status and
// without Träger membership never switched.
func (s *AccountAuthentication) mayDescendFromSwitch(ctx context.Context, session domain.AccountSession) (bool, error) {
	switch session.PortalScope {
	case domain.PortalScopeTenant, domain.PortalScopeSchool, domain.PortalScopeUnknown:
	default:
		return false, nil
	}
	if session.OrganizationID != 0 {
		return true, nil
	}
	mappings, _, err := s.store.CountAccountTenantMappings(ctx, session.AccountID)
	if err != nil {
		return false, fmt.Errorf("count school mappings of a legacy refresh family: %w", err)
	}
	if mappings > 1 {
		return true, nil
	}
	if s.traeger == nil {
		return false, nil
	}
	organizations, err := s.traeger.ListAccountTraegerOrganizations(ctx, session.AccountID)
	if err != nil {
		return false, fmt.Errorf("list Träger memberships of a legacy refresh family: %w", err)
	}
	return len(organizations) > 0, nil
}
