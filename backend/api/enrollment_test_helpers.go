package api

import (
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// ApprovedOfferingTestProjection exposes the composed read view to API test roots.
type ApprovedOfferingTestProjection = enrollmentCompose.ApprovedOfferingProjection

func NewOwnerApprovedOfferingTestProjection(db transactionruntime.Database) (*ApprovedOfferingTestProjection, error) {
	return NewApprovedOfferingTestProjection(db, services.NewEnrollmentBookingProjection(enrollmentCompose.New()))
}

func NewApprovedOfferingTestProjection(db transactionruntime.Database, selections enrollmentCompose.ApprovedSelections) (*enrollmentCompose.ApprovedOfferingProjection, error) {
	students, err := NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	return enrollmentCompose.NewApprovedOfferingProjection(selections, services.OfferingStudents{Query: students}), nil
}
