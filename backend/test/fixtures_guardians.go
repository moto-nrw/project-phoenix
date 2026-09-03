package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Guardian fixtures (#2663). The guardian rows belong to the People
// Directory; tests that need a contact person with a specific name, or a
// link with a specific role, build it here instead of naming the owner's
// repositories.

// CreateTestGuardianProfileNamed creates a guardian profile with the given
// names in the test's tenant. email is made unique.
func CreateTestGuardianProfileNamed(tb testing.TB, db *bun.DB, firstName, lastName, email string) *users.GuardianProfile {
	tb.Helper()
	return CreateTestGuardianProfileForTenant(tb, db, fixtureTenantID(tb), firstName, lastName, email)
}

// CreateTestGuardianProfileForTenant creates a guardian profile in tenantID.
// email is made unique.
func CreateTestGuardianProfileForTenant(tb testing.TB, db *bun.DB, tenantID int64, firstName, lastName, email string) *users.GuardianProfile {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uniqueEmail := fmt.Sprintf(testEmailFormat, email, uniqueFixtureSuffix())
	profile := &users.GuardianProfile{
		FirstName:              firstName,
		LastName:               lastName,
		Email:                  &uniqueEmail,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	profile.SetTenantID(tenantID)
	err := db.NewInsert().Model(profile).ModelTableExpr(`users.guardian_profiles`).Scan(ctx)
	require.NoError(tb, err, "Failed to create test guardian profile")
	return profile
}

// CreateTestStudentGuardianLink links a guardian to a student in the test's
// tenant with the given role preset; the role decides the parents-portal
// permissions exactly as the production link path does.
func CreateTestStudentGuardianLink(tb testing.TB, db *bun.DB, studentID, guardianProfileID int64, role string) *users.StudentGuardian {
	tb.Helper()
	return CreateTestStudentGuardianLinkForTenant(tb, db, fixtureTenantID(tb), studentID, guardianProfileID, role)
}

// CreateTestStudentGuardianLinkForTenant links a guardian to a student in
// tenantID with the given role preset.
func CreateTestStudentGuardianLinkForTenant(tb testing.TB, db *bun.DB, tenantID, studentID, guardianProfileID int64, role string) *users.StudentGuardian {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	link := &users.StudentGuardian{
		StudentID:          studentID,
		GuardianProfileID:  guardianProfileID,
		RelationshipType:   "parent",
		IsEmergencyContact: true,
		CanPickup:          true,
		EmergencyPriority:  1,
	}
	authorize.ApplyStudentGuardianRole(link, role)
	link.SetTenantID(tenantID)
	err := db.NewInsert().Model(link).ModelTableExpr(`users.students_guardians`).Scan(ctx)
	require.NoError(tb, err, "Failed to create students_guardians link")
	return link
}

// StudentGuardianLinkGrantsPortalAccess reports whether the stored link grants
// parent_portal.access, read straight from the row so a test can pin what a
// write left behind.
func StudentGuardianLinkGrantsPortalAccess(tb testing.TB, db *bun.DB, linkID int64) bool {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var link users.StudentGuardian
	err := db.NewSelect().Model(&link).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Where(`"student_guardian".id = ?`, linkID).
		Scan(ctx)
	require.NoError(tb, err, "Failed to load students_guardians link")
	return authorize.StudentGuardianHasPermission(&link, authorize.GuardianPermissionPortalAccess)
}
