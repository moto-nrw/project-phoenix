package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const phaseExpiryWarningDays = 30

var errPhaseExpiryTenantRequired = errors.New("phase expiry report requires a tenant context")

// PhaseExpirySnapshots lists the owner's expiry snapshots for a report day
// and a warning horizon.
type PhaseExpirySnapshots interface {
	ListSnapshots(ctx context.Context, asOf, warningThrough calendar.Date) ([]*enrollment.PhaseExpirySnapshot, error)
}

// PhaseExpiryProjection assembles the owner report inputs from the
// tenant-scoped read ports of People Directory and Care Plan.
type PhaseExpiryProjection struct {
	owner    ExpirySnapshots
	students enrollment.PhaseExpiryStudents
	carePlan enrollment.PhaseExpiryOfferings
	bookings enrollment.PhaseExpiryBookings
	tenantID func(context.Context) int64
}

// NewPhaseExpiryProjection binds the report to its read ports. tenantID
// returns the tenant in context, or 0 outside one.
func NewPhaseExpiryProjection(owner ExpirySnapshots, students enrollment.PhaseExpiryStudents, carePlan enrollment.PhaseExpiryOfferings, bookings enrollment.PhaseExpiryBookings, tenantID func(context.Context) int64) *PhaseExpiryProjection {
	return &PhaseExpiryProjection{owner: owner, students: students, carePlan: carePlan, bookings: bookings, tenantID: tenantID}
}

// directoryStudentArrays projects the tenant's non-alumni students into the
// parallel arrays the report query unnests: id, status, and the care window
// as YYYY-MM-DD text ("" for unset, cast to NULL in SQL).
func (r *PhaseExpiryProjection) directoryStudentArrays(ctx context.Context) ([]int64, []string, []string, []string, error) {
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

// ListSnapshots reads the owner's expiry snapshots for the tenant in
// context.
func (r *PhaseExpiryProjection) ListSnapshots(ctx context.Context, asOf, warningThrough calendar.Date) ([]*enrollment.PhaseExpirySnapshot, error) {
	if asOf.IsZero() || warningThrough.IsZero() {
		return nil, errors.New("phase expiry report dates are required")
	}
	if warningThrough.Before(asOf) {
		return nil, errors.New("phase expiry warning horizon must not be before the report date")
	}
	if r.tenantID == nil || r.tenantID(ctx) <= 0 {
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
	if r.bookings == nil {
		return nil, errors.New("phase expiry report requires Care Plan bookings")
	}
	links, err := r.bookings.AllCareOfferingLinks(ctx)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(links)
	if err != nil {
		return nil, fmt.Errorf("encode bookings for phase expiry report: %w", err)
	}

	return r.owner.PhaseExpirySnapshots(ctx, enrollment.PhaseExpiryInput{
		AsOf: enrollment.Date(asOf), WarningThrough: enrollment.Date(warningThrough), OfferingsJSON: offerings,
		BookingsJSON: string(encoded),
		StudentIDs:   ids, StudentStatuses: statuses, EnrolledFrom: enrolledFrom, EnrolledUntil: enrolledUntil,
	})
}

func (r *PhaseExpiryProjection) careOfferingProjection(ctx context.Context) (string, error) {
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

// PhaseExpiryWarnings turns the owner's expiry snapshots into the
// administrator-facing warnings.
type PhaseExpiryWarnings struct {
	repo PhaseExpirySnapshots
}

// NewPhaseExpiryWarnings builds the warning list over a snapshot source.
func NewPhaseExpiryWarnings(repo PhaseExpirySnapshots) *PhaseExpiryWarnings {
	return &PhaseExpiryWarnings{repo: repo}
}

// ListWarnings returns the phases whose care ends within the warning window
// without a complete successor.
func (s *PhaseExpiryWarnings) ListWarnings(ctx context.Context, asOf calendar.Date) ([]*enrollment.PhaseExpiryWarning, error) {
	if asOf.IsZero() {
		return nil, errors.New("phase expiry report date is required")
	}
	if s.repo == nil {
		return nil, errors.New("phase expiry repository is required")
	}

	snapshots, err := s.repo.ListSnapshots(ctx, asOf, asOf.AddDays(phaseExpiryWarningDays))
	if err != nil {
		return nil, fmt.Errorf("list phase expiry warnings: %w", err)
	}

	warnings := make([]*enrollment.PhaseExpiryWarning, 0, len(snapshots))
	for _, snapshot := range snapshots {
		warning, err := phaseExpiryWarning(snapshot, asOf)
		if err != nil {
			return nil, err
		}
		if warning != nil {
			warnings = append(warnings, warning)
		}
	}
	return warnings, nil
}

// phaseExpiryWarning is the warning of one snapshot, or nil for a successor
// that already resolves every affected child.
func phaseExpiryWarning(snapshot *enrollment.PhaseExpirySnapshot, asOf calendar.Date) (*enrollment.PhaseExpiryWarning, error) {
	if err := validatePhaseExpirySnapshot(snapshot); err != nil {
		return nil, err
	}
	firstAffectedDate, err := calendar.ParseDate(string(snapshot.FirstAffectedDate))
	if err != nil {
		return nil, fmt.Errorf("list phase expiry warnings: %w", err)
	}
	if snapshot.SuccessorPhaseID != nil && snapshot.UnresolvedChildren == 0 {
		return nil, nil
	}
	state := enrollment.PhaseExpiryStateMissingSuccessor
	if snapshot.SuccessorPhaseID != nil {
		state = enrollment.PhaseExpiryStateIncomplete
	}
	return &enrollment.PhaseExpiryWarning{
		SourcePhaseID:      snapshot.SourcePhaseID,
		SourcePhaseName:    snapshot.SourcePhaseName,
		SuccessorPhaseID:   snapshot.SuccessorPhaseID,
		SuccessorPhaseName: snapshot.SuccessorPhaseName,
		FirstAffectedDate:  firstAffectedDate,
		AffectedChildren:   snapshot.AffectedChildren,
		UnresolvedChildren: snapshot.UnresolvedChildren,
		State:              state,
		Overdue:            !firstAffectedDate.After(asOf),
	}, nil
}

func validatePhaseExpirySnapshot(snapshot *enrollment.PhaseExpirySnapshot) error {
	if snapshot == nil {
		return errors.New("phase expiry repository returned a nil snapshot")
	}
	if snapshot.SourcePhaseID <= 0 || snapshot.FirstAffectedDate.IsZero() {
		return fmt.Errorf("phase expiry repository returned an invalid snapshot for phase %d", snapshot.SourcePhaseID)
	}
	if snapshot.AffectedChildren <= 0 {
		return fmt.Errorf("phase expiry repository returned no affected children for phase %d", snapshot.SourcePhaseID)
	}
	if snapshot.UnresolvedChildren < 0 || snapshot.UnresolvedChildren > snapshot.AffectedChildren {
		return fmt.Errorf("phase expiry repository returned invalid unresolved count for phase %d", snapshot.SourcePhaseID)
	}
	return nil
}
