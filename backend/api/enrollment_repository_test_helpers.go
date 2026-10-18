//go:build testhooks

package api

import (
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// EnrollmentTestRepositories is the enrollment suites' repository graph;
// DataAccessLog is filled by the composition that owns Audit persistence
// (NewEnrollmentTestRepositories).
type EnrollmentTestRepositories struct {
	TimetableTestRepositories
	School organizationtenancy.Capability

	StudentGuardian     peoplePorts.StudentGuardianRepository
	GuardianProfile     peoplePorts.GuardianProfileRepository
	GuardianPhoneNumber peoplePorts.GuardianPhoneNumberRepository
	Membership          schoolmembership.Capability
	DataAccessLog       auditModels.DataAccessLogRepository
}

func NewEnrollmentTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories) (EnrollmentTestRepositories, error) {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return EnrollmentTestRepositories{}, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return EnrollmentTestRepositories{}, err
	}
	return EnrollmentTestRepositories{TimetableTestRepositories: tt,
		School:          organizations,
		StudentGuardian: members.StudentGuardian, GuardianProfile: members.GuardianProfile, GuardianPhoneNumber: peopleCompose.NewGuardianPhoneNumberRepository(db.Pool()),
		Membership: members.Membership}, nil
}
