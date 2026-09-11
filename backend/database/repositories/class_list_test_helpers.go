package repositories

import (
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

type ClassListTestRepositories struct {
	Entry   usersModels.ClassListEntryRepository
	Student usersModels.StudentRepository
	Audit   auditModels.ClassListEntryChangeRepository
}

func NewClassListTestRepositories(db *bun.DB, command auditModels.Command) (ClassListTestRepositories, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return ClassListTestRepositories{}, err
	}
	return ClassListTestRepositories{
		Entry:   classListEntryMembershipRepository{membership: membership},
		Student: usersRepo.NewStudentRepository(db), Audit: classListEntryChangeCommand{command: command},
	}, nil
}
