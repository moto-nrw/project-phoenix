package api

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

func NewCarePlan(db transactionruntime.Database, students peopledirectory.Capability, slots timetableCompose.StatusSlotRows) (careplan.Capability, error) {
	if students == nil || slots == nil {
		return nil, errors.New("compose Care Plan: People Directory and instance-student repository are required")
	}
	profiles, err := carePlanCompose.NewStudentProfiles(db.Pool(), func(carePlanCompose.Observation) {})
	if err != nil {
		return nil, err
	}
	statusStudents, err := services.CarePlanStatusStudents(students, profiles)
	if err != nil {
		return nil, err
	}
	studentLock, studentNotFound := peopleCompose.CareStudentLock(students)
	capability, err := carePlanCompose.New(carePlanCompose.Dependencies{
		DB: db.Pool(), Observe: func(carePlanCompose.Observation) {}, AmbientDB: carePlanCompose.TenantAmbientDatabase(db.Pool()),
		StatusStudents: statusStudents,
		StatusSlots:    services.CarePlanStatusSlots(slots),
		People: carePlanCompose.StudentNameFinderFunc(func(ctx context.Context, ids []int64) ([]carePlanCompose.StudentName, error) {
			values, err := students.ListStudentNamesByID(ctx, ids)
			if err != nil {
				return nil, err
			}
			result := make([]carePlanCompose.StudentName, 0, len(values))
			for _, value := range values {
				result = append(result, carePlanCompose.StudentName{StudentID: value.StudentID, FirstName: value.FirstName, LastName: value.LastName})
			}
			return result, nil
		}),
		StudentLock: studentLock, StudentNotFound: studentNotFound,
	})
	if err != nil {
		return nil, err
	}
	return capability, nil
}

// NewUnobservedCarePlan composes Care Plan over the People Directory of
// dependencies, reading the participant slots through the Timetable adapter
// over an unobserved Student Presence owner, as the test graphs need it.
func NewUnobservedCarePlan(db transactionruntime.Database, dependencies services.TimetableDependencies) (careplan.Capability, error) {
	slots := timetableCompose.NewInstanceStudentRepository(db.Pool(), dependencies.Capability, services.NewUnloggedStudentPresence(db.Pool()), services.NewPartialAbsencePreview(db.Pool()))
	return NewCarePlan(db, dependencies.Students, slots)
}
