package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application"
	"github.com/uptrace/bun"
)

// NewStaffEmployment composes the Workforce owner of the staff employment
// profile (#2753). It needs only the database, so School Membership can be
// bound to it before the rest of Workforce, which in turn depends on School
// Membership, is composed. Every operation joins the caller's transaction or
// opens one for the tenant in context. observe receives one observation per
// call; nil records nothing.
func NewStaffEmployment(db *bun.DB, observe func(Observation)) (workforce.StaffEmployments, error) {
	if db == nil {
		return nil, errors.New("workforce staff employment: database is required")
	}
	if observe == nil {
		observe = func(Observation) {}
	}
	return application.NewStaffEmployment(postgres.New(postgres.DatabaseRuntime(db)), observe), nil
}

// MustNewStaffEmployment composes the staff employment owner without
// observations, for compositions and test graphs that treat a composition
// failure as a programming error. Production roots compose it observed.
func MustNewStaffEmployment(db *bun.DB) workforce.StaffEmployments {
	employment, err := NewStaffEmployment(db, nil)
	if err != nil {
		panic(err)
	}
	return employment
}
