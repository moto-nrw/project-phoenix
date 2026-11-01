package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// capLegacyLineage handles a refresh family issued before the switch link
// existed (#5422). Every family the new code issues names at least itself as
// its lineage origin (switched_from_family_id); an empty origin marks a
// family issued before, or by a backend that predates the column. Such a
// family may descend from a school switch or a Träger Absprung without the
// link, so a logout of another family of its lineage cannot reach it, and
// renewing it on every rotation would keep it alive without bound. If it may
// descend from one, it keeps its current expiry as family_expiry_cap, which
// every later rotation and switch inherits: it ends one refresh lifetime after
// the deploy at the latest, and capped reports that the deadline is new. A
// family that cannot stem from a switch becomes its own origin and renews as
// before. The branch has no work left once the last family issued before
// #5422 has expired.
func (s *AccountAuthentication) capLegacyLineage(ctx context.Context, current *domain.AccountSession) (capped bool, err error) {
	if current.SwitchedFromFamilyID != "" {
		return false, nil
	}
	unlinked, err := s.mayDescendFromSwitch(ctx, *current)
	if err != nil {
		return false, err
	}
	if !unlinked {
		current.SwitchedFromFamilyID = current.FamilyID
		return false, nil
	}
	if current.FamilyExpiryCap == nil || current.Expiry.Before(*current.FamilyExpiryCap) {
		deadline := current.Expiry
		current.FamilyExpiryCap = &deadline
	}
	deadline := *current.FamilyExpiryCap
	current.LineageDeadline = &deadline
	return true, nil
}

// legacyDeadline is the transition deadline a school switch out of source
// hands down: the lineage deadline it already carries, or, for a family
// issued before #5422, its cap, else its expiry. A family the new code issued
// outside such a lineage hands down nothing; its retirement grace stays its
// own.
func legacyDeadline(source domain.AccountSession) *time.Time {
	if source.SwitchedFromFamilyID != "" {
		return source.LineageDeadline
	}
	deadline := source.Expiry
	if source.FamilyExpiryCap != nil && source.FamilyExpiryCap.Before(deadline) {
		deadline = *source.FamilyExpiryCap
	}
	return &deadline
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
