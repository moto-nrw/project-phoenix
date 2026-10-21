//go:build testhooks

package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

func NewCarePlan(db transactionruntime.Database, students peopledirectory.Capability, slots timetablePorts.ParticipantRows) (careplan.Capability, error) {
	if students == nil || slots.Presence == nil {
		return nil, errors.New("compose Care Plan: People Directory and instance-student repository are required")
	}
	profiles, err := carePlanCompose.NewStudentProfiles(db.Pool(), studentDirectoryView.NewReader(), func(carePlanCompose.Observation) {})
	if err != nil {
		return nil, err
	}
	statusStudents, err := CarePlanStatusStudents(students, profiles)
	if err != nil {
		return nil, err
	}
	studentLock, studentNotFound := peopleCompose.CareStudentLock(students)
	capability, err := carePlanCompose.New(carePlanCompose.Dependencies{
		StudentDirectory: studentDirectoryView.NewReader(),
		DB:               db.Pool(), Observe: func(carePlanCompose.Observation) {}, AmbientDB: carePlanCompose.TenantAmbientDatabase(db.Pool()),
		StatusStudents: statusStudents,
		StatusSlots:    CarePlanStatusSlots(slots),
		People: carePlanStudentNames(func(ctx context.Context, ids []int64) ([]carePlanPorts.StudentName, error) {
			values, err := students.ListStudentNamesByID(ctx, ids)
			if err != nil {
				return nil, err
			}
			result := make([]carePlanPorts.StudentName, 0, len(values))
			for _, value := range values {
				result = append(result, carePlanPorts.StudentName{StudentID: value.StudentID, FirstName: value.FirstName, LastName: value.LastName})
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

// NewUnobservedCarePlan composes Care Plan over the People Directory
// students, reading the participant slots through the Timetable participant rows over
// the Timetable capability and an unobserved Student Presence owner, as the
// test graphs need it.
func NewUnobservedCarePlan(db transactionruntime.Database, capability timetable.Capability, students peopledirectory.Capability) (careplan.Capability, error) {
	slots := timetablePorts.ParticipantRows{Plans: capability, Presence: NewUnloggedStudentPresence(db), Reads: MustPresenceReads(db)}
	return NewCarePlan(db, students, slots)
}
