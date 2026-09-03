package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/audit"
	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanLegacy "github.com/moto-nrw/project-phoenix/modules/careplan/legacy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/uptrace/bun"
)

func NewCarePlan(db *bun.DB, students peopledirectory.Capability) (careplan.Capability, error) {
	if students == nil {
		return nil, errors.New("compose Care Plan: People Directory capability is required")
	}
	studentLock, studentNotFound := CareStudentLock(students)
	return carePlanCompose.New(carePlanCompose.Dependencies{
		DB: db, Observe: func(carePlanCompose.Observation) {}, AmbientDB: carePlanLegacy.NewAmbientDatabase(db),
		People:      students,
		StudentLock: studentLock, StudentNotFound: studentNotFound,
	})
}

// BindCarePlan replaces the bootstrap adapters with the observed Care Plan
// capability composed by the production root.
func (f *Factory) BindCarePlan(capability careplan.Capability) {
	if capability == nil {
		panic("repository factory: care plan capability is required")
	}
	if f.carePlanBound {
		return
	}
	f.carePlanBound = true
	f.bindCarePlanAdapters(capability)
}

// CarePlan returns the capability used by the legacy repository adapters.
func (f *Factory) CarePlan() careplan.Capability { return f.carePlan }

func (f *Factory) bindCarePlanAdapters(capability careplan.Capability) {
	f.carePlan = capability
	f.CareOffering = carePlanLegacy.NewCareOfferingRepository(capability)
	f.OfferingChangeRequest = carePlanLegacy.NewOfferingChangeRepository(capability, f.students)
	companion := carePlanLegacy.NewCompanionRepository(capability)
	f.StudentCompanion = companion
	f.StudentDocument = carePlanLegacy.NewCareDocumentRepository(capability)
	if repository, ok := f.Student.(*usersRepo.StudentRepository); ok {
		repository.BindCompanionRepository(companion)
	}
	f.bindCarePlanAuditDirectory()
	if repository, ok := f.PhaseExpiry.(*enrollmentRepo.PhaseExpiryRepository); ok {
		repository.BindCarePlan(phaseExpiryCarePlanDirectory{query: capability})
	}
	if repository, ok := f.CareExitCleanup.(*usersRepo.CareExitCleanupRepository); ok {
		repository.BindCarePlan(careExitCarePlanDirectory{capability: capability})
	}
	if repository, ok := f.CareExit.(*usersRepo.CareExitRepository); ok {
		repository.BindCarePlan(capability)
	}
	if repository, ok := f.StudentDeletion.(*usersRepo.StudentDeletionRepository); ok {
		repository.BindCarePlan(capability)
	}
}

func (f *Factory) bindCarePlanAuditDirectory() {
	if f.carePlan == nil {
		return
	}
	if repository, ok := f.BookingConsistency.(interface {
		BindCarePlan(audit.CareOfferingDirectory)
	}); ok {
		repository.BindCarePlan(auditCarePlanDirectory{query: f.carePlan})
	}
}

type auditCarePlanDirectory struct{ query careplan.Query }

func (d auditCarePlanDirectory) ListCareOfferings(ctx context.Context) ([]audit.CareOfferingProjection, error) {
	values, err := d.query.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]audit.CareOfferingProjection, 0, len(values))
	for _, value := range values {
		result = append(result, audit.CareOfferingProjection{
			ID: value.ID, TenantID: value.TenantID, PhaseID: value.PhaseID,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays,
			IsActive: value.IsActive, IsRequired: value.IsRequired,
			CountsAsCare: value.CountsAsCare, PickupTimes: value.PickupTimes,
		})
	}
	return result, nil
}

type phaseExpiryCarePlanDirectory struct{ query careplan.Query }

func (d phaseExpiryCarePlanDirectory) ListCareOfferings(ctx context.Context) ([]enrollmentRepo.CareOfferingProjection, error) {
	values, err := d.query.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentRepo.CareOfferingProjection, 0, len(values))
	for _, value := range values {
		result = append(result, enrollmentRepo.CareOfferingProjection{
			ID: value.ID, TenantID: value.TenantID, PhaseID: value.PhaseID,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays, IsActive: value.IsActive,
		})
	}
	return result, nil
}

type careExitCarePlanDirectory struct{ capability careplan.Capability }

func (d careExitCarePlanDirectory) ListCareOfferings(ctx context.Context) ([]usersRepo.CareOfferingProjection, error) {
	values, err := d.capability.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]usersRepo.CareOfferingProjection, 0, len(values))
	for _, value := range values {
		result = append(result, usersRepo.CareOfferingProjection{
			ID: value.ID, TenantID: value.TenantID, Name: value.Name,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays,
			CountsAsCare: value.CountsAsCare, SortOrder: value.SortOrder,
		})
	}
	return result, nil
}

func (d careExitCarePlanDirectory) LockCareOfferings(ctx context.Context, ids []int64) error {
	_, err := d.capability.ListCareOfferings(ctx, careplan.CareOfferingFilter{
		IDs: ids, LockForUpdate: true, Order: careplan.OfferingOrderID,
	})
	return err
}

func (d careExitCarePlanDirectory) ListPendingOfferingChanges(ctx context.Context, studentIDs []int64, lock bool) ([]usersRepo.PendingOfferingChange, error) {
	values, err := d.capability.ListOfferingChanges(ctx, careplan.OfferingChangeFilter{
		StudentIDs: studentIDs, Statuses: []string{careplan.OfferingChangePending},
		LockForUpdate: lock, Order: careplan.ChangeOrderCreated,
	})
	if err != nil {
		return nil, err
	}
	result := make([]usersRepo.PendingOfferingChange, 0, len(values))
	for _, value := range values {
		result = append(result, usersRepo.PendingOfferingChange{StudentID: value.StudentID})
	}
	return result, nil
}

func (d careExitCarePlanDirectory) ClosePendingOfferingChanges(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (int64, error) {
	return d.capability.ClosePendingOfferingChanges(ctx, studentIDs, reason, reviewedBy, at)
}

func (d careExitCarePlanDirectory) ListCareExitRemovals(ctx context.Context, studentIDs []int64) ([]careplan.CareExitRemoval, error) {
	return d.capability.ListCareExitRemovals(ctx, studentIDs)
}

func (d careExitCarePlanDirectory) ListCareExitSourceRemovals(ctx context.Context, studentIDs []int64) ([]careplan.CareExitSourceRemoval, error) {
	return d.capability.ListCareExitSourceRemovals(ctx, studentIDs)
}

func (d careExitCarePlanDirectory) RecordCareExitRemovals(ctx context.Context, values []careplan.CareExitRemoval) error {
	return d.capability.RecordCareExitRemovals(ctx, values)
}

func (d careExitCarePlanDirectory) RecordCareExitSourceRemovals(ctx context.Context, values []careplan.CareExitSourceRemoval) error {
	return d.capability.RecordCareExitSourceRemovals(ctx, values)
}

func (d careExitCarePlanDirectory) DiscardCareExitRemovals(ctx context.Context, studentIDs []int64) error {
	return d.capability.DiscardCareExitRemovals(ctx, studentIDs)
}
