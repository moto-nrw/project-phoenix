// Package compose binds request-review ports to native owner capabilities.
package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	structurecompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/uptrace/bun"
)

// Students is the People Directory read needed for group decoration.
type Students interface {
	ListStudentsByID(context.Context, []int64) ([]peopledirectory.Student, error)
}

type DirectoryObservation = structurecompose.Observation

// NewStudentDirectory binds group decoration to native owner reads. A missing
// optional people source leaves decoration disabled.
func NewStudentDirectory(db *bun.DB, people Students, observe func(DirectoryObservation)) (requestreview.StudentDirectory, error) {
	if people == nil {
		return nil, nil
	}
	groups, err := structurecompose.New(structurecompose.Dependencies{DB: db, Observe: observe})
	if err != nil {
		return nil, err
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
