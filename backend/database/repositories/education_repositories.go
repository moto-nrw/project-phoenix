package repositories

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// NewEducationGroupRepository is the retained group store on the tenant
// runtime of the School Structure owner.
func NewEducationGroupRepository(db *bun.DB, options ...educationRepo.GroupRepositoryDependencies) *educationGroupRepository {
	var deps educationRepo.GroupRepositoryDependencies
	if len(options) > 0 {
		deps = options[0]
	}
	directory := &educationRoomDirectory{}
	if deps.Rooms == nil {
		deps.Rooms = func() educationRepo.GroupRoomLookup {
			if directory.rooms == nil {
				return nil
			}
			return directory
		}
	}
	return &educationGroupRepository{GroupRepository: educationRepo.NewGroupRepository(educationRepo.NewLegacyRepositoryRuntime(db), deps), rooms: directory}
}

type educationGroupRepository struct {
	*educationRepo.GroupRepository
	rooms *educationRoomDirectory
}

// The retained School Structure repository contracts the legacy composition
// hands to its consumers, over the owner's values its composition names
// (#2742, #3556). Every contract goes with the last consumer that uses it.

// EducationGroupRepository is the retained contract of education.groups,
// served by the School Structure composition adapter.
type EducationGroupRepository interface {
	Create(ctx context.Context, group *educationRepo.Group) error
	FindByID(ctx context.Context, id any) (*educationRepo.Group, error)
	Update(ctx context.Context, group *educationRepo.Group) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*educationRepo.Group, error)
	FindByIDForUpdate(ctx context.Context, id any) (*educationRepo.Group, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*educationRepo.Group, error)
	// FindByIDsWithRooms is the bulk sibling of FindWithRoom (#2094 review).
	FindByIDsWithRooms(ctx context.Context, ids []int64) (map[int64]*educationRepo.Group, error)
	// Exists reports whether a group with the given ID exists in the current
	// tenant (issue #584).
	Exists(ctx context.Context, id int64) (bool, error)
	// ListWithRooms returns groups and their optional room.
	ListWithRooms(ctx context.Context, query *educationRepo.GroupListQuery) ([]*educationRepo.Group, error)
	// CountGroups counts the groups the overview filters select.
	CountGroups(ctx context.Context, query *educationRepo.GroupListQuery) (int, error)
	FindByName(ctx context.Context, name string) (*educationRepo.Group, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*educationRepo.Group, error)
	FindWithRoom(ctx context.Context, groupID int64) (*educationRepo.Group, error)
	// ListStaffIDsByEducationGroupIDs answers who supervises these groups on
	// the given day. It mirrors the caller context's group chain from the
	// group side, so the two must agree on the join shape.
	ListStaffIDsByEducationGroupIDs(ctx context.Context, groupIDs []int64, on calendar.Date) ([]educationRepo.StaffGroupID, error)
	// ListGroupSupervisors is the same answer for the staff notification
	// recipients, in their own pair type (#3556).
	ListGroupSupervisors(ctx context.Context, groupIDs []int64, on calendar.Date) ([]notifications.StaffGroupPair, error)
}

// ListGroupSupervisors answers who supervises the groups on the day for the
// staff notification recipients.
func (r *educationGroupRepository) ListGroupSupervisors(ctx context.Context, groupIDs []int64, on calendar.Date) ([]notifications.StaffGroupPair, error) {
	pairs, err := r.ListStaffIDsByEducationGroupIDs(ctx, groupIDs, on)
	if err != nil {
		return nil, err
	}
	result := make([]notifications.StaffGroupPair, 0, len(pairs))
	for _, pair := range pairs {
		result = append(result, notifications.StaffGroupPair{StaffID: pair.StaffID, GroupID: pair.GroupID})
	}
	return result, nil
}

// GroupTeacherRepository is the retained contract of education.group_teacher,
// served over School Membership.
type GroupTeacherRepository interface {
	Create(ctx context.Context, relation *schoolmembership.GroupAssignment) error
	FindByID(ctx context.Context, id any) (*schoolmembership.GroupAssignment, error)
	Update(ctx context.Context, relation *schoolmembership.GroupAssignment) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*schoolmembership.GroupAssignment, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*schoolmembership.GroupAssignment, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*schoolmembership.GroupAssignment, error)
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*schoolmembership.GroupAssignment, error)
	// The group service's teacher assignment store (#3556).
	educationRepo.GroupTeacherStore
	// ListGroupTeacherBlockers returns group assignments as
	// caregiver-capability blocker rows.
	ListGroupTeacherBlockers(ctx context.Context, teacherID, tenantID int64) ([]userModels.BlockerGroup, error)
}

// ClassTeacherRepository is the retained contract of the staff-to-school-class
// assignments (#1772), served over School Membership.
type ClassTeacherRepository interface {
	Create(ctx context.Context, assignment *schoolmembership.ClassAssignment) error
	FindByID(ctx context.Context, id any) (*schoolmembership.ClassAssignment, error)
	Update(ctx context.Context, assignment *schoolmembership.ClassAssignment) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*schoolmembership.ClassAssignment, error)
	// The class assignment service's store (#3556).
	educationRepo.ClassTeacherStore
	// FindByStaff returns the class assignments of one staff member.
	FindByStaff(ctx context.Context, staffID int64) ([]*schoolmembership.ClassAssignment, error)
}

// GroupSubstitutionRow is the Workforce row the group substitution contract
// reads and writes.
type GroupSubstitutionRow = workforceLegacy.GroupSubstitution

// GroupSubstitutionRepository is the retained contract of
// education.group_substitution: the Workforce rows, and the reads with
// relations as School Structure values.
type GroupSubstitutionRepository interface {
	GroupSubstitutionRows
	// FindGroupHandovers lists the substitutions of one group as School
	// Structure values, the group service's deletion guard.
	FindGroupHandovers(ctx context.Context, groupID int64) ([]*educationRepo.GroupSubstitution, error)
	// ListWithRelations is ListWithOptions with the group and staff attached.
	ListWithRelations(ctx context.Context, options *userModels.QueryOptions) ([]*educationRepo.GroupSubstitution, error)
}

// GroupSubstitutionRows are the Workforce rows of the contract, served over
// the Workforce capability.
type GroupSubstitutionRows interface {
	Create(ctx context.Context, substitution *workforceLegacy.GroupSubstitution) error
	FindByID(ctx context.Context, id any) (*workforceLegacy.GroupSubstitution, error)
	Update(ctx context.Context, substitution *workforceLegacy.GroupSubstitution) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*workforceLegacy.GroupSubstitution, error)
	FindByIDForUpdate(ctx context.Context, id any) (*workforceLegacy.GroupSubstitution, error)
	// ListActiveSubstitutionBlockers returns current/upcoming typed group
	// handovers as caregiver-capability blocker rows.
	ListActiveSubstitutionBlockers(ctx context.Context, staffID, tenantID int64) ([]userModels.BlockerSubstitution, error)
	ListWithOptions(ctx context.Context, options *userModels.QueryOptions) ([]*workforceLegacy.GroupSubstitution, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*workforceLegacy.GroupSubstitution, error)
}
