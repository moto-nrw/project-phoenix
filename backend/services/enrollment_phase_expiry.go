package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
)

type phaseExpiryCarePlanDirectory struct{ query careplan.Query }

func (d phaseExpiryCarePlanDirectory) ListCareOfferings(ctx context.Context) ([]enrollment.PhaseExpiryOffering, error) {
	values, err := d.query.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]enrollment.PhaseExpiryOffering, 0, len(values))
	for _, value := range values {
		result = append(result, enrollment.PhaseExpiryOffering{
			ID: value.ID, TenantID: value.TenantID, PhaseID: value.PhaseID,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays, IsActive: value.IsActive,
		})
	}
	return result, nil
}

type phaseExpiryStudents struct{ query peopledirectory.StudentQuery }

func (d phaseExpiryStudents) ListEnrolledStudents(ctx context.Context) ([]enrollment.PhaseExpiryStudent, error) {
	values, err := d.query.ListEnrolledStudents(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]enrollment.PhaseExpiryStudent, 0, len(values))
	for _, value := range values {
		result = append(result, enrollment.PhaseExpiryStudent{ID: value.ID, Status: value.Status, EnrolledFrom: value.EnrolledFrom, EnrolledUntil: value.EnrolledUntil})
	}
	return result, nil
}

type offeringStudents struct{ query peopledirectory.StudentQuery }

func (d offeringStudents) ListOfferingStudents(ctx context.Context, ids []int64) ([]enrollment.OfferingStudent, error) {
	values, err := d.query.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	rows := make([]enrollment.OfferingStudent, 0, len(values))
	for _, value := range values {
		rows = append(rows, enrollment.OfferingStudent{ID: value.ID, SchoolClass: value.SchoolClass, Alumnus: value.IsAlumnus()})
	}
	return rows, nil
}
