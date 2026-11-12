package owners

import (
	"context"
	"fmt"

	enrollmentOwner "github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	parentBillingCompose "github.com/moto-ogs/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
	var asked bool
	if err := (CareContractRuntime{}).WithinTraeger(ctx, school.OrganizationID, func(txCtx context.Context) error {
		var err error
		asked, err = parentBillingCompose.NewParentDocuments().CountsFamilyChildrenAt(txCtx, school.OrganizationID, schoolID, calendar.TodayDate())
		return err
	}); err != nil {
		return false, fmt.Errorf("enrollment: read the sibling rules of the Träger: %w", err)
	}
	return asked, nil
}
