package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type EnrollmentBookingFixture = repositories.EnrollmentBookingFixture

func NewEnrollmentBookingFixture() EnrollmentBookingFixture {
	return repositories.NewEnrollmentBookingFixture(tenant.NewTransactionRunner().RunInTx)
}
