// Package compose wires the School Membership module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
	// Employment is the Workforce half of every staff member (#2753).
	Employment StaffEmployment
	// ChildQuota reads the school's Kinderkontingent (#3567). A graph that
	// never enrolls children may leave it unbound; its counting writes then
	// fail instead of skipping the check.
	ChildQuota ChildQuota
	// ClassListStudents and ClassListTrail serve the audited class-list
	// administration (#2382): the People Directory lookup and the Audit
	// trail. Optional together; without them the audited flows refuse.
	ClassListStudents schoolmembership.ClassListEntryStudents
	ClassListTrail    schoolmembership.ClassListEntryTrail
}

// ChildQuota is the consumer-owned port over Organisation & Tenancy's
// Kinderkontingent. The composition root binds it to
// organizationtenancy.ChildQuotaLimits; School Membership never reads
// platform.schools itself. It answers for the tenant in context on the
// caller's transaction; limited is false when the school has none.
type ChildQuota interface {
	ChildQuotaLimit(ctx context.Context) (limit int, limited bool, err error)
}

// New composes the School Membership service, which is the whole Owner
// surface. Every operation runs on the caller's ambient tenant transaction
// when one exists and otherwise opens one for the tenant in context, so
// row-level security decides visibility exactly as it did for the legacy
// repositories.
func New(dependencies Dependencies) (schoolmembership.Owner, error) {
	service, err := newApplication(dependencies)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func newApplication(dependencies Dependencies) (*application.Service, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("school membership compose: all dependencies are required")
	}
	if dependencies.Employment == nil {
		return nil, errStaffEmploymentUnbound
	}
	var classList *application.ClassListAdministration
	switch {
	case dependencies.ClassListStudents != nil && dependencies.ClassListTrail != nil:
		classList = &application.ClassListAdministration{Students: dependencies.ClassListStudents, Trail: dependencies.ClassListTrail}
	case dependencies.ClassListStudents != nil || dependencies.ClassListTrail != nil:
		return nil, errors.New("school membership compose: the class list entries need a student directory and an audit trail")
	}
	store := postgres.New(postgres.NewContextDatabase(tenant.TenantFromContext, tenant.TransactionFromContext, tenant.IsAdminTx))
	var quota ports.ChildQuota
	if dependencies.ChildQuota != nil {
		quota = dependencies.ChildQuota
	}
	service := application.New(store, application.NewStaffEmployment(dependencies.Employment), quota, postgres.BerlinToday, application.TenantTransaction{}, dependencies.Observe, classList)
	return service, nil
}
