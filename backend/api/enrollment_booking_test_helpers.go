//go:build testhooks

package api

import (
	"github.com/moto-nrw/project-phoenix/tenant"
)

func NewEnrollmentBookingFixture() EnrollmentBookingFixture {
	return NewEnrollmentBookingFixtureWithin(tenant.NewTransactionRunner().RunInTx)
}
