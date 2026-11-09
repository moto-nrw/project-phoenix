package owners

import (
	"context"
	"fmt"
	"slices"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// enrollmentFamilyChildrenPurpose binds Enrollment's question whether a
// registration asks for the children of the family (#6335) to the rules of
// the Elternabrechnung: only where a current sibling rule of the school's
// Träger counts the family and applies to the school (data minimisation).
// It reads the school at Organisation & Tenancy and the Träger's rules in
// that Träger's transaction.
type enrollmentFamilyChildrenPurpose struct {
	schools organizationtenancy.SchoolQuery
}

// NewEnrollmentFamilyChildrenPurpose binds the question to the schools.
func NewEnrollmentFamilyChildrenPurpose(schools organizationtenancy.SchoolQuery) enrollmentOwner.FamilyChildrenPurpose {
	return enrollmentFamilyChildrenPurpose{schools: schools}
}

func (p enrollmentFamilyChildrenPurpose) FamilyChildrenAsked(ctx context.Context, schoolID int64) (bool, error) {
	school, found, err := LookupSchool(ctx, p.schools, schoolID)
	if err != nil || !found || school.OrganizationID <= 0 {
		return false, err
	}
	var rules []parentbilling.ContributionRule
	if err := (CareContractRuntime{}).WithinTraeger(ctx, school.OrganizationID, func(txCtx context.Context) error {
		var err error
		rules, err = parentBillingCompose.NewContributionRules().ListContributionRules(txCtx, school.OrganizationID)
		return err
	}); err != nil {
		return false, fmt.Errorf("enrollment: read the sibling rules of the Träger: %w", err)
	}
	return countsFamilyAt(rules, schoolID, calendar.TodayDate()), nil
}

// countsFamilyAt tells whether a current care sibling rule counts the
// family's children for a contract at the school.
func countsFamilyAt(rules []parentbilling.ContributionRule, schoolID int64, today calendar.Date) bool {
	return slices.ContainsFunc(rules, func(rule parentbilling.ContributionRule) bool {
		return rule.Service == parentbilling.ServiceCare && rule.Kind == parentbilling.RuleSibling && rule.CountsFamilyChildren &&
			(rule.ValidUntil == nil || !rule.ValidUntil.Before(today)) &&
			slices.Contains(rule.SchoolIDs, schoolID)
	})
}
