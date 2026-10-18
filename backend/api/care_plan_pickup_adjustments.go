package api

import (
	"context"

	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
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
	Settings         OfferingChangeSettingsReads
	Audit            peoplePorts.StudentPickupPlanRecorder
	Students         PickupAdjustmentStudentRows
	Today            func() calendar.Date
}

func NewPickupAdjustments(inputs PickupAdjustmentInputs) (careplan.PickupAdjustments, error) {
	return carePlanCompose.NewPickupAdjustments(carePlanCompose.PickupAdjustmentDependencies{
		PickupSchedules: inputs.PickupSchedules, ArrivalSchedules: inputs.ArrivalSchedules, Records: inputs.CarePlan,
		Baselines: inputs.Baselines, Offerings: inputs.Offerings,
		Settings: offeringChangeSettings{settingsModule.NewCareBookingValues(inputs.Settings), inputs.Settings},
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

// OfferingChangeSettingsReads is the slice of the settings service offering
// changes and pickup adjustments resolve through.
type OfferingChangeSettingsReads interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// NewOfferingChangeSettings resolves the tenant settings the root's
// offering-change review obeys, the ones the pickup adjustment reads too.
func NewOfferingChangeSettings(reads OfferingChangeSettingsReads) carePlanCompose.OfferingChangeSettings {
	return offeringChangeSettings{settingsModule.NewCareBookingValues(reads), reads}
}

// offeringChangeSettings resolves the tenant settings offering changes and
// pickup adjustments obey.
type offeringChangeSettings struct {
	settingsModule.CareBookingValues
	reads OfferingChangeSettingsReads
}

func (s offeringChangeSettings) OfferingChangesEnabled(ctx context.Context) (bool, error) {
	return s.reads.ResolveBool(ctx, configModels.KeyEnrollmentOfferingChangesEnabled)
}

func (s offeringChangeSettings) CourseRequestsEnabled(ctx context.Context) (bool, error) {
	return s.reads.ResolveBool(ctx, configModels.KeyEnrollmentParentCourseRequestsEnabled)
}

func (s offeringChangeSettings) OfferingChangeLeadDays(ctx context.Context) (string, error) {
	return s.reads.ResolveString(ctx, configModels.KeyEnrollmentOfferingChangesLeadDays)
}

func (s offeringChangeSettings) PickupOfferingReviewRequired(ctx context.Context) (bool, error) {
	return s.reads.ResolveBool(ctx, configModels.KeyRequirePickupOfferingReview)
}
