package owners

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// BookingConsistencyAudit checks one tenant day's approved booking windows.
type BookingConsistencyAudit func(context.Context, auditModels.Date) (*auditModels.BookingConsistencyReport, error)

// NewBookingConsistencyAudit composes the booking drift audit the Worker runs
// (ADR 0049) next to the job that runs it (#4012). The Audit composition
// builds it over the caller's tenant transaction, with the root database as
// the fallback outside one; People Directory names the alumni the audit
// excludes, Care Plan supplies the offerings and Enrollment the approved
// bookings, each through a port the Audit composition declares.
func NewBookingConsistencyAudit(db transactionruntime.Database, students peopledirectory.StudentQuery, carePlan careplan.Query) (BookingConsistencyAudit, error) {
	if db.Pool() == nil || students == nil || carePlan == nil {
		return nil, fmt.Errorf("booking consistency audit: database, People Directory and Care Plan are required")
	}
	audits, err := auditCompose.NewPersistence(auditCompose.PersistenceDependencies{
		DB:         db.Pool(),
		Enrollment: auditEnrollmentBookings{bookings: NewEnrollmentBookings()},
		Students:   auditStudentDirectory{students: students},
		CarePlan:   auditCarePlanDirectory{query: carePlan},
	})
	if err != nil {
		return nil, err
	}
	return audits.BookingConsistency.Audit, nil
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

// auditEnrollmentBookings serves the Audit's approved-booking read from
// Enrollment's booking projection in the audit's own vocabulary (#4072).
type auditEnrollmentBookings struct{ bookings *enrollment.BookingProjection }

func (b auditEnrollmentBookings) ApprovedBookings(ctx context.Context) ([]auditlog.ApprovedBooking, error) {
	bookings, err := b.bookings.ApprovedBookings(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]auditlog.ApprovedBooking, 0, len(bookings))
	for _, booking := range bookings {
		result = append(result, auditlog.ApprovedBooking{
			RequestChildID: booking.RequestChildID, StudentID: booking.StudentID,
			PhaseID: booking.PhaseID, TenantID: booking.TenantID,
			ServiceStartDate: string(booking.ServiceStartDate), ServiceEndDate: string(booking.ServiceEndDate),
			CareOfferingSelectionMode: booking.CareOfferingSelectionMode,
		})
	}
	return result, nil
}

func (b auditEnrollmentBookings) ApprovedBookingOfferingLinks(ctx context.Context) ([]auditlog.ApprovedBookingOfferingLink, error) {
	links, err := b.bookings.ApprovedBookingOfferingLinks(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]auditlog.ApprovedBookingOfferingLink, 0, len(links))
	for _, link := range links {
		result = append(result, auditlog.ApprovedBookingOfferingLink{
			ID: link.ID, TenantID: link.TenantID, RequestChildID: link.RequestChildID,
			CareOfferingID: link.CareOfferingID, SelectedDays: link.SelectedDays,
			ValidFrom: optionalDay(link.ValidFrom), ValidUntil: optionalDay(link.ValidUntil),
		})
	}
	return result, nil
}

func optionalDay(day *enrollment.Date) *string {
	if day == nil {
		return nil
	}
	value := string(*day)
	return &value
}
