package api

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

type RFIDTestRepositories struct {
	Membership services.MembershipTestRepositories
	RFID       identityaccess.RFIDCards
	Student    usersModels.StudentRepository
}

func NewRFIDTestRepositories(db transactionruntime.Database) (RFIDTestRepositories, error) {
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		return RFIDTestRepositories{}, err
	}
	return RFIDTestRepositories{Membership: members, RFID: services.NewPeopleIdentityAccess(db.Pool(), nil), Student: NewUnboundStudentRepository(db)}, nil
}

// NewStudentLookupTestRepository is the retained student repository for suites
// that only look children up.
func NewStudentLookupTestRepository(db transactionruntime.Database) usersModels.StudentRepository {
	return NewUnboundStudentRepository(db)
}

// NewGuardianProfileTestRepository is the guardian profile repository with the
// Identity & Access reachability queries bound.
func NewGuardianProfileTestRepository(db transactionruntime.Database) usersModels.GuardianProfileRepository {
	return services.NewGuardianProfileRepository(db.Pool())
}
