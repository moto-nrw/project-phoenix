package services

import (
	"context"
	"fmt"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/uptrace/bun"
)

// BookingConsistencyAudit checks one tenant day's approved booking windows.
type BookingConsistencyAudit func(context.Context, auditModels.Date) (*auditModels.BookingConsistencyReport, error)

// NewBookingConsistencyAudit composes the booking drift audit the Worker runs
// (ADR 0049). The Audit composition builds it over the caller's tenant
// transaction; People Directory names the alumni the audit excludes, Care Plan
// supplies the offerings and Enrollment the approved bookings, each through a
// port the Audit composition declares.
func NewBookingConsistencyAudit(db *bun.DB, students peopledirectory.StudentQuery, carePlan careplan.Query) (BookingConsistencyAudit, error) {
	if db == nil || students == nil || carePlan == nil {
		return nil, fmt.Errorf("booking consistency audit: database, People Directory and Care Plan are required")
	}
	audits, err := auditCompose.NewPersistence(auditCompose.PersistenceDependencies{
		Reads:      tenantAuditReads(db),
		Enrollment: NewEnrollmentBookingProjection(enrollmentCompose.New()),
		Students:   auditStudentDirectory{students: students},
		CarePlan:   auditCarePlanDirectory{query: carePlan},
	})
	if err != nil {
		return nil, err
	}
	return audits.BookingConsistency.Audit, nil
}

// tenantAuditReads reads through the caller's tenant transaction and falls back
// to the root database outside one or for any other transaction value, as the
// retained repository Factory's Audit runtime did.
func tenantAuditReads(db *bun.DB) auditCompose.Runtime {
	return func(ctx context.Context) (bun.IDB, int64) {
		tenantID := auditModels.TenantIDFromContext(ctx)
		if raw, ok := auditModels.TransactionFromContext(ctx); ok {
			switch tx := raw.(type) {
			case bun.Tx:
				return tx, tenantID
			case *bun.Tx:
				if tx != nil {
					return tx, tenantID
				}
			}
		}
		return db, tenantID
	}
}

// auditStudentDirectory serves the Audit composition's alumni read from the
// People Directory (#2662).
type auditStudentDirectory struct{ students peopledirectory.StudentQuery }

func (d auditStudentDirectory) ListStudentsByID(ctx context.Context, ids []int64) ([]auditCompose.DirectoryStudent, error) {
	students, err := d.students.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]auditCompose.DirectoryStudent, 0, len(students))
	for _, student := range students {
		result = append(result, auditCompose.DirectoryStudent{ID: student.ID, Alumnus: student.IsAlumnus()})
	}
	return result, nil
}

// auditCarePlanDirectory serves the Audit composition's offering read from
// Care Plan.
type auditCarePlanDirectory struct{ query careplan.Query }

func (d auditCarePlanDirectory) ListCareOfferings(ctx context.Context) ([]auditCompose.CareOffering, error) {
	values, err := d.query.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]auditCompose.CareOffering, 0, len(values))
	for _, value := range values {
		result = append(result, auditCompose.CareOffering{
			ID: value.ID, TenantID: value.TenantID, PhaseID: value.PhaseID,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays,
			IsActive: value.IsActive, IsRequired: value.IsRequired,
			CountsAsCare: value.CountsAsCare, PickupTimes: value.PickupTimes,
		})
	}
	return result, nil
}
