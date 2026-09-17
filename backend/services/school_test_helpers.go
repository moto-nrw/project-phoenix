package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type SchoolTestModule struct {
	ClassDayTestModule
	DeliveryTestModule
	Auth auth.AuthService
	MFA  auth.MFAService
}

func NewSchoolTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (SchoolTestModule, error) {
	classday, err := NewClassDayTestModule(db, unit, clocks...)
	if err != nil {
		return SchoolTestModule{}, err
	}
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return SchoolTestModule{}, err
	}
	delivery, err := NewDeliveryTestModule(db, unit)
	if err != nil {
		return SchoolTestModule{}, err
	}
	return SchoolTestModule{ClassDayTestModule: classday, DeliveryTestModule: delivery, Auth: auth.Auth, MFA: auth.MFA}, nil
}

// SchoolDeletionLookupForTests reads whether a seeded school exists and is
// soft-deleted through the Organisation & Tenancy capability over db, the
// owner the serving root binds. Every read runs under runtime, as the request
// middleware provides it.
func SchoolDeletionLookupForTests(db *bun.DB, runtime tenant.UnitOfWork) (func(context.Context, int64) (found, deleted bool, err error), error) {
	schools, err := repositories.NewOrganizationTenancy(db)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, id int64) (bool, bool, error) {
		school, found, err := findSchool(tenant.WithUnitOfWork(ctx, runtime), schools, id)
		return found, found && school.IsDeleted(), err
	}, nil
}
