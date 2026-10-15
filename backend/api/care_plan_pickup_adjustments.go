package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/services"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PickupAdjustmentInputs are the owners Care Plan's pickup adjustment
// (#3561) is composed over.
type PickupAdjustmentInputs struct {
	CarePlan         careplan.Capability
	PickupSchedules  careplan.PickupScheduleService
	ArrivalSchedules careplan.ArrivalScheduleService
	Baselines        careplan.PickupBaselineReader
	Offerings        careplan.DirectOfferingAdjustments
	Settings         services.OfferingChangeSettingsReads
	Audit            peopleCompose.StudentPickupPlanRecorder
	Students         PickupAdjustmentStudentRows
	Today            func() calendar.Date
}

func NewPickupAdjustments(inputs PickupAdjustmentInputs) (careplan.PickupAdjustments, error) {
	return carePlanCompose.NewPickupAdjustments(carePlanCompose.PickupAdjustmentDependencies{
		PickupSchedules: inputs.PickupSchedules, ArrivalSchedules: inputs.ArrivalSchedules, Records: inputs.CarePlan,
		Baselines: inputs.Baselines, Offerings: inputs.Offerings,
		Settings: services.OfferingChangeSettings{BookingSettings: services.BookingSettings{Settings: inputs.Settings}, Reads: inputs.Settings},
		Audit:    inputs.Audit, Students: pickupAdjustmentStudents{students: inputs.Students},
		Fingerprint: securityruntime.Fingerprint, Today: inputs.Today,
	})
}

// pickupAdjustmentStudents locks the People Directory students a pickup
// adjustment writes for.
type pickupAdjustmentStudents struct{ students PickupAdjustmentStudentRows }

// PickupAdjustmentStudentRows are the People Directory child rows a pickup
// adjustment locks.
type PickupAdjustmentStudentRows interface {
	FindByIDForUpdate(ctx context.Context, id int64) (*usersModels.Student, error)
	FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*usersModels.Student, error)
}

func (s pickupAdjustmentStudents) LockStudent(ctx context.Context, id int64) (*careplan.ScheduleStudent, error) {
	student, err := s.students.FindByIDForUpdate(ctx, id)
	if err != nil || student == nil {
		return nil, err
	}
	return &careplan.ScheduleStudent{ID: student.ID, TenantID: student.TenantID}, nil
}

func (s pickupAdjustmentStudents) LockStudents(ctx context.Context, ids []int64) (map[int64]careplan.ScheduleStudent, error) {
	students, err := s.students.FindByIDsForUpdate(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]careplan.ScheduleStudent, len(students))
	for id, student := range students {
		if student != nil {
			result[id] = careplan.ScheduleStudent{ID: student.ID, TenantID: student.TenantID}
		}
	}
	return result, nil
}
