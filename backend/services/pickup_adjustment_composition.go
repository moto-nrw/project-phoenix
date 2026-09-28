package services

import (
	"context"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// pickupAdjustmentInputs are the owners Care Plan's pickup adjustment
// (#3561) is composed over.
type pickupAdjustmentInputs struct {
	CarePlan         careplan.Capability
	PickupSchedules  careplan.PickupScheduleService
	ArrivalSchedules careplan.ArrivalScheduleService
	Baselines        careplan.PickupBaselineReader
	Offerings        careplan.DirectOfferingAdjustments
	Settings         offeringChangeSettingsReads
	Audit            users.StudentPickupPlanRecorder
	Students         usersModels.StudentRepository
	Today            func() calendar.Date
}

func newPickupAdjustments(inputs pickupAdjustmentInputs) (careplan.PickupAdjustments, error) {
	return carePlanCompose.NewPickupAdjustments(carePlanCompose.PickupAdjustmentDependencies{
		PickupSchedules: inputs.PickupSchedules, ArrivalSchedules: inputs.ArrivalSchedules, Records: inputs.CarePlan,
		Baselines: inputs.Baselines, Offerings: inputs.Offerings,
		Settings: offeringChangeSettings{bookingSettings{settings: inputs.Settings}, inputs.Settings},
		Audit:    inputs.Audit, Students: pickupAdjustmentStudents{students: inputs.Students},
		Fingerprint: securityruntime.Fingerprint, Today: inputs.Today,
	})
}

// pickupAdjustmentStudents locks the People Directory students a pickup
// adjustment writes for.
type pickupAdjustmentStudents struct{ students usersModels.StudentRepository }

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
