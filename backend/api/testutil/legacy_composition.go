package testutil

import "github.com/moto-nrw/project-phoenix/services"

// The api and cmd suites may not import the legacy composition (#2750);
// these forward the owner compositions it builds for them (#2743).

// SessionRecordOwners are the owners the Student Presence session records
// read through.
type SessionRecordOwners = services.SessionRecordOwners

var (
	// NewOrganizationTenancy composes the Organization & Tenancy owner.
	NewOrganizationTenancy = services.NewOrganizationTenancy
	// NewSchoolMembership composes the School Membership owner.
	NewSchoolMembership = services.NewSchoolMembership
	// NewDeviceFleet composes the Device Fleet owner.
	NewDeviceFleet = services.NewDeviceFleet
	// NewDeviceRepository serves the retained device repository shape from
	// an unobserved Device Fleet owner.
	NewDeviceRepository = services.NewDeviceRepository
	// NewSessionRecords composes the Student Presence session records.
	NewSessionRecords = services.NewSessionRecords
	// NewIdentityAccessForTests composes an unobserved Identity & Access
	// owner.
	NewIdentityAccessForTests = services.NewIdentityAccessForTests
	// NewUnobservedCarePlan composes Care Plan over the timetable
	// dependencies.
	NewUnobservedCarePlan = services.NewUnobservedCarePlan
	// NewCareWithdrawalCompletionRepository serves the retained withdrawal
	// completion rows over Care Plan.
	NewCareWithdrawalCompletionRepository = services.NewCareWithdrawalCompletionRepository
	// NewEnrollmentTestRepositoriesWithMembers composes the enrollment
	// suites' repositories over the given membership repositories.
	NewEnrollmentTestRepositoriesWithMembers = services.NewEnrollmentTestRepositoriesWithMembers
	// NewRetainedTimetableRepositories builds the retained Timetable
	// adapters over the given dependencies.
	NewRetainedTimetableRepositories = services.NewTimetableRepositories
)
