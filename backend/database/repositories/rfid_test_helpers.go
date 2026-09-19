package repositories

import (
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	authModels "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	authRepo "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authpostgres"
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
	return RFIDTestRepositories{Membership: members, RFID: authRepo.NewRFIDCardRepository(db), Student: NewStudentRepository(db)}, nil
}
