package repositories

import (
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/uptrace/bun"
)

type RFIDTestRepositories struct {
	Membership MembershipTestRepositories
	RFID       identityaccess.RFIDCards
	Student    usersModels.StudentRepository
}

func NewRFIDTestRepositories(db *bun.DB) (RFIDTestRepositories, error) {
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		return RFIDTestRepositories{}, err
	}
	return RFIDTestRepositories{Membership: members, RFID: newIdentityAccess(db, nil), Student: NewStudentRepository(db)}, nil
}
