//go:build testhooks

package api

import (
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
)

// ApprovedOfferingTestProjection exposes the composed read view to API test roots.
type ApprovedOfferingTestProjection = enrollmentCompose.ApprovedOfferingProjection

func NewOwnerApprovedOfferingTestProjection(db transactionruntime.Database) (*ApprovedOfferingTestProjection, error) {
	return NewApprovedOfferingTestProjection(db, owners.NewEnrollmentBookingProjection(owners.NewEnrollmentBookings(), owners.UngatedEnrollmentPhaseBounds()))
}

func NewApprovedOfferingTestProjection(db transactionruntime.Database, selections enrollmentCompose.ApprovedSelections) (*enrollmentCompose.ApprovedOfferingProjection, error) {
	students, err := owners.NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	return enrollmentCompose.NewApprovedOfferingProjection(selections, owners.OfferingStudents{Query: students}), nil
}
