// Package compose binds request-review ports to native owner capabilities.
package requestreviewbinding

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// Students is the People Directory read needed for group decoration.
type Students interface {
	ListStudentsByID(context.Context, []int64) ([]peopledirectory.Student, error)
}

// NewStudentDirectory binds group decoration to the People Directory and the
// School Structure the root builds. A missing optional people source leaves
// decoration disabled.
func NewStudentDirectory(people Students, groups schoolstructure.Query) (requestreview.StudentDirectory, error) {
	if people == nil {
		return nil, nil
	}
	if groups == nil {
		return nil, errors.New("request review student directory: school structure is required")
	}
	return directory{people: people, groups: groups}, nil
}

type directory struct {
	people Students
	groups schoolstructure.Query
}

func (d directory) GroupNames(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	students, err := d.people.ListStudentsByID(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	groupIDs := make([]int64, 0, len(students))
	for _, student := range students {
		if student.GroupID != nil {
			groupIDs = append(groupIDs, *student.GroupID)
		}
	}
	groups, err := d.groups.ListGroupsByID(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	groupNames := make(map[int64]string, len(groups))
	for _, group := range groups {
		groupNames[group.ID] = group.Name
	}
	names := make(map[int64]string, len(students))
	for _, student := range students {
		if student.GroupID != nil {
			if name, ok := groupNames[*student.GroupID]; ok {
				names[student.ID] = name
			}
		}
	}
	return names, nil
}
