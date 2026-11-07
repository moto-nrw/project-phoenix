//go:build testhooks

package api

import (
	"context"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/api/common"
	notificationsHTTP "github.com/moto-nrw/project-phoenix/modules/delivery/http/notifications"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/schoolportal"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SchoolTestModule is the Identity & Access side of the school portal.
type SchoolTestModule struct {
	Auth identityaccess.Owner
	MFA  identityaccess.AccountMFA
	// SchoolAuth and SchoolMFA are the portal runtimes the composition root
	// binds, so a router test drives the same scope binding production has.
	SchoolAuth schoolportal.AuthRuntime
	SchoolMFA  schoolportal.MFARuntime
}

// NewSchoolTestModule composes the school portal's Identity & Access side
// over db and returns it with the class day and the Delivery owner the
// portal mounts beside it.
func NewSchoolTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (SchoolTestModule, ClassDayTestModule, DeliveryTestModule, error) {
	classday, err := NewClassDayTestModule(db, unit, clocks...)
	if err != nil {
		return SchoolTestModule{}, ClassDayTestModule{}, DeliveryTestModule{}, err
	}
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return SchoolTestModule{}, ClassDayTestModule{}, DeliveryTestModule{}, err
	}
	delivery, err := NewDeliveryTestModule(db, unit)
	if err != nil {
		return SchoolTestModule{}, ClassDayTestModule{}, DeliveryTestModule{}, err
	}
	return SchoolTestModule{
		Auth: auth.Auth, MFA: auth.MFA,
		SchoolAuth: SchoolPortalAuth(auth.Auth),
		SchoolMFA:  SchoolPortalMFA(auth.MFA),
	}, classday, delivery, nil
}

// NewDeviceSchools answers the device school guard through the Organisation
// & Tenancy capability over db, the owner the serving root binds, so handler
// tests exercise the inactive- and deleted-school check against real
// fixtures. Every read runs under unit, as the request middleware provides it.
func NewDeviceSchools(db transactionruntime.Database, unit tenant.UnitOfWork) (deviceauth.SchoolDirectory, error) {
	schools, err := owners.NewOrganizationTenancy(db)
	if err != nil {
		return nil, err
	}
	return deviceSchools{lookup: func(ctx context.Context, id int64) (*deviceauth.School, error) {
		school, found, err := owners.LookupSchool(tenant.WithUnitOfWork(ctx, unit), schools, id)
		if err != nil || !found {
			return nil, err
		}
		return &deviceauth.School{Active: school.Active, Deleted: school.IsDeleted()}, nil
	}}, nil
}

type deviceSchools struct {
	lookup func(context.Context, int64) (*deviceauth.School, error)
}

func (d deviceSchools) FindSchool(ctx context.Context, id int64) (*deviceauth.School, error) {
	return d.lookup(ctx, id)
}

// SchoolMFAResendResponse is the native response the school router renders.
type SchoolMFAResendResponse = common.MFAResendResponse

// newSchoolNotificationsForTests binds the native resource from the existing
// single-owner Delivery fixture, as the production school mount does.
func newSchoolNotificationsForTests(module DeliveryTestModule) *notificationsHTTP.Resource {
	return NewNotificationHTTP(module.Notifications, module.PushSubscriptions, module.NotificationPreferences)
}
