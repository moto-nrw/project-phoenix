package groups

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

// The ports of the retained School Structure services (#2742). The service
// and the substitution module own these shapes; the legacy composition binds
// them over the retained repositories and the tenant runtime, so this package
// imports neither the People Directory, Facilities and Audit Platform models
// nor the tenant runtime and the ORM.

// Runtime runs the service's writes in the caller's tenant transaction.
type Runtime interface {
	// RunInTx joins the ambient tenant transaction or opens one for the
	// context's school.
	RunInTx(ctx context.Context, fn func(context.Context) error) error
	// TenantID is the school the caller acts for, 0 when none.
	TenantID(ctx context.Context) int64
	// MarkRollback asks the request's tenant transaction to roll back.
	MarkRollback(ctx context.Context)
}

// GroupRecords is the education.groups store of the group service.
type GroupRecords interface {
	Create(ctx context.Context, group *domain.Group) error
	FindByID(ctx context.Context, id any) (*domain.Group, error)
	FindByIDForUpdate(ctx context.Context, id any) (*domain.Group, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*domain.Group, error)
	FindByIDsWithRooms(ctx context.Context, ids []int64) (map[int64]*domain.Group, error)
	FindByName(ctx context.Context, name string) (*domain.Group, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*domain.Group, error)
	FindWithRoom(ctx context.Context, groupID int64) (*domain.Group, error)
	ListWithRooms(ctx context.Context, query *domain.GroupListQuery) ([]*domain.Group, error)
	CountGroups(ctx context.Context, query *domain.GroupListQuery) (int, error)
	Update(ctx context.Context, group *domain.Group) error
	Delete(ctx context.Context, id any) error
}

// GroupTeacherStore is the education.group_teacher store of the group service.
type GroupTeacherStore interface {
	Create(ctx context.Context, relation *domain.TeacherAssignment) error
	Delete(ctx context.Context, id any) error
	FindByGroup(ctx context.Context, groupID int64) ([]*domain.TeacherAssignment, error)
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*domain.TeacherAssignment, error)
}

// ClassTeacherStore is the store of the staff-to-school-class assignments
// (#1772).
type ClassTeacherStore interface {
	Create(ctx context.Context, assignment *domain.ClassAssignment) error
	Update(ctx context.Context, assignment *domain.ClassAssignment) error
	Delete(ctx context.Context, id any) error
	FindByStaff(ctx context.Context, staffID int64) ([]*domain.ClassAssignment, error)
}

// HandoverReader lists the substitutions of one group, the deletion guard's
// read.
type HandoverReader interface {
	FindByGroup(ctx context.Context, groupID int64) ([]*domain.GroupSubstitution, error)
}

// RoomDirectory resolves the room a group is assigned to. Any error means the
// room does not exist for the caller.
type RoomDirectory interface {
	FindRoom(ctx context.Context, id int64) (*domain.GroupRoom, error)
}

// StaffDirectory answers whether a staff member exists. A missing staff
// member is (false, nil); a failed read keeps its error.
type StaffDirectory interface {
	StaffExists(ctx context.Context, id int64) (bool, error)
}

// StudentCounter counts the children of each group.
type StudentCounter interface {
	CountByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error)
}

// TeacherDirectory reads the teachers groups are assigned to.
type TeacherDirectory interface {
	// FindTeacher fails when the teacher does not exist for the caller.
	FindTeacher(ctx context.Context, id int64) error
	// ListTeachers returns the teachers with their staff member's name; an
	// unknown ID is absent.
	ListTeachers(ctx context.Context, ids []int64) ([]*Teacher, error)
}

// Teacher is a teacher assigned to a group, as the group reads show it.
type Teacher = domain.Teacher

// TeacherPerson is the name and login of a teacher's person.
type TeacherPerson = domain.TeacherPerson

// ClassAssignmentAudit appends a rewrite of a staff member's class
// assignments to the Stammdaten audit trail (#1772).
type ClassAssignmentAudit interface {
	RecordSchoolClassChange(ctx context.Context, change domain.SchoolClassChange) error
}

// SubstitutionAudit appends to the responsibility trail.
type SubstitutionAudit interface {
	RecordSubstitutionChange(ctx context.Context, change domain.SubstitutionChange) error
}

// GroupPublisher emits lossy post-commit group-access invalidations.
type GroupPublisher = realtimeevents.Publisher
