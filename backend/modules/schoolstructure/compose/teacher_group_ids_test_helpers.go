package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/education"
)

// TeacherGroupRecords supplies the existing assignment query at composition.
type TeacherGroupRecords interface {
	GetTeacherGroups(context.Context, int64) ([]*education.Group, error)
}

type TeacherGroupIDs struct{ records TeacherGroupRecords }

func NewTeacherGroupIDs(records TeacherGroupRecords) *TeacherGroupIDs {
	return &TeacherGroupIDs{records: records}
}

func (r *TeacherGroupIDs) TeacherGroupIDs(ctx context.Context, teacherID int64) ([]int64, error) {
	groups, err := r.records.GetTeacherGroups(ctx, teacherID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			ids = append(ids, group.ID)
		}
	}
	return ids, nil
}
