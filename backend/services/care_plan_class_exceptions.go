package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type classArrivalStudentDirectory interface {
	ListStudentRecordsByClass(context.Context, []string, peopledirectory.StudentScope) ([]peopledirectory.StudentRecord, error)
}

func NewClassArrivalExceptions(records timetable.ClassArrivalExceptions, students classArrivalStudentDirectory) (careplan.ClassArrivalExceptions, error) {
	if records == nil || students == nil {
		return nil, errors.New("class arrival exceptions: timetable and students are required")
	}
	return careplanCompose.NewClassArrivalExceptions(classArrivalExceptionRecords{records}, classArrivalStudents{students})
}

type classArrivalExceptionRecords struct {
	records timetable.ClassArrivalExceptions
}

func (s classArrivalExceptionRecords) List(ctx context.Context, classes []string, from, to timezone.Date) ([]*careplan.ClassArrivalException, error) {
	rows, err := s.records.ListClassArrivalExceptions(ctx, classes, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	result := make([]*careplan.ClassArrivalException, len(rows))
	for i, row := range rows {
		result[i] = carePlanClassException(row)
	}
	return result, nil
}

func (s classArrivalExceptionRecords) Upsert(ctx context.Context, row *careplan.ClassArrivalException) error {
	stored, err := s.records.UpsertClassArrivalException(ctx, timetable.ClassArrivalExceptionInput{SchoolClass: row.SchoolClass, Date: row.Date.String(),
		ArrivalTime: row.ArrivalTime, Reason: row.Reason, CreatedBy: row.CreatedBy, Origin: row.Origin})
	if err == nil {
		*row = *carePlanClassException(stored)
	}
	return err
}

func (s classArrivalExceptionRecords) Delete(ctx context.Context, class string, date timezone.Date) (bool, error) {
	return s.records.DeleteClassArrivalException(ctx, class, date.String())
}

func carePlanClassException(row timetable.ClassArrivalException) *careplan.ClassArrivalException {
	return &careplan.ClassArrivalException{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		SchoolClass: row.SchoolClass, Date: timezone.Date(row.Date), ArrivalTime: row.ArrivalTime, Reason: row.Reason, CreatedBy: row.CreatedBy, Origin: row.Origin}
}

type classArrivalStudents struct{ students classArrivalStudentDirectory }

func (s classArrivalStudents) HasActiveClass(ctx context.Context, class string, today timezone.Date) (bool, error) {
	rows, err := s.students.ListStudentRecordsByClass(ctx, []string{class}, peopledirectory.StudentScopeEnrolled)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.EnrolledUntil == "" || !today.After(timezone.Date(row.EnrolledUntil)) {
			return true, nil
		}
	}
	return false, nil
}
