package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/uptrace/bun"
)

// ApprovedOfferingTestProjection exposes the composed read view to API test roots.
type ApprovedOfferingTestProjection = enrollment.ApprovedOfferingProjection

type ApprovedSelectionTestReader = enrollment.ApprovedSelectionReader

func NewOwnerApprovedOfferingTestProjection(db *bun.DB) (*ApprovedOfferingTestProjection, error) {
	return NewApprovedOfferingTestProjection(db, enrollmentCompose.New())
}

func NewApprovedOfferingTestProjection(db *bun.DB, selections enrollment.ApprovedSelectionReader) (*enrollment.ApprovedOfferingProjection, error) {
	students, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	return enrollment.NewApprovedOfferingProjection(selections, offeringStudents{query: students}), nil
}
