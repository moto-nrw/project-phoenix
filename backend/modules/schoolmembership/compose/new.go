// Package compose wires the School Membership module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"

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

// New composes the School Membership module. Every operation runs on the
// caller's ambient tenant transaction when one exists and otherwise opens
// one for the tenant in context, so row-level security decides visibility
// exactly as it did for the legacy repositories.
func New(dependencies Dependencies) (*schoolmembership.Module, error) {
	service, err := newApplication(dependencies)
	if err != nil {
		return nil, err
	}
	moduleEngine := application.NewEngine(service)
	return schoolmembership.NewModule(moduleEngine, moduleEngine), nil
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
		classList = application.NewClassListAdministration(dependencies.ClassListStudents, dependencies.ClassListTrail)
	case dependencies.ClassListStudents != nil || dependencies.ClassListTrail != nil:
		return nil, errors.New("school membership compose: the class list entries need a student directory and an audit trail")
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, postgres.TenantScope, error) {
		scope := postgres.TenantScope{}
		id, err := tenant.TenantFromContext(ctx)
		if err != nil {
			if !tenant.IsAdminTx(ctx) {
				return nil, scope, err
			}
			scope.AcrossSchools = true
		} else {
			scope.TenantID = id.Int64()
		}
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, scope, errors.New("schoolmembership postgres: transaction is required")
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, scope, nil
		case *bun.Tx:
			if tx != nil {
				return *tx, scope, nil
			}
		}
		return nil, scope, fmt.Errorf("schoolmembership postgres: unsupported transaction %T", transaction)
	},
	)
	var quota ports.ChildQuota
	if dependencies.ChildQuota != nil {
		quota = dependencies.ChildQuota
	}
	service := application.New(store, staffEmployment{port: dependencies.Employment}, quota, postgres.BerlinToday, application.TenantTransaction{}, func(observation Observation) {
		observation.Err = application.MapError(observation.Err)
		dependencies.Observe(observation)
	}, classList)
	return service, nil
}
