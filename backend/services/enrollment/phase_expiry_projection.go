package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/tenant"
)

var errPhaseExpiryTenantRequired = errors.New("phase expiry report requires a tenant context")

type phaseExpiryProjection struct {
	owner    PhaseExpiryReader
	students PhaseExpiryStudents
	carePlan PhaseExpiryOfferings
}

// PhaseExpiryOffering is the narrow owner data used by phase-expiry reports.
type PhaseExpiryOffering struct {
	ID             int64    `json:"id"`
	TenantID       int64    `json:"tenant_id"`
	PhaseID        int64    `json:"phase_id"`
	DaysOfWeekMode string   `json:"days_of_week_mode"`
	AvailableDays  []string `json:"available_days"`
	IsActive       bool     `json:"is_active"`
}

// PhaseExpiryOfferings supplies the current tenant's care offerings.
type PhaseExpiryOfferings interface {
	ListCareOfferings(context.Context) ([]PhaseExpiryOffering, error)
}

// NewPhaseExpiryProjection assembles owner inputs from tenant-scoped read ports.
func NewPhaseExpiryProjection(owner PhaseExpiryReader, students PhaseExpiryStudents, carePlan PhaseExpiryOfferings) PhaseExpirySnapshots {
	return &phaseExpiryProjection{owner: owner, students: students, carePlan: carePlan}
}

type PhaseExpiryStudent struct {
	ID            int64
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}

type PhaseExpiryStudents interface {
	ListEnrolledStudents(context.Context) ([]PhaseExpiryStudent, error)
}

// directoryStudentArrays projects the tenant's non-alumni students into the
// parallel arrays the report query unnests: id, status, and the care window
// as YYYY-MM-DD text (” for unset, cast to NULL in SQL).
func (r *phaseExpiryProjection) directoryStudentArrays(ctx context.Context) ([]int64, []string, []string, []string, error) {
	if r.students == nil {
		return nil, nil, nil, nil, errors.New("enrollment repositories: student directory is not bound")
	}
	students, err := r.students.ListEnrolledStudents(ctx)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	ids := make([]int64, 0, len(students))
	statuses := make([]string, 0, len(students))
	from := make([]string, 0, len(students))
	until := make([]string, 0, len(students))
	for _, student := range students {
		ids = append(ids, student.ID)
		statuses = append(statuses, student.Status)
		from = append(from, student.EnrolledFrom)
		until = append(until, student.EnrolledUntil)
	}
	return ids, statuses, from, until, nil
}

func (r *phaseExpiryProjection) ListSnapshots(
	ctx context.Context,
	asOf, warningThrough timezone.Date,
) ([]*capability.PhaseExpirySnapshot, error) {
	if asOf.IsZero() || warningThrough.IsZero() {
		return nil, errors.New("phase expiry report dates are required")
	}
	if warningThrough.Before(asOf) {
		return nil, errors.New("phase expiry warning horizon must not be before the report date")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, errPhaseExpiryTenantRequired
	}

	ids, statuses, enrolledFrom, enrolledUntil, err := r.directoryStudentArrays(ctx)
	if err != nil {
		return nil, err
	}
	offerings, err := r.careOfferingProjection(ctx)
	if err != nil {
		return nil, err
	}

	return r.owner.PhaseExpirySnapshots(ctx, capability.PhaseExpiryInput{
		AsOf: capability.Date(asOf), WarningThrough: capability.Date(warningThrough), OfferingsJSON: offerings,
		StudentIDs: ids, StudentStatuses: statuses, EnrolledFrom: enrolledFrom, EnrolledUntil: enrolledUntil,
	})
}

func (r *phaseExpiryProjection) careOfferingProjection(ctx context.Context) (string, error) {
	if r.carePlan == nil {
		return "", errors.New("phase expiry report requires the Care Plan capability")
	}
	offerings, err := r.carePlan.ListCareOfferings(ctx)
	if err != nil {
		return "", fmt.Errorf("list care offerings for phase expiry report: %w", err)
	}
	encoded, err := json.Marshal(offerings)
	if err != nil {
		return "", fmt.Errorf("encode care offerings for phase expiry report: %w", err)
	}
	return string(encoded), nil
}

type PhaseExpiryReader interface {
	PhaseExpirySnapshots(context.Context, capability.PhaseExpiryInput) ([]*capability.PhaseExpirySnapshot, error)
}
