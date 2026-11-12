package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	identityPorts "github.com/moto-ogs/project-phoenix/modules/identityaccess/ports"
)

// activeSchools lists the schools an account is actively mapped to.
type activeSchools interface {
	ListActiveTenantIDs(ctx context.Context, accountID int64) ([]int64, domain.OperationStats, error)
}

// AccountOwnership decides whether one school alone holds an account
// (#6862). A school administrator may rewrite what holds for the whole
// account, its login address and its second factor, only then: an account
// that is also active at another school or belongs to a Träger-Büro signs
// in there with the same identity. It is the rule staff offboarding applies
// before it disables an account.
type AccountOwnership struct {
	schools activeSchools
	traeger identityPorts.TraegerMemberships
}

// NewAccountOwnership composes the rule over the account's school mappings
// and the Träger memberships of Organisation & Tenancy.
func NewAccountOwnership(schools activeSchools, traeger identityPorts.TraegerMemberships) *AccountOwnership {
	if schools == nil {
		return nil
	}
	return &AccountOwnership{schools: schools, traeger: traeger}
}

// HeldBySchoolAlone reports whether tenantID is the account's only active
// school and no Träger membership is retained. An unreadable fact refuses:
// without the Träger memberships the rule cannot tell, and says so.
func (o *AccountOwnership) HeldBySchoolAlone(ctx context.Context, accountID, tenantID int64) (bool, error) {
	if o == nil || o.traeger == nil {
		return false, errTraegerMembershipsUnavailable
	}
	schools, _, err := o.schools.ListActiveTenantIDs(ctx, accountID)
	if err != nil {
		return false, err
	}
	if len(schools) != 1 || schools[0] != tenantID {
		return false, nil
	}
	retained, err := o.traeger.HasRetainedTraegerMembership(ctx, accountID)
	if err != nil {
		return false, err
	}
	return !retained, nil
}
