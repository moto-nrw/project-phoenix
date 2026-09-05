package services

import (
	"time"

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
