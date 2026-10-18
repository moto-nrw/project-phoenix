//go:build testhooks

package api

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

func NewCarePlan(db transactionruntime.Database, students peopledirectory.Capability, slots timetablePorts.StatusSlotRows) (careplan.Capability, error) {
	if students == nil || slots == nil {
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
// students, reading the participant slots through the Timetable adapter over
// the Timetable capability and an unobserved Student Presence owner, as the
// test graphs need it.
func NewUnobservedCarePlan(db transactionruntime.Database, capability timetable.Capability, students peopledirectory.Capability) (careplan.Capability, error) {
	slots := timetableCompose.NewInstanceStudentRepository(db.Pool(), capability, NewUnloggedStudentPresence(db), NewPartialAbsencePreview(db))
	return NewCarePlan(db, students, slots)
}

// CreateParentPickupExceptionFixture keeps the native owner's validation,
// error mapping and persisted row fields for parent workflow fixtures.
func CreateParentPickupExceptionFixture(ctx context.Context, owner careplan.Capability, row *careplan.PickupException) error {
	if err := row.Validate(); err != nil {
		return err
	}
	rows := NewCarePlanRows(owner)
	value, err := rows.Records.CreatePickupException(ctx, carePickupExceptionToRows(*row))
	if err != nil {
		return rows.ScheduleError("create", err)
	}
	*row = carePickupExceptionFromRows(value)
	return nil
}

// CreateParentArrivalExceptionFixture keeps the native owner's validation,
// error mapping and persisted row fields for parent workflow fixtures.
func CreateParentArrivalExceptionFixture(ctx context.Context, owner careplan.Capability, row *careplan.ArrivalException) error {
	if err := row.Validate(); err != nil {
		return err
	}
	rows := NewCarePlanRows(owner)
	value, err := rows.Records.CreateArrivalException(ctx, careArrivalExceptionToRows(*row))
	if err != nil {
		return rows.ScheduleError("create", err)
	}
	*row = careArrivalExceptionFromRows(value)
	return nil
}

// CreateParentPickupScheduleFixture keeps the native owner's validation,
// error mapping and persisted row fields for parent workflow fixtures.
func CreateParentPickupScheduleFixture(ctx context.Context, owner careplan.Capability, row *careplan.PickupSchedule) error {
	if err := row.Validate(); err != nil {
		return err
	}
	rows := NewCarePlanRows(owner)
	value, err := rows.Records.CreatePickupSchedule(ctx, carePickupScheduleToRows(*row))
	if err != nil {
		return rows.ScheduleError("create", err)
	}
	*row = carePickupScheduleFromRows(value)
	return nil
}

// FindParentPickupExceptionFixture uses the same filtered owner read and
// retained diagnostic as the former schedule fixture adapter.
func FindParentPickupExceptionFixture(ctx context.Context, owner careplan.Capability, studentID int64, date calendar.Date) (*careplan.PickupException, error) {
	rows := NewCarePlanRows(owner)
	values, err := rows.Records.ListPickupExceptions(ctx, timetablePorts.CareScheduleFilter{StudentIDs: []int64{studentID}, Date: timetablePorts.CareDate(date)})
	if err != nil {
		return nil, rows.ScheduleError("find by student id and date", err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	value := carePickupExceptionFromRows(values[0])
	return &value, nil
}
