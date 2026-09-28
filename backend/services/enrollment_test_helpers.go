package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/uptrace/bun"
)

// ApprovedOfferingTestProjection exposes the composed read view to API test roots.
type ApprovedOfferingTestProjection = enrollmentCompose.ApprovedOfferingProjection

type ApprovedSelectionTestReader = enrollmentCompose.ApprovedSelections

func NewOwnerApprovedOfferingTestProjection(db *bun.DB) (*ApprovedOfferingTestProjection, error) {
	return NewApprovedOfferingTestProjection(db, repositories.NewEnrollmentBookingProjection(enrollmentCompose.New()))
}

func NewApprovedOfferingTestProjection(db *bun.DB, selections enrollmentCompose.ApprovedSelections) (*enrollmentCompose.ApprovedOfferingProjection, error) {
	students, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	return enrollmentCompose.NewApprovedOfferingProjection(selections, offeringStudents{query: students}), nil
}
