package application

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// capLegacyLineage ends a refresh family issued before the switch link
// existed (#5422). Every family the new code issues names at least itself as
// its lineage origin (switched_from_family_id); an empty origin marks a
// family issued before, or by a backend that predates the column. No rule can
// tell which of these came from a school switch or a Träger Absprung (a
// switch also accepts the current school), and a logout of another family of
// such a lineage cannot reach one, so renewing it would keep it alive without
// bound. Every such family therefore keeps its current expiry as
// family_expiry_cap and lineage_deadline at its first rotation; every later
// rotation and every switch out of it inherit the deadline, so it ends one
// refresh lifetime after the deploy at the latest and its user signs in once
// more. capped reports that current is such a family. The branch has no
// work left once the last family issued before #5422 has expired.
func capLegacyLineage(current *domain.AccountSession) (capped bool) {
	if current.SwitchedFromFamilyID != "" {
		return false
	}
	if current.FamilyExpiryCap == nil || current.Expiry.Before(*current.FamilyExpiryCap) {
		deadline := current.Expiry
		current.FamilyExpiryCap = &deadline
	}
	deadline := *current.FamilyExpiryCap
	current.LineageDeadline = &deadline
	return true
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
