package owners

import (
	"context"
	"errors"
	"fmt"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/guardianhold"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// EnrollmentGuardianHold is Enrollment's held renewal permission (#6638).
// Candidates are locked as one ordered set before guardian profiles and
// relationships; grants are returned per child.
type EnrollmentGuardianHold interface {
	HoldAccountStudentsPermission(ctx context.Context, accountID int64, studentIDs []int64, tenantID int64, permission string) (map[int64]bool, error)
	HoldGuardianEmailStudentsPermission(ctx context.Context, email string, studentIDs []int64, tenantID int64, permission string) (map[int64]bool, error)
}

// enrollmentGuardianHold binds Enrollment's held renewal permission
// (#6638) to the parent guard's protocol (guardianhold.HoldMany): Identity &
// Access's membership lock and People Directory's child, profile and
// relationship rows, the same locks the Parent Portal's child write guard
// takes.
type enrollmentGuardianHold struct {
	relationships peopledirectory.GuardianRelationshipReads
	students      EnrollmentStudentMatches
	profiles      peopledirectory.GuardianProfileReads
	membership    func(context.Context, int64) (bool, error)
}

// NewEnrollmentGuardianHold binds the held renewal permission of
// Enrollment to People Directory and Identity & Access.
func NewEnrollmentGuardianHold(db transactionruntime.Database, relationships peopledirectory.GuardianRelationshipReads, students EnrollmentStudentMatches, profiles peopledirectory.GuardianProfileReads) EnrollmentGuardianHold {
	return enrollmentGuardianHold{
		relationships: relationships, students: students, profiles: profiles,
		membership: identityaccessCompose.NewSchoolMembershipLock(db.Pool()),
	}
}

// HoldAccountStudentsPermission holds the account's relationship to the
// student with the permission until the caller's school transaction ends.
func (a enrollmentGuardianHold) HoldAccountStudentsPermission(ctx context.Context, accountID int64, studentIDs []int64, tenantID int64, permission string) (map[int64]bool, error) {
	if err := enrollmentHoldTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	profile, err := a.profiles.GuardianProfileByAccount(ctx, accountID)
	if found, err := enrollmentHoldProfile(profile, err, tenantID); !found || err != nil {
		return nil, err
	}
	return a.hold(ctx, guardianhold.Request{AccountID: accountID, ProfileID: profile.ID, Permissions: []string{permission}}, studentIDs)
}

// HoldGuardianEmailStudentsPermission holds the relationship of the guardian
// the address names; a profile with an account needs its membership as well.
func (a enrollmentGuardianHold) HoldGuardianEmailStudentsPermission(ctx context.Context, email string, studentIDs []int64, tenantID int64, permission string) (map[int64]bool, error) {
	if err := enrollmentHoldTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	profile, err := a.profiles.GuardianProfileByEmail(ctx, email)
	if found, err := enrollmentHoldProfile(profile, err, tenantID); !found || err != nil {
		return nil, err
	}
	return a.hold(ctx, guardianhold.Request{Email: email, ProfileID: profile.ID, Permissions: []string{permission}}, studentIDs)
}

func (a enrollmentGuardianHold) hold(ctx context.Context, req guardianhold.Request, studentIDs []int64) (map[int64]bool, error) {
	grants, err := guardianhold.HoldMany(ctx, guardianhold.BatchLocks{
		Locks:    guardianhold.Locks{Membership: a.membership, Profile: a.lockProfile, Owner: a.owner},
		Children: a.lockChildren, Relationships: a.lockRelationships,
	}, req, studentIDs)
	if errors.Is(err, guardianhold.ErrNotLinked) || errors.Is(err, guardianhold.ErrPermissionDenied) {
		return nil, nil
	}
	return grants, err
}

func (a enrollmentGuardianHold) lockChildren(ctx context.Context, studentIDs []int64) error {
	students, err := a.students.FindByIDsForUpdate(ctx, studentIDs)
	if err != nil {
		return err
	}
	for _, id := range studentIDs {
		if students[id] == nil {
			return guardianhold.ErrNotLinked
		}
	}
	return nil
}

func (a enrollmentGuardianHold) lockProfile(ctx context.Context, profileID int64) (bool, error) {
	err := a.profiles.LockGuardianProfile(ctx, profileID)
	if errors.Is(err, peopledirectory.ErrGuardianProfileNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (a enrollmentGuardianHold) lockRelationships(ctx context.Context, studentIDs []int64, profileID int64) (map[int64]func(string) bool, error) {
	links, err := a.relationships.GuardianRelationshipsForUpdate(ctx, studentIDs, profileID)
	if err != nil {
		return nil, err
	}
	grants := make(map[int64]func(string) bool, len(links))
	for _, link := range links {
		grants[link.StudentID] = func(permission string) bool {
			return securityruntime.GuardianPermissionGrantedJSON(link.Permissions, permission)
		}
	}
	return grants, nil
}

func (a enrollmentGuardianHold) owner(ctx context.Context, profileID int64) (*int64, string, bool, error) {
	profile, err := a.profiles.GuardianProfileByID(ctx, profileID)
	if errors.Is(err, peopledirectory.ErrGuardianProfileNotFound) || (err == nil && profile == nil) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}
	email := ""
	if profile.Email != nil {
		email = *profile.Email
	}
	return profile.AccountID, email, true, nil
}

// enrollmentHoldTenant refuses a hold outside the school transaction of the
// tenant the renewal names: the membership lock reads the school in context.
func enrollmentHoldTenant(ctx context.Context, tenantID int64) error {
	if scoped := tenant.FromContext(ctx); scoped <= 0 || scoped != tenantID {
		return fmt.Errorf("enrollment guardian hold: school transaction of tenant %d required", tenantID)
	}
	return nil
}

// enrollmentHoldProfile reports whether the identity has a guardian profile
// at the school.
func enrollmentHoldProfile(profile *peopledirectory.GuardianProfileSnapshot, err error, tenantID int64) (bool, error) {
	if errors.Is(err, peopledirectory.ErrGuardianProfileNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("enrollment guardian hold: resolve guardian profile: %w", err)
	}
	return profile != nil && profile.TenantID == tenantID, nil
}
