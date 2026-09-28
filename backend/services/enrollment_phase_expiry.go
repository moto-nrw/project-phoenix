package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type phaseExpiryCarePlanDirectory struct{ query careplan.Query }

func (d phaseExpiryCarePlanDirectory) ListCareOfferings(ctx context.Context) ([]enrollmentOwner.PhaseExpiryOffering, error) {
	values, err := d.query.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentOwner.PhaseExpiryOffering, 0, len(values))
	for _, value := range values {
		result = append(result, enrollmentOwner.PhaseExpiryOffering{
			ID: value.ID, TenantID: value.TenantID, PhaseID: value.PhaseID,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays, IsActive: value.IsActive,
		})
	}
	return result, nil
}

type phaseExpiryStudents struct{ query peopledirectory.StudentQuery }

func (d phaseExpiryStudents) ListEnrolledStudents(ctx context.Context) ([]enrollmentOwner.PhaseExpiryStudent, error) {
	values, err := d.query.ListEnrolledStudents(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentOwner.PhaseExpiryStudent, 0, len(values))
	for _, value := range values {
		result = append(result, enrollmentOwner.PhaseExpiryStudent{ID: value.ID, Status: value.Status, EnrolledFrom: value.EnrolledFrom, EnrolledUntil: value.EnrolledUntil})
	}
	return result, nil
}

type offeringStudents struct{ query peopledirectory.StudentQuery }

func (d offeringStudents) ListOfferingStudents(ctx context.Context, ids []int64) ([]enrollmentCompose.OfferingStudent, error) {
	values, err := d.query.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	rows := make([]enrollmentCompose.OfferingStudent, 0, len(values))
	for _, value := range values {
		rows = append(rows, enrollmentCompose.OfferingStudent{ID: value.ID, SchoolClass: value.SchoolClass, Alumnus: value.IsAlumnus()})
	}
	return rows, nil
}
