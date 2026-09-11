package repositories

import (
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

type RFIDTestRepositories struct {
	Membership MembershipTestRepositories
	RFID       authModels.RFIDCardRepository
	Student    usersModels.StudentRepository
}

func NewRFIDTestRepositories(db *bun.DB) (RFIDTestRepositories, error) {
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		return RFIDTestRepositories{}, err
	}
	return RFIDTestRepositories{Membership: members, RFID: authRepo.NewRFIDCardRepository(db), Student: usersRepo.NewStudentRepository(db)}, nil
}
