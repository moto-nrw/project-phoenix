package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// overlappingRosterGroupNames completes the School Structure cutover for the
// one student roster read whose calendar-date signature keeps its decorator
// out of the repository composition (see repositories.BindSchoolStructure).
type overlappingRosterGroupNames struct {
	userModels.StudentRepository
	groups schoolstructure.Query
}

func (r overlappingRosterGroupNames) FindByTeacherIDsWithGroups(ctx context.Context, teacherIDs []int64) ([]*userModels.StudentWithGroupInfo, error) {
	return r.StudentRepository.FindByTeacherIDsWithGroups(ctx, teacherIDs)
}

func (r overlappingRosterGroupNames) FindOverlappingWithGroups(ctx context.Context, from, to, today timezone.Date) ([]*userModels.StudentWithGroupInfo, error) {
	rows, err := r.StudentRepository.FindOverlappingWithGroups(ctx, from, to, today)
	return repositories.EnrichStudentGroupNames(ctx, r.groups, rows, err)
}
