package api

import (
	"context"
	"errors"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"

	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync"
)

// The owner reads behind School Structure's rule of who supervises a group
// on a day (#2667, #2688): the teacher and staff rows come from School
// Membership, the substitutions from Workforce. The rule itself is School
// Structure's; these bindings only translate.

// GroupSubstitutionRow is the Workforce row the group substitution contract
// reads and writes.
type GroupSubstitutionRow = workforceCompose.GroupSubstitution

// EducationHandovers serves the substitution module's handover store: the
// shift-plan-sync workflow writes the handovers into Workforce's rows, the
// reads come from the retained group substitution repository. The binding
// translates the rows into the School Structure values (#3556).
type EducationHandovers struct {
	substitutions owners.GroupSubstitutions
	handovers     shiftplansync.GroupHandovers
}

// NewEducationHandovers binds the handover store to the group substitution
// reads and the handover writes.
func NewEducationHandovers(substitutions owners.GroupSubstitutions, handovers shiftplansync.GroupHandovers) EducationHandovers {
	return EducationHandovers{substitutions: substitutions, handovers: handovers}
}

// Create stores a handover and reports educationRepo.ErrHandoverExists for
// a duplicate.
func (h EducationHandovers) Create(ctx context.Context, handover *educationRepo.GroupSubstitution) error {
	if handover == nil {
		return errors.New("group_substitution cannot be nil or zero value")
	}
	created, err := h.handovers.CreateHandover(ctx, workforceHandover(handover))
	if errors.Is(err, shiftplansync.ErrHandoverExists) {
		return educationRepo.ErrHandoverExists
	}
	if err != nil {
		return err
	}
	stored := educationHandoverFromWorkforce(created)
	stored.Group, stored.RegularStaff, stored.SubstituteStaff = handover.Group, handover.RegularStaff, handover.SubstituteStaff
	*handover = *stored
	return nil
}

// Delete removes a handover.
func (h EducationHandovers) Delete(ctx context.Context, id any) error {
	handoverID, err := owners.MembershipID(id)
	if err != nil {
		return err
	}
	return h.handovers.DeleteHandover(ctx, handoverID)
}

// FindByID reads one handover.
func (h EducationHandovers) FindByID(ctx context.Context, id any) (*educationRepo.GroupSubstitution, error) {
	return educationHandoverResult(h.substitutions.FindByID(ctx, id))
}

// FindByIDForUpdate reads one handover under a row lock.
func (h EducationHandovers) FindByIDForUpdate(ctx context.Context, id any) (*educationRepo.GroupSubstitution, error) {
	return educationHandoverResult(h.substitutions.FindByIDForUpdate(ctx, id))
}

// ListHandovers selects the handovers of one school.
func (h EducationHandovers) ListHandovers(ctx context.Context, query educationRepo.HandoverQuery) ([]*educationRepo.GroupSubstitution, error) {
	return owners.EducationHandoversBinding(h.substitutions.ListWithFilter(ctx, owners.HandoverFilter(query)))
}

// ListHandoversWithRelations is ListHandovers with the group and the staff
// members attached.
func (h EducationHandovers) ListHandoversWithRelations(ctx context.Context, query educationRepo.HandoverQuery) ([]*educationRepo.GroupSubstitution, error) {
	return h.substitutions.ListWithRelations(ctx, query)
}

func educationHandoverResult(row *workforceCompose.GroupSubstitution, err error) (*educationRepo.GroupSubstitution, error) {
	if err != nil {
		return nil, err
	}
	return owners.EducationHandover(row), nil
}

// workforceHandover is the Workforce value of a School Structure handover.
func workforceHandover(handover *educationRepo.GroupSubstitution) workforce.GroupSubstitution {
	return workforce.GroupSubstitution{
		ID: handover.ID, TenantID: handover.TenantID, TargetType: handover.TargetType, GroupID: handover.GroupID,
		RegularStaffID: handover.RegularStaffID, SubstituteStaffID: handover.SubstituteStaffID,
		StartDate: handover.StartDate.String(), EndDate: handover.EndDate.String(), Reason: handover.Reason,
		CreatedAt: handover.CreatedAt, UpdatedAt: handover.UpdatedAt,
	}
}

// educationHandoverFromWorkforce is the School Structure value of a stored
// Workforce handover.
func educationHandoverFromWorkforce(value workforce.GroupSubstitution) *educationRepo.GroupSubstitution {
	return &educationRepo.GroupSubstitution{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		TargetType: value.TargetType, GroupID: value.GroupID, RegularStaffID: value.RegularStaffID,
		SubstituteStaffID: value.SubstituteStaffID, StartDate: timezone.Date(value.StartDate), EndDate: timezone.Date(value.EndDate), Reason: value.Reason,
	}
}
