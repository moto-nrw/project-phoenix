package repositories

import (
	"context"

	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
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
// hands to its consumers. models/education no longer declares them: a domain
// package names neither the generic query shapes nor the People Directory
// rows (#2742). Every contract goes with the last consumer that uses it.

// EducationGroupRepository is the retained contract of education.groups,
// served by the School Structure composition adapter.
type EducationGroupRepository interface {
	Create(ctx context.Context, group *educationModels.Group) error
	FindByID(ctx context.Context, id any) (*educationModels.Group, error)
	Update(ctx context.Context, group *educationModels.Group) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*educationModels.Group, error)
	FindByIDForUpdate(ctx context.Context, id any) (*educationModels.Group, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*educationModels.Group, error)
	// FindByIDsWithRooms is the bulk sibling of FindWithRoom (#2094 review).
	FindByIDsWithRooms(ctx context.Context, ids []int64) (map[int64]*educationModels.Group, error)
	// Exists reports whether a group with the given ID exists in the current
	// tenant (issue #584).
	Exists(ctx context.Context, id int64) (bool, error)
	// ListWithRooms returns groups and their optional room.
	ListWithRooms(ctx context.Context, query *educationModels.GroupListQuery) ([]*educationModels.Group, error)
	// CountGroups counts the groups the overview filters select.
	CountGroups(ctx context.Context, query *educationModels.GroupListQuery) (int, error)
	FindByName(ctx context.Context, name string) (*educationModels.Group, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*educationModels.Group, error)
	FindWithRoom(ctx context.Context, groupID int64) (*educationModels.Group, error)
	// ListStaffIDsByEducationGroupIDs answers who supervises these groups on
	// the given day. It mirrors the caller context's group chain from the
	// group side, so the two must agree on the join shape.
	ListStaffIDsByEducationGroupIDs(ctx context.Context, groupIDs []int64, on calendar.Date) ([]educationModels.StaffGroupID, error)
}

// GroupTeacherRepository is the retained contract of education.group_teacher,
// served over School Membership.
type GroupTeacherRepository interface {
	Create(ctx context.Context, relation *educationModels.GroupTeacher) error
	FindByID(ctx context.Context, id any) (*educationModels.GroupTeacher, error)
	Update(ctx context.Context, relation *educationModels.GroupTeacher) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*educationModels.GroupTeacher, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*educationModels.GroupTeacher, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*educationModels.GroupTeacher, error)
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*educationModels.GroupTeacher, error)
	// ListGroupTeacherBlockers returns group assignments as
	// caregiver-capability blocker rows.
	ListGroupTeacherBlockers(ctx context.Context, teacherID, tenantID int64) ([]userModels.BlockerGroup, error)
}

// ClassTeacherRepository is the retained contract of the staff-to-school-class
// assignments (#1772), served over School Membership.
type ClassTeacherRepository interface {
	Create(ctx context.Context, assignment *educationModels.ClassTeacher) error
	FindByID(ctx context.Context, id any) (*educationModels.ClassTeacher, error)
	Update(ctx context.Context, assignment *educationModels.ClassTeacher) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*educationModels.ClassTeacher, error)
	// FindByStaff returns the class assignments of one staff member.
	FindByStaff(ctx context.Context, staffID int64) ([]*educationModels.ClassTeacher, error)
}

// GroupSubstitutionRepository is the retained contract of
// education.group_substitution, served over Workforce.
type GroupSubstitutionRepository interface {
	Create(ctx context.Context, substitution *educationModels.GroupSubstitution) error
	FindByID(ctx context.Context, id any) (*educationModels.GroupSubstitution, error)
	Update(ctx context.Context, substitution *educationModels.GroupSubstitution) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*educationModels.GroupSubstitution, error)
	FindByIDForUpdate(ctx context.Context, id any) (*educationModels.GroupSubstitution, error)
	// ListActiveSubstitutionBlockers returns current/upcoming typed group
	// handovers as caregiver-capability blocker rows.
	ListActiveSubstitutionBlockers(ctx context.Context, staffID, tenantID int64) ([]userModels.BlockerSubstitution, error)
	ListWithOptions(ctx context.Context, options *userModels.QueryOptions) ([]*educationModels.GroupSubstitution, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*educationModels.GroupSubstitution, error)
	// ListWithRelations is ListWithOptions with the group and staff attached.
	ListWithRelations(ctx context.Context, options *userModels.QueryOptions) ([]*educationModels.GroupSubstitution, error)
}
