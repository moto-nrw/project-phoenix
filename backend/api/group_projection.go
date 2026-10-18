package api

import (
	"context"

	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"

	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// groupStudentRepository and OverlappingRosterGroupNames name the groups of
// the student roster and birthday reads through the School Structure owner, so
// no student read joins education.groups itself.
type OverlappingRosterGroupNames struct {
	peoplePorts.StudentRepository
	Groups groupNameQuery
}

// groupNameQuery is the School Structure read the decorators name the groups
// of their rows through: the two reads schoolStructureCompose.EnrichGroupNames
// takes.
type groupNameQuery interface {
	FindGroup(ctx context.Context, id int64) (schoolstructure.Group, error)
	ListGroupsByID(ctx context.Context, ids []int64) ([]schoolstructure.Group, error)
}

func (r OverlappingRosterGroupNames) FindByTeacherStaffIDsWithGroups(ctx context.Context, staffIDs []int64) ([]*userModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindByTeacherStaffIDsWithGroups(ctx, staffIDs)
	return rows, err
}

func (r OverlappingRosterGroupNames) FindOverlappingWithGroups(ctx context.Context, from, to, today timezone.Date) ([]*userModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindOverlappingWithGroups(ctx, from, to, today)
	return EnrichStudentGroupNames(ctx, r.Groups, rows, err)
}

type groupStudentRepository struct {
	peoplePorts.StudentRepository
	groups groupNameQuery
}

func (r groupStudentRepository) FindByTeacherIDWithGroups(ctx context.Context, teacherID int64) ([]*userModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindByTeacherIDWithGroups(ctx, teacherID)
	return EnrichStudentGroupNames(ctx, r.groups, rows, err)
}

func (r groupStudentRepository) FindByTeacherStaffIDsWithGroups(ctx context.Context, staffIDs []int64) ([]*userModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindByTeacherStaffIDsWithGroups(ctx, staffIDs)
	return EnrichStudentGroupNames(ctx, r.groups, rows, err)
}

func (r groupStudentRepository) FindAllWithGroups(ctx context.Context) ([]*userModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindAllWithGroups(ctx)
	return EnrichStudentGroupNames(ctx, r.groups, rows, err)
}

// EnrichStudentGroupNames fills StudentWithGroupInfo.GroupName from the
// School Structure owner.
func EnrichStudentGroupNames(ctx context.Context, groups schoolstructure.Query, rows []*userModels.StudentWithGroupInfo, queryErr error) ([]*userModels.StudentWithGroupInfo, error) {
	if queryErr != nil {
		return nil, queryErr
	}
	return schoolStructureCompose.EnrichGroupNames(ctx, groups, rows,
		func(row *userModels.StudentWithGroupInfo) int64 {
			if row.Student == nil {
				return 0
			}
			return schoolStructureCompose.OptionalGroupID(row.GroupID)
		},
		func(row *userModels.StudentWithGroupInfo, name string) { row.GroupName = name },
		"", "student roster")
}

func (r groupStudentRepository) FindBirthdaysOn(ctx context.Context, days []userModels.MonthDay) ([]userModels.BirthdayEntry, error) {
	rows, err := r.StudentRepository.FindBirthdaysOn(ctx, days)
	if err != nil {
		return nil, err
	}
	pointers := make([]*userModels.BirthdayEntry, len(rows))
	for index := range rows {
		pointers[index] = &rows[index]
	}
	if _, err := schoolStructureCompose.EnrichGroupNames(ctx, r.groups, pointers,
		func(row *userModels.BirthdayEntry) int64 { return schoolStructureCompose.OptionalGroupID(row.GroupID) },
		func(row *userModels.BirthdayEntry, name string) { row.GroupName = name },
		"", "birthdays"); err != nil {
		return nil, err
	}
	return rows, nil
}
