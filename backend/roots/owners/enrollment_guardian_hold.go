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
type EnrollmentGuardianHold interface {
	HoldAccountStudentPermission(ctx context.Context, accountID, studentID, tenantID int64, permission string) (bool, error)
	HoldGuardianEmailStudentPermission(ctx context.Context, email string, studentID, tenantID int64, permission string) (bool, error)
}

// enrollmentGuardianHold binds Enrollment's held renewal permission
// (#6638) to the parent guard's protocol (guardianhold.Hold): Identity &
// Access's membership lock and People Directory's child, profile and
// relationship rows, the same locks the Parent Portal's child write guard
// takes.
type enrollmentGuardianHold struct {
	relationships peopledirectory.GuardianRelationshipReads
	students      peopledirectory.StudentSnapshotReads
	profiles      peopledirectory.GuardianProfileReads
	membership    func(context.Context, int64) (bool, error)
}

// NewEnrollmentGuardianHold binds the held renewal permission of
// Enrollment to People Directory and Identity & Access.
func NewEnrollmentGuardianHold(db transactionruntime.Database, relationships peopledirectory.GuardianRelationshipReads, students peopledirectory.StudentSnapshotReads, profiles peopledirectory.GuardianProfileReads) EnrollmentGuardianHold {
	return enrollmentGuardianHold{
		relationships: relationships, students: students, profiles: profiles,
		membership: identityaccessCompose.NewSchoolMembershipLock(db.Pool()),
	}
}

// HoldAccountStudentPermission holds the account's relationship to the
// student with the permission until the caller's school transaction ends.
func (a enrollmentGuardianHold) HoldAccountStudentPermission(ctx context.Context, accountID, studentID, tenantID int64, permission string) (bool, error) {
	if err := enrollmentHoldTenant(ctx, tenantID); err != nil {
		return false, err
	}
	profile, err := a.profiles.GuardianProfileByAccount(ctx, accountID)
	if found, err := enrollmentHoldProfile(profile, err, tenantID); !found || err != nil {
		return false, err
	}
	return a.hold(ctx, guardianhold.Request{AccountID: accountID, StudentID: studentID, ProfileID: profile.ID, Permissions: []string{permission}})
}

// HoldGuardianEmailStudentPermission holds the relationship of the guardian
// the address names; a profile with an account needs its membership as well.
func (a enrollmentGuardianHold) HoldGuardianEmailStudentPermission(ctx context.Context, email string, studentID, tenantID int64, permission string) (bool, error) {
	if err := enrollmentHoldTenant(ctx, tenantID); err != nil {
		return false, err
	}
	profile, err := a.profiles.GuardianProfileByEmail(ctx, email)
	if found, err := enrollmentHoldProfile(profile, err, tenantID); !found || err != nil {
		return false, err
	}
	return a.hold(ctx, guardianhold.Request{Email: email, StudentID: studentID, ProfileID: profile.ID, Permissions: []string{permission}})
}

func (a enrollmentGuardianHold) hold(ctx context.Context, req guardianhold.Request) (bool, error) {
	err := guardianhold.Hold(ctx, a.locks(), req)
	if errors.Is(err, guardianhold.ErrNotLinked) || errors.Is(err, guardianhold.ErrPermissionDenied) {
		return false, nil
	}
	return err == nil, err
}

func (a enrollmentGuardianHold) locks() guardianhold.Locks {
	return guardianhold.Locks{Membership: a.membership, Child: a.lockChild, Profile: a.lockProfile, Relationship: a.lockRelationship, Owner: a.owner}
}

func (a enrollmentGuardianHold) lockChild(ctx context.Context, studentID int64) error {
	_, err := a.students.StudentSnapshotForUpdate(ctx, studentID)
	return err
}

func (a enrollmentGuardianHold) lockProfile(ctx context.Context, profileID int64) (bool, error) {
	err := a.profiles.LockGuardianProfile(ctx, profileID)
	if errors.Is(err, peopledirectory.ErrGuardianProfileNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (a enrollmentGuardianHold) lockRelationship(ctx context.Context, studentID, profileID int64) (func(string) bool, error) {
	link, err := a.relationships.GuardianRelationshipForUpdate(ctx, studentID, profileID)
	if errors.Is(err, peopledirectory.ErrStudentGuardianNotFound) || (err == nil && link == nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return func(permission string) bool {
		return securityruntime.GuardianPermissionGrantedJSON(link.Permissions, permission)
	}, nil
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
