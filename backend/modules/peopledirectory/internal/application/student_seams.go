package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	internalports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// NewStudentCompanions adapts Care Plan's companion seam to the module's
// port.
func NewStudentCompanions(seam ports.StudentCompanions) internalports.StudentCompanions {
	return studentCompanions{companionCoverage: seam, seam: seam}
}

// StudentOwnerWrites are the root's enrollment and care owners behind the
// module's port, with the class write lock the student store takes.
type StudentOwnerWrites interface {
	internalports.StudentOwners
	LockClassWrites(context.Context, bool) error
}

// NewStudentOwners adapts the root's enrollment and care owners to the
// module's port; unbound owners refuse every call.
func NewStudentOwners(owners ports.StudentOwners) StudentOwnerWrites {
	return studentOwners{owners: owners}
}

// studentCompanions adapts the composition root's public-typed seam to the
// module's internal port.
type studentCompanions struct {
	companionCoverage
	seam ports.StudentCompanions
}

// companionCoverage is the seam read the port takes as it is.
type companionCoverage interface {
	CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, error)
}

func (a studentCompanions) ListForStudent(ctx context.Context, studentID int64) ([]domain.CompanionEdge, error) {
	edges, err := a.seam.ListCompanionEdgesForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.CompanionEdge, 0, len(edges))
	for _, edge := range edges {
		result = append(result, domain.CompanionEdge(edge))
	}
	return result, nil
}

// DeleteEdges removes the edges and records that this write touched
// companion links, the only honest basis for announcing
// student_companions_changed: the signal rides on the caller's context, and
// this is the moment the links actually change.
func (a studentCompanions) DeleteEdges(ctx context.Context, edgeIDs []int64) error {
	if err := a.seam.DeleteCompanionEdges(ctx, edgeIDs); err != nil {
		return err
	}
	if len(edgeIDs) > 0 {
		peopledirectory.RecordCompanionChange(ctx)
	}
	return nil
}

type studentOwners struct{ owners ports.StudentOwners }

func (s studentOwners) LockClassWrites(ctx context.Context, exclusive bool) error {
	if s.owners == nil {
		return errors.New("people directory: student owners are not bound")
	}
	return s.owners.LockClassWrites(ctx, exclusive)
}

func (s studentOwners) Enroll(ctx context.Context, record domain.StudentRecord) (int64, error) {
	if s.owners == nil {
		return 0, errors.New("people directory: student owners are not bound")
	}
	return s.owners.Enroll(ctx, peopledirectory.StudentRecord(record))
}

func (s studentOwners) Renew(ctx context.Context, record domain.StudentRecord) (int64, error) {
	if s.owners == nil {
		return 0, errors.New("people directory: student owners are not bound")
	}
	return s.owners.Renew(ctx, peopledirectory.StudentRecord(record))
}

func (s studentOwners) SaveCare(ctx context.Context, membershipID int64, record domain.StudentRecord, plan domain.DeparturePlan, note *string, touched bool) error {
	if s.owners == nil {
		return errors.New("people directory: student owners are not bound")
	}
	return s.owners.SaveCare(ctx, membershipID, peopledirectory.StudentRecord(record), peopledirectory.StudentPlan{
		AllowedDepartureModes: plan.AllowedDepartureModes, DepartureDays: plan.DepartureDays, BusDays: plan.BusDays, PickupDays: plan.PickupDays,
	}, note, touched)
}
