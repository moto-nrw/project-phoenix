package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type SchoolTestModule struct {
	ClassDayTestModule
	DeliveryTestModule
	Auth *identityaccess.Module
	MFA  identityaccess.AccountMFA
	// SchoolAuth and SchoolMFA are the portal runtimes the composition root
	// binds, so a router test drives the same scope binding production has.
	SchoolAuth SchoolPortalAuthRuntime
	SchoolMFA  SchoolPortalMFARuntime
	// TimetableSupervisionSheets serves the per-child sheet of the
	// supervision surface from the enrollment report, as the root binds it.
	TimetableSupervisionSheets TimetableSupervisionSheets
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
	return SchoolTestModule{
		ClassDayTestModule: classday, DeliveryTestModule: delivery,
		Auth: auth.Auth, MFA: auth.MFA,
		SchoolAuth:                 SchoolPortalAuthenticationOver(auth.Auth),
		SchoolMFA:                  SchoolPortalMFAOver(auth.MFA),
		TimetableSupervisionSheets: NewTimetableSupervisionSheets(classday.ClassDay),
	}, nil
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
