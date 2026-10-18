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
	return newModule(dependencies, nil)
}

// NewClassListEntries composes the module with its audited class-list
// administration (#2382) over the People Directory students and the Audit
// trail; the root builds it after both owners.
func NewClassListEntries(dependencies Dependencies, students schoolmembership.ClassListEntryStudents, trail schoolmembership.ClassListEntryTrail) (*schoolmembership.Module, error) {
	if students == nil || trail == nil {
		return nil, errors.New("school membership compose: the class list entries need a student directory and an audit trail")
	}
	return newModule(dependencies, application.NewClassListAdministration(students, trail))
}

func newModule(dependencies Dependencies, classList *application.ClassListAdministration) (*schoolmembership.Module, error) {
	service, err := newApplication(dependencies, classList)
	if err != nil {
		return nil, err
	}
	moduleEngine := application.NewEngine(service)
	return schoolmembership.NewModule(moduleEngine, moduleEngine), nil
}

func newApplication(dependencies Dependencies, classList *application.ClassListAdministration) (*application.Service, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("school membership compose: all dependencies are required")
	}
	if dependencies.Employment == nil {
		return nil, errStaffEmploymentUnbound
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("school membership postgres: transaction is required")
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenant.FromContext(ctx), nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenant.FromContext(ctx), nil
			}
		}
		return nil, 0, fmt.Errorf("school membership postgres: unsupported transaction %T", transaction)
	})
	var quota ports.ChildQuota
	if dependencies.ChildQuota != nil {
		quota = dependencies.ChildQuota
	}
	service := application.New(store, staffEmployment{port: dependencies.Employment}, quota, postgres.BerlinToday, transaction{}, func(observation Observation) {
		observation.Err = application.MapError(observation.Err)
		dependencies.Observe(observation)
	}, classList)
	return service, nil
}

type transaction struct{}

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return tenant.WithinCurrentTenant(ctx, callback)
}

func (transaction) RunSavepoint(ctx context.Context, callback func(context.Context) error) error {
	return tenant.WithSavepoint(ctx, callback)
}

func (transaction) RunRead(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if _, err := tenant.TenantFromContext(ctx); err == nil {
		return tenant.WithinCurrentTenant(ctx, callback)
	}
	return tenant.WithinAdmin(ctx, callback)
}
