package services

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
	return RFIDTestRepositories{Membership: members, RFID: newPeopleIdentityAccess(db, nil), Student: NewUnboundStudentRepository(db)}, nil
}

// NewStudentLookupTestRepository is the retained student repository for suites
// that only look children up.
func NewStudentLookupTestRepository(db *bun.DB) usersModels.StudentRepository {
	return NewUnboundStudentRepository(db)
}

// NewGuardianProfileTestRepository is the guardian profile repository with the
// Identity & Access reachability queries bound.
func NewGuardianProfileTestRepository(db *bun.DB) usersModels.GuardianProfileRepository {
	return NewGuardianProfileRepository(db)
}
