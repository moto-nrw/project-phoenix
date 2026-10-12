package repositories

import (
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/uptrace/bun"
)

// EnrollmentTestRepositories is the enrollment suites' repository graph;
// DataAccessLog is filled by the composition that owns Audit persistence
// (services.NewEnrollmentTestRepositories).
type EnrollmentTestRepositories struct {
	TimetableTestRepositories
	School organizationtenancy.Capability

	StudentGuardian     usersModels.StudentGuardianRepository
	GuardianProfile     usersModels.GuardianProfileRepository
	GuardianPhoneNumber usersModels.GuardianPhoneNumberRepository
	Membership          schoolmembership.Capability
	DataAccessLog       auditModels.DataAccessLogRepository
}

func NewEnrollmentTestRepositories(db *bun.DB, members MembershipTestRepositories) (EnrollmentTestRepositories, error) {
	tt, err := NewTimetableTestRepositories(db, members)
	if err != nil {
		return EnrollmentTestRepositories{}, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return EnrollmentTestRepositories{}, err
	}
	return EnrollmentTestRepositories{TimetableTestRepositories: tt,
		School:          organizations,
		StudentGuardian: members.StudentGuardian, GuardianProfile: members.GuardianProfile, GuardianPhoneNumber: usersRepo.NewGuardianPhoneNumberRepository(peopleRuntime(db)),
		Membership: members.Membership}, nil
}
