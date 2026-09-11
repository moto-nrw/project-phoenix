// Package users_test tests the users service layer with hermetic testing pattern.
package users_test

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	usermodels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// isProfileLockTimeout reports whether err carries a PostgreSQL lock_timeout
// (55P03), used by the guardian-profile serialization test to prove a writer
// blocked on a held FOR UPDATE lock rather than racing through it.
func isProfileLockTimeout(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "lock timeout") ||
		strings.Contains(msg, "lock_not_available") ||
		strings.Contains(msg, "55P03")
}

// setupGuardianService creates a GuardianService with real database connection
func setupGuardianService(t *testing.T, db *bun.DB) *users.GuardianService {
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	serviceFactory, err := services.NewFactoryForTests(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Guardian
}

// =============================================================================
// CreateGuardian Tests
// =============================================================================

func TestGuardianService_CreateGuardian(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates guardian successfully", func(t *testing.T) {
		// ARRANGE - use unique email to avoid collisions
		email := fmt.Sprintf("test-guardian-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Test",
			LastName:               "Guardian",
			Email:                  &email,
			PreferredContactMethod: "email",
			LanguagePreference:     "de",
		}

		// ACT
		result, err := service.CreateGuardian(ctx, req)
		defer func() {
		}()

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, result.ID, int64(0))
		assert.Equal(t, "Test", result.FirstName)
		assert.Equal(t, "Guardian", result.LastName)
		assert.Equal(t, &email, result.Email)
		assert.False(t, result.HasAccount)
	})

	t.Run("creates guardian with defaults", func(t *testing.T) {
		// ARRANGE - testing default language and contact method
		// Note: Phone numbers are now added separately via AddPhoneNumber
		req := users.GuardianCreateRequest{
			FirstName: "Default",
			LastName:  "Guardian",
		}

		// ACT
		result, err := service.CreateGuardian(ctx, req)
		defer func() {
		}()

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, "phone", result.PreferredContactMethod)
		assert.Equal(t, "de", result.LanguagePreference)
	})

	t.Run("creates guardian without email", func(t *testing.T) {
		// ARRANGE - guardian can be created without email
		// Phone numbers are added separately via AddPhoneNumber
		req := users.GuardianCreateRequest{
			FirstName: "NoEmail",
			LastName:  "Guardian",
		}

		// ACT
		result, err := service.CreateGuardian(ctx, req)
		defer func() {
		}()

		// ASSERT
		require.NoError(t, err)
		assert.Nil(t, result.Email)
	})
}

// TestGuardianService_CreateGuardian_DuplicateEmail verifies that creating a
// second guardian with an email already in use returns a *ValidationError
// (rendered as a 400 with a German "use the search" message) instead of letting
// the tenant-scoped UNIQUE(tenant_id, email) index fail the INSERT with a raw
// 23505 that would surface as a generic 500 (#1513).
func TestGuardianService_CreateGuardian_DuplicateEmail(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	email := fmt.Sprintf("dup-guardian-%d@example.com", time.Now().UnixNano())
	req := users.GuardianCreateRequest{
		FirstName:              "First",
		LastName:               "Guardian",
		Email:                  &email,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}

	// First create succeeds.
	first, err := service.CreateGuardian(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, first)

	// Second create with the SAME email (different name) must be rejected as a
	// validation error, not a DB/operational failure — and nothing is inserted.
	dupReq := req
	dupReq.FirstName = "Second"
	second, err := service.CreateGuardian(ctx, dupReq)

	require.Error(t, err)
	assert.Nil(t, second)
	var validationErr *users.ValidationError
	require.ErrorAs(t, err, &validationErr, "duplicate email must be a ValidationError → 400, not a 500")
	assert.Contains(t, err.Error(), "bereits vergeben")
	assert.Contains(t, err.Error(), "Suche")
}

// =============================================================================
// GetGuardianByID Tests
// =============================================================================

func TestGuardianService_GetGuardianByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns guardian when found", func(t *testing.T) {
		// ARRANGE
		profile := testpkg.CreateTestGuardianProfile(t, db, "get-by-id")

		// ACT
		result, err := service.GetGuardianByID(ctx, profile.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, profile.ID, result.ID)
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetGuardianByID(ctx, 99999999)

		// ASSERT - may return error or nil depending on repository
		if err != nil {
			assert.Nil(t, result)
		} else {
			assert.Nil(t, result)
		}
	})
}

// =============================================================================
// UpdateGuardian Tests
// =============================================================================

func TestGuardianService_UpdateGuardian(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates guardian successfully", func(t *testing.T) {
		// ARRANGE
		profile := testpkg.CreateTestGuardianProfile(t, db, "to-update")

		// Use unique email to avoid collisions
		newEmail := fmt.Sprintf("updated-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Updated",
			LastName:               "Name",
			Email:                  &newEmail,
			PreferredContactMethod: "email",
			LanguagePreference:     "en",
		}

		// ACT
		err := service.UpdateGuardian(ctx, profile.ID, req)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		updated, err := service.GetGuardianByID(ctx, profile.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated", updated.FirstName)
		assert.Equal(t, "Name", updated.LastName)
	})

	t.Run("returns error when guardian not found", func(t *testing.T) {
		// ARRANGE
		req := users.GuardianCreateRequest{
			FirstName: "NonExistent",
			LastName:  "Guardian",
		}

		// ACT
		err := service.UpdateGuardian(ctx, 99999999, req)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("rejects an email already owned by another guardian", func(t *testing.T) {
		// ARRANGE — two distinct guardians.
		other := testpkg.CreateTestGuardianProfile(t, db, "update-dedup-other")
		target := testpkg.CreateTestGuardianProfile(t, db, "update-dedup-target")

		req := users.GuardianCreateRequest{
			FirstName:              "Target",
			LastName:               "Guardian",
			Email:                  other.Email, // steal the other guardian's email
			PreferredContactMethod: "email",
			LanguagePreference:     "de",
		}

		// ACT
		err := service.UpdateGuardian(ctx, target.ID, req)

		// ASSERT — must be a ValidationError (→ 400), never a raw 500 from 23505.
		require.Error(t, err)
		var validationErr *users.ValidationError
		require.ErrorAs(t, err, &validationErr, "duplicate email on update must be a ValidationError → 400")
		assert.Contains(t, err.Error(), "bereits vergeben")
	})

	t.Run("allows saving a guardian without changing its own email (self-exclusion)", func(t *testing.T) {
		// ARRANGE — a guardian kept at its own email is NOT a collision with itself.
		profile := testpkg.CreateTestGuardianProfile(t, db, "update-self")

		req := users.GuardianCreateRequest{
			FirstName:              "Renamed",
			LastName:               "Same-Email",
			Email:                  profile.Email, // unchanged
			PreferredContactMethod: "email",
			LanguagePreference:     "de",
		}

		// ACT
		err := service.UpdateGuardian(ctx, profile.ID, req)

		// ASSERT
		require.NoError(t, err, "re-saving a guardian with its own email must not collide")
		updated, err := service.GetGuardianByID(ctx, profile.ID)
		require.NoError(t, err)
		assert.Equal(t, "Renamed", updated.FirstName)
	})
}

// =============================================================================
// DeleteGuardian Tests
// =============================================================================

func TestGuardianService_DeleteGuardian(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes guardian successfully", func(t *testing.T) {
		// ARRANGE
		profile := testpkg.CreateTestGuardianProfile(t, db, "to-delete")
		require.NoError(t, service.UpdateGuardianPayment(ctx, profile.ID, users.GuardianPaymentInput{
			IBAN:          strPtr("DE89370400440532013000"),
			AccountHolder: strPtr("Sabine Schneider"),
		}, 1, ""))
		// No defer - we're testing deletion

		// ACT
		err := service.DeleteGuardian(ctx, profile.ID, 1)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		result, _ := service.GetGuardianByID(ctx, profile.ID)
		assert.Nil(t, result)

		var deletionAudit auditModels.GuardianFinancialChange
		require.NoError(t, db.NewSelect().
			Model(&deletionAudit).
			ModelTableExpr(`audit.guardian_financial_changes AS "guardian_financial_change"`).
			Where(`"guardian_financial_change".guardian_profile_id = ?`, profile.ID).
			Where(`"guardian_financial_change".field_name = ?`, auditModels.GuardianPaymentFieldIBAN).
			Where(`"guardian_financial_change".new_value = ?`, "").
			Scan(ctx))
		assert.Equal(t, "•••• 3000", deletionAudit.OldValue)
		assert.Equal(t, "Erziehungsberechtigte Person gelöscht", deletionAudit.Note)

		var accountHolderAudits []auditModels.GuardianFinancialChange
		require.NoError(t, db.NewSelect().
			Model(&accountHolderAudits).
			ModelTableExpr(`audit.guardian_financial_changes AS "guardian_financial_change"`).
			Where(`"guardian_financial_change".guardian_profile_id = ?`, profile.ID).
			Where(`"guardian_financial_change".field_name = ?`, auditModels.GuardianPaymentFieldAccountHolder).
			Scan(ctx))
		require.Len(t, accountHolderAudits, 2)
		for _, audit := range accountHolderAudits {
			assert.NotContains(t, audit.OldValue, "Sabine")
			assert.NotContains(t, audit.NewValue, "Sabine")
		}

		var accountHolderDeletionAudit auditModels.GuardianFinancialChange
		require.NoError(t, db.NewSelect().
			Model(&accountHolderDeletionAudit).
			ModelTableExpr(`audit.guardian_financial_changes AS "guardian_financial_change"`).
			Where(`"guardian_financial_change".guardian_profile_id = ?`, profile.ID).
			Where(`"guardian_financial_change".field_name = ?`, auditModels.GuardianPaymentFieldAccountHolder).
			Where(`"guardian_financial_change".new_value = ?`, "").
			Scan(ctx))
		assert.Equal(t, "••••••••", accountHolderDeletionAudit.OldValue)
		assert.Equal(t, "Erziehungsberechtigte Person gelöscht", accountHolderDeletionAudit.Note)
	})

	t.Run("plain delete is refused while the guardian is still linked (RESTRICT)", func(t *testing.T) {
		// ARRANGE — guardian linked to a student. Since migration 1.15.127 the
		// students_guardians → guardian_profiles FK is ON DELETE RESTRICT, so a
		// blind delete must fail instead of silently cascading the link away
		// (the #819 sibling-data-loss bug).
		guardian := testpkg.CreateTestGuardianProfile(t, db, "restrict-linked")
		student := testpkg.CreateTestStudent(t, db, "Restrict", "Linked", "1a")
		_, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			EmergencyPriority: 1,
		})
		require.NoError(t, err)

		// ACT
		err = service.DeleteGuardian(ctx, guardian.ID, 1)

		// ASSERT — the FK violation surfaces; the guardian survives.
		require.Error(t, err, "RESTRICT FK must block deleting a linked guardian")
		survivor, _ := service.GetGuardianByID(ctx, guardian.ID)
		assert.NotNil(t, survivor, "guardian must still exist after a refused delete")
	})
}

// =============================================================================
// DeleteGuardianWithLinks + GetGuardianDeleteImpact Tests (#819)
// =============================================================================

func TestGuardianService_DeleteGuardianWithLinks(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE — guardian linked to two students (the sibling case #819 is about).
	guardian := testpkg.CreateTestGuardianProfile(t, db, "force-delete")
	siblingA := testpkg.CreateTestStudent(t, db, "Sibling", "Aaa", "1a")
	siblingB := testpkg.CreateTestStudent(t, db, "Sibling", "Bbb", "1a")
	for _, s := range []int64{siblingA.ID, siblingB.ID} {
		_, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:         s,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			EmergencyPriority: 1,
		})
		require.NoError(t, err)
	}
	for _, studentID := range []int64{siblingA.ID, siblingB.ID} {
		require.NoError(t, service.SetStudentPayer(ctx, studentID, &guardian.ID, 1))
	}

	// The delete preview reports both children before deletion.
	impact, err := service.GetGuardianDeleteImpact(ctx, guardian.ID)
	require.NoError(t, err)
	assert.Len(t, impact.StudentNames, 2, "both linked children must be reported for the 409 warning")
	require.Len(t, impact.LinkIDs, 2, "both link IDs must be returned as the delete concurrency token")

	// ACT — the deliberate full delete removes links first, then the guardian.
	// Run inside a tenant transaction: DeleteGuardianWithLinks documents that it
	// MUST run in one (the SELECT ... FOR UPDATE row lock and the link-then-
	// guardian ordering are only meaningful/atomic within a single tx), and the
	// HTTP handler wraps it that way. Exercising the real contract here.
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		return service.DeleteGuardianWithLinks(txCtx, guardian.ID, impact.LinkIDs, 1)
	})

	// ASSERT — guardian gone, and no links survive.
	require.NoError(t, err)
	gone, _ := service.GetGuardianByID(ctx, guardian.ID)
	assert.Nil(t, gone, "guardian must be deleted by the force path")
	remainingImpact, err := service.GetGuardianDeleteImpact(ctx, guardian.ID)
	require.NoError(t, err)
	assert.Empty(t, remainingImpact.StudentNames, "all student links must be removed")

	var unassignments []auditModels.GuardianFinancialChange
	require.NoError(t, db.NewSelect().
		Model(&unassignments).
		ModelTableExpr(`audit.guardian_financial_changes AS "guardian_financial_change"`).
		Where(`"guardian_financial_change".guardian_profile_id = ?`, guardian.ID).
		Where(`"guardian_financial_change".field_name = ?`, auditModels.GuardianPaymentFieldIsPayer).
		Where(`"guardian_financial_change".new_value = ?`, "false").
		Scan(ctx))
	assert.Len(t, unassignments, 2, "every deleted payer relationship must be audited")
}

func TestGuardianService_DeleteGuardianWithLinks_RejectsChangedPreview(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	guardian := testpkg.CreateTestGuardianProfile(t, db, "force-delete-stale")
	student := testpkg.CreateTestStudent(t, db, "Preview", "Changed", "1a")

	_, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
		StudentID:         student.ID,
		GuardianProfileID: guardian.ID,
		RelationshipType:  "parent",
		EmergencyPriority: 1,
	})
	require.NoError(t, err)

	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		return service.DeleteGuardianWithLinks(txCtx, guardian.ID, []int64{999999}, 1)
	})

	require.ErrorIs(t, err, users.ErrGuardianDeletePreviewChanged)
	survivor, err := service.GetGuardianByID(ctx, guardian.ID)
	require.NoError(t, err)
	assert.NotNil(t, survivor, "guardian must survive when preview token is stale")
	remainingImpact, err := service.GetGuardianDeleteImpact(ctx, guardian.ID)
	require.NoError(t, err)
	assert.Len(t, remainingImpact.StudentNames, 1, "link must survive when preview token is stale")
}

// =============================================================================
// LinkGuardianToStudent Tests
// =============================================================================

func TestGuardianService_LinkGuardianToStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("links guardian to student successfully", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "link-to-student")
		student := testpkg.CreateTestStudent(t, db, "Linked", "Student", "1a")

		req := users.StudentGuardianCreateRequest{
			StudentID:          student.ID,
			GuardianProfileID:  guardian.ID,
			RelationshipType:   "parent",
			IsPrimary:          true,
			IsEmergencyContact: true,
			CanPickup:          true,
			EmergencyPriority:  1,
		}

		// ACT
		result, err := service.LinkGuardianToStudent(ctx, req)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, student.ID, result.StudentID)
		assert.Equal(t, guardian.ID, result.GuardianProfileID)
		assert.Equal(t, "parent", result.RelationshipType)
		assert.True(t, result.IsPrimary)
	})

	t.Run("is idempotent when the guardian is already linked", func(t *testing.T) {
		// ARRANGE — link a guardian once.
		guardian := testpkg.CreateTestGuardianProfile(t, db, "relink")
		student := testpkg.CreateTestStudent(t, db, "Relink", "Student", "1c")

		req := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			EmergencyPriority: 1,
		}
		first, err := service.LinkGuardianToStudent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, first)

		// ACT — link the SAME guardian to the SAME student again.
		second, err := service.LinkGuardianToStudent(ctx, req)

		// ASSERT — no UNIQUE-constraint error; the existing relationship is
		// returned, and only one link exists.
		require.NoError(t, err, "re-linking an already-linked guardian must not error")
		require.NotNil(t, second)
		assert.Equal(t, first.ID, second.ID, "re-link must return the existing relationship, not create a new one")

		links, err := service.GetStudentGuardians(ctx, student.ID)
		require.NoError(t, err)
		var count int
		for _, l := range links {
			if l.Profile != nil && l.Profile.ID == guardian.ID {
				count++
			}
		}
		assert.Equal(t, 1, count, "exactly one link must exist after re-linking")
	})

	t.Run("re-linking is a no-op and never overwrites the existing relationship flags", func(t *testing.T) {
		// Linking is a CREATE, not an edit: a re-link must return the existing
		// row untouched. Changing flags goes through the dedicated update path
		// (UpdateStudentGuardianRelationship). This guards against a stray or
		// retried link request silently flipping pickup/emergency-contact flags.
		// ARRANGE — link a guardian with a minimal relationship.
		guardian := testpkg.CreateTestGuardianProfile(t, db, "upsert")
		student := testpkg.CreateTestStudent(t, db, "Upsert", "Student", "1d")

		first, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			IsPrimary:         false,
			CanPickup:         false,
			EmergencyPriority: 1,
		})
		require.NoError(t, err)
		require.NotNil(t, first)

		// ACT — re-link the SAME guardian with DIFFERENT relationship flags.
		second, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:          student.ID,
			GuardianProfileID:  guardian.ID,
			RelationshipType:   "guardian",
			IsPrimary:          true,
			IsEmergencyContact: true,
			CanPickup:          true,
			EmergencyPriority:  2,
		})

		// ASSERT — same single row, and the flags STILL reflect the ORIGINAL
		// link, not the re-link request (the changed flags were ignored).
		require.NoError(t, err, "re-linking must not error")
		require.NotNil(t, second)
		assert.Equal(t, first.ID, second.ID, "re-link must return the existing row, not create a new one")

		links, err := service.GetStudentGuardians(ctx, student.ID)
		require.NoError(t, err)
		var matched int
		for _, l := range links {
			if l.Profile == nil || l.Profile.ID != guardian.ID {
				continue
			}
			matched++
			require.NotNil(t, l.Relationship)
			assert.Equal(t, "parent", l.Relationship.RelationshipType, "relationship type must be unchanged by a re-link")
			assert.False(t, l.Relationship.IsPrimary, "is_primary must be unchanged by a re-link")
			assert.False(t, l.Relationship.IsEmergencyContact, "is_emergency_contact must be unchanged by a re-link")
			assert.False(t, l.Relationship.CanPickup, "can_pickup must be unchanged by a re-link")
			assert.Equal(t, 1, l.Relationship.EmergencyPriority, "emergency_priority must be unchanged by a re-link")
		}
		assert.Equal(t, 1, matched, "exactly one link must exist after re-linking")
	})

	t.Run("returns error when guardian not found", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "NoGuardian", "Student", "1b")

		req := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: 99999999,
			RelationshipType:  "parent",
		}

		// ACT
		result, err := service.LinkGuardianToStudent(ctx, req)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "guardian")
	})

	t.Run("returns error when student not found", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "orphan-guardian")

		req := users.StudentGuardianCreateRequest{
			StudentID:         99999999,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
		}

		// ACT
		result, err := service.LinkGuardianToStudent(ctx, req)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "student")
	})

	t.Run("applies explicit pickup-only role without portal permissions", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "link-pickup-only")
		student := testpkg.CreateTestStudent(t, db, "PickupOnly", "Student", "4a")

		result, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "relative",
			GuardianRole:      authorize.GuardianRolePickupOnly,
			CanPickup:         true,
			EmergencyPriority: 1,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, authorize.GuardianRolePickupOnly, result.GuardianRole)
		assert.True(t, result.CanPickup)
		assert.False(t, authorize.StudentGuardianHasPermission(result, authorize.GuardianPermissionPortalAccess))
		assert.False(t, authorize.StudentGuardianHasPermission(result, authorize.GuardianPermissionSickNoteSubmit))
	})

	t.Run("defaults primary relationship to full portal permissions", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "link-primary-role")
		student := testpkg.CreateTestStudent(t, db, "PrimaryRole", "Student", "4a")

		result, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			IsPrimary:         true,
			EmergencyPriority: 1,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, authorize.GuardianRolePrimaryGuardian, result.GuardianRole)
		assert.True(t, authorize.StudentGuardianHasPermission(result, authorize.GuardianPermissionPortalAccess))
		assert.True(t, authorize.StudentGuardianHasPermission(result, authorize.GuardianPermissionSickNoteSubmit))
		assert.True(t, authorize.StudentGuardianHasPermission(result, authorize.GuardianPermissionNotesWrite))
	})
}

// =============================================================================
// GetStudentGuardians Tests
// =============================================================================

func TestGuardianService_GetStudentGuardians(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns guardians for student", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "student-guardian")
		student := testpkg.CreateTestStudent(t, db, "HasGuardian", "Student", "2a")

		// Link guardian to student
		req := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			IsPrimary:         true,
		}
		_, err := service.LinkGuardianToStudent(ctx, req)
		require.NoError(t, err)

		// ACT
		result, err := service.GetStudentGuardians(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, guardian.ID, result[0].Profile.ID)
		// No account and no invitation → not pending.
		assert.False(t, result[0].InvitationPending,
			"guardian without account or invitation must not be pending")
	})

	t.Run("marks guardian with an open invitation as pending", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "pending-invite")
		student := testpkg.CreateTestStudent(t, db, "PendingInvite", "Student", "2c")

		_, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
		})
		require.NoError(t, err)

		// An open invitation: not accepted, not expired, not rejected.
		inviter := testpkg.CreateTestAccount(t, db, "pending-inviter")
		repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
		invitation := &authModels.GuardianInvitation{
			Token:             fmt.Sprintf("pending-token-%d", time.Now().UnixNano()),
			GuardianProfileID: guardian.ID,
			CreatedBy:         inviter.ID,
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			ApprovalStatus:    authModels.GuardianInvitationApprovalNotRequired,
		}
		invitation.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repoFactory.GuardianInvitation.Create(ctx, invitation))

		// ACT
		result, err := service.GetStudentGuardians(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.True(t, result[0].InvitationPending,
			"guardian with an open invitation must be marked pending")
	})

	t.Run("returns empty list when no guardians", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "NoGuardians", "Student", "2b")

		// ACT
		result, err := service.GetStudentGuardians(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// GetGuardianStudents Tests
// =============================================================================

func TestGuardianService_GetGuardianStudents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns students for guardian", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "has-students")
		student := testpkg.CreateTestStudent(t, db, "GuardianChild", "Student", "3a")

		// Link guardian to student
		req := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "guardian",
		}
		_, err := service.LinkGuardianToStudent(ctx, req)
		require.NoError(t, err)

		// ACT
		result, err := service.GetGuardianStudents(ctx, guardian.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, student.ID, result[0].Student.ID)
	})

	t.Run("excludes graduated students", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "alumnus-child")
		student := testpkg.CreateTestStudent(t, db, "Former", "Student", "4a")

		_, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "guardian",
		})
		require.NoError(t, err)

		_, err = db.NewUpdate().
			TableExpr("users.students").
			Set("status = ?", string(usermodels.StudentStatusAlumnus)).
			Where("id = ?", student.ID).
			Exec(ctx)
		require.NoError(t, err)

		result, err := service.GetGuardianStudents(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns empty list when no students", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "no-students")

		// ACT
		result, err := service.GetGuardianStudents(ctx, guardian.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// GetStudentGuardianRelationship Tests
// =============================================================================

func TestGuardianService_GetStudentGuardianRelationship(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns relationship by ID", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "rel-get")
		student := testpkg.CreateTestStudent(t, db, "RelGet", "Student", "4a")

		// Create relationship
		req := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
		}
		created, err := service.LinkGuardianToStudent(ctx, req)
		require.NoError(t, err)

		// ACT
		result, err := service.GetStudentGuardianRelationship(ctx, created.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, created.ID, result.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetStudentGuardianRelationship(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// UpdateStudentGuardianRelationship Tests
// =============================================================================

func TestGuardianService_UpdateStudentGuardianRelationship(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates relationship successfully", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "rel-update")
		student := testpkg.CreateTestStudent(t, db, "RelUpdate", "Student", "5a")

		// Create relationship
		createReq := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			IsPrimary:         false,
		}
		created, err := service.LinkGuardianToStudent(ctx, createReq)
		require.NoError(t, err)

		// Update
		newType := "guardian"
		isPrimary := true
		isEmergencyContact := true
		canPickup := true
		pickupNotes := "Nur mit Ausweis"
		emergencyPriority := 2
		updateReq := users.StudentGuardianUpdateRequest{
			RelationshipType:   &newType,
			IsPrimary:          &isPrimary,
			IsEmergencyContact: &isEmergencyContact,
			CanPickup:          &canPickup,
			PickupNotes:        &pickupNotes,
			EmergencyPriority:  &emergencyPriority,
		}

		// ACT
		err = service.UpdateStudentGuardianRelationship(ctx, created.ID, updateReq)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		updated, err := service.GetStudentGuardianRelationship(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "guardian", updated.RelationshipType)
		assert.True(t, updated.IsPrimary)
		assert.True(t, updated.IsEmergencyContact)
		assert.True(t, updated.CanPickup)
		require.NotNil(t, updated.PickupNotes)
		assert.Equal(t, pickupNotes, *updated.PickupNotes)
		assert.Equal(t, emergencyPriority, updated.EmergencyPriority)
	})

	t.Run("updates role and derived permissions", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "rel-update-role")
		student := testpkg.CreateTestStudent(t, db, "RelUpdateRole", "Student", "5b")

		created, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			GuardianRole:      authorize.GuardianRoleLegalGuardian,
			EmergencyPriority: 1,
		})
		require.NoError(t, err)
		require.True(t, authorize.StudentGuardianHasPermission(created, authorize.GuardianPermissionPortalAccess))

		role := authorize.GuardianRoleEmergency
		err = service.UpdateStudentGuardianRelationship(ctx, created.ID, users.StudentGuardianUpdateRequest{
			GuardianRole: &role,
		})
		require.NoError(t, err)

		updated, err := service.GetStudentGuardianRelationship(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, authorize.GuardianRoleEmergency, updated.GuardianRole)
		assert.False(t, authorize.StudentGuardianHasPermission(updated, authorize.GuardianPermissionPortalAccess))
		assert.False(t, authorize.StudentGuardianHasPermission(updated, authorize.GuardianPermissionNotesWrite))
	})

	t.Run("updates every optional relationship field", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "rel-update-all")
		student := testpkg.CreateTestStudent(t, db, "RelUpdateAll", "Student", "5b")

		created, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:          student.ID,
			GuardianProfileID:  guardian.ID,
			RelationshipType:   "parent",
			IsPrimary:          false,
			IsEmergencyContact: false,
			CanPickup:          false,
			EmergencyPriority:  1,
		})
		require.NoError(t, err)

		newType := "relative"
		isPrimary := true
		isEmergencyContact := true
		canPickup := true
		pickupNotes := "Nur mit Ausweis"
		emergencyPriority := 3

		err = service.UpdateStudentGuardianRelationship(ctx, created.ID, users.StudentGuardianUpdateRequest{
			RelationshipType:   &newType,
			IsPrimary:          &isPrimary,
			IsEmergencyContact: &isEmergencyContact,
			CanPickup:          &canPickup,
			PickupNotes:        &pickupNotes,
			EmergencyPriority:  &emergencyPriority,
		})
		require.NoError(t, err)

		updated, err := service.GetStudentGuardianRelationship(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, newType, updated.RelationshipType)
		assert.True(t, updated.IsPrimary)
		assert.True(t, updated.IsEmergencyContact)
		assert.True(t, updated.CanPickup)
		require.NotNil(t, updated.PickupNotes)
		assert.Equal(t, pickupNotes, *updated.PickupNotes)
		assert.Equal(t, emergencyPriority, updated.EmergencyPriority)
	})

	// The payer mark is owned by SetStudentPayer (guardians:financial, audited).
	// A relationship edit holding a stale copy of the row must not carry it
	// back into the table — that would silently undo a payer assignment.
	t.Run("leaves the payer mark untouched", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "rel-update-payer")
		student := testpkg.CreateTestStudent(t, db, "RelUpdatePayer", "Student", "5c")

		created, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			EmergencyPriority: 1,
		})
		require.NoError(t, err)
		require.False(t, created.IsPayer)

		// The payer is assigned AFTER the caller read the relationship, so the
		// in-memory copy still says false — exactly the concurrent shape.
		require.NoError(t, service.SetStudentPayer(ctx, student.ID, &guardian.ID, 1))

		canPickup := true
		require.NoError(t, service.UpdateStudentGuardianRelationship(ctx, created.ID,
			users.StudentGuardianUpdateRequest{CanPickup: &canPickup}))

		updated, err := service.GetStudentGuardianRelationship(ctx, created.ID)
		require.NoError(t, err)
		assert.True(t, updated.CanPickup, "the requested field must still be written")
		assert.True(t, updated.IsPayer, "an unrelated relationship edit must not clear the payer mark")
	})

	t.Run("returns error when relationship is missing", func(t *testing.T) {
		missingID := time.Now().UnixNano()
		isPrimary := true

		err := service.UpdateStudentGuardianRelationship(ctx, missingID, users.StudentGuardianUpdateRequest{
			IsPrimary: &isPrimary,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "relationship not found")
	})
}

// =============================================================================
// New Student Guardian Batch Tests
// =============================================================================

func validNewStudentGuardian(email string) users.NewStudentGuardian {
	return users.NewStudentGuardian{
		Profile: users.GuardianCreateRequest{
			FirstName:              "Batch",
			LastName:               "Guardian",
			Email:                  &email,
			PreferredContactMethod: "email",
			LanguagePreference:     "de",
		},
		Relationship: users.StudentGuardianRelationship{
			RelationshipType:  "parent",
			EmergencyPriority: 1,
		},
	}
}

func TestGuardianService_ValidateNewGuardians(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("accepts existing profile link request", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "validate-existing")

		req := validNewStudentGuardian("")
		req.ExistingProfileID = &profile.ID
		req.Relationship.RelationshipType = "guardian"

		err := service.ValidateNewGuardians(ctx, []users.NewStudentGuardian{req})
		require.NoError(t, err)
	})

	t.Run("rejects duplicate emails in one request", func(t *testing.T) {
		email := fmt.Sprintf("dup-batch-%d@example.com", time.Now().UnixNano())
		first := validNewStudentGuardian(email)
		second := validNewStudentGuardian("  " + email + "  ")

		err := service.ValidateNewGuardians(ctx, []users.NewStudentGuardian{first, second})
		require.Error(t, err)
		var validationErr *users.ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Contains(t, err.Error(), "mehrfach angegeben")
	})

	t.Run("rejects email already owned by a profile", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "validate-owned")

		req := validNewStudentGuardian(*profile.Email)

		err := service.ValidateNewGuardians(ctx, []users.NewStudentGuardian{req})
		require.Error(t, err)
		var validationErr *users.ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Contains(t, err.Error(), "bereits vergeben")
	})

	t.Run("rejects invalid existing profile relationship", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "validate-bad-rel")

		req := validNewStudentGuardian("")
		req.ExistingProfileID = &profile.ID
		req.Relationship.RelationshipType = "invalid"

		err := service.ValidateNewGuardians(ctx, []users.NewStudentGuardian{req})
		require.Error(t, err)
		var validationErr *users.ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Contains(t, err.Error(), "ungültiger Beziehungstyp")
	})

	t.Run("rejects missing existing profile", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "validate-missing-existing")

		missingID := profile.ID + time.Now().UnixNano()
		req := validNewStudentGuardian("")
		req.ExistingProfileID = &missingID

		err := service.ValidateNewGuardians(ctx, []users.NewStudentGuardian{req})
		require.Error(t, err)
		var validationErr *users.ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Contains(t, err.Error(), "ausgewählte Person nicht gefunden")
	})

	t.Run("rejects invalid new relationship", func(t *testing.T) {
		email := fmt.Sprintf("bad-rel-%d@example.com", time.Now().UnixNano())
		req := validNewStudentGuardian(email)
		req.Relationship.RelationshipType = "friend"

		err := service.ValidateNewGuardians(ctx, []users.NewStudentGuardian{req})
		require.Error(t, err)
		var validationErr *users.ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Contains(t, err.Error(), "ungültiger Beziehungstyp")
	})

	t.Run("rejects invalid emergency priority", func(t *testing.T) {
		email := fmt.Sprintf("bad-priority-%d@example.com", time.Now().UnixNano())
		req := validNewStudentGuardian(email)
		req.Relationship.EmergencyPriority = 0

		err := service.ValidateNewGuardians(ctx, []users.NewStudentGuardian{req})
		require.Error(t, err)
		var validationErr *users.ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Contains(t, err.Error(), "Notfall-Priorität")
	})

	t.Run("rejects invalid profile fields", func(t *testing.T) {
		email := "not-an-email"
		req := validNewStudentGuardian(email)

		err := service.ValidateNewGuardians(ctx, []users.NewStudentGuardian{req})
		require.Error(t, err)
		var validationErr *users.ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Contains(t, err.Error(), "ungültiges E-Mail-Format")
	})

	t.Run("rejects invalid phone number", func(t *testing.T) {
		email := fmt.Sprintf("bad-phone-%d@example.com", time.Now().UnixNano())
		req := validNewStudentGuardian(email)
		req.PhoneNumbers = []users.PhoneNumberCreateRequest{{
			PhoneNumber: "12",
			PhoneType:   "mobile",
		}}

		err := service.ValidateNewGuardians(ctx, []users.NewStudentGuardian{req})
		require.Error(t, err)
		var validationErr *users.ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Contains(t, err.Error(), "Telefonnummer")
	})
}

func TestGuardianService_AddGuardiansToStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates new guardian links student and adds phone", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "BatchAdd", "Student", "7a")

		email := fmt.Sprintf("batch-add-%d@example.com", time.Now().UnixNano())
		req := validNewStudentGuardian(email)
		req.PhoneNumbers = []users.PhoneNumberCreateRequest{{
			PhoneNumber: "+49 221 123456",
			PhoneType:   "mobile",
			IsPrimary:   true,
		}}

		err := service.AddGuardiansToStudent(ctx, student.ID, []users.NewStudentGuardian{req})
		require.NoError(t, err)

		guardians, err := service.GetStudentGuardians(ctx, student.ID)
		require.NoError(t, err)
		require.Len(t, guardians, 1)
		assert.Equal(t, "Batch", guardians[0].Profile.FirstName)
		assert.Equal(t, email, *guardians[0].Profile.Email)
	})

	t.Run("links existing profile once when selected twice", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "ExistingBatch", "Student", "7b")
		profile := testpkg.CreateTestGuardianProfile(t, db, "existing-batch")

		first := validNewStudentGuardian("")
		first.ExistingProfileID = &profile.ID
		second := validNewStudentGuardian("")
		second.ExistingProfileID = &profile.ID
		second.Relationship.IsEmergencyContact = true

		err := service.AddGuardiansToStudent(ctx, student.ID, []users.NewStudentGuardian{first, second})
		require.NoError(t, err)

		guardians, err := service.GetStudentGuardians(ctx, student.ID)
		require.NoError(t, err)
		require.Len(t, guardians, 1)
		assert.Equal(t, profile.ID, guardians[0].Profile.ID)
		assert.False(t, guardians[0].Relationship.IsEmergencyContact,
			"duplicate existing-profile links are skipped without updating the first relationship")
	})

	t.Run("returns validation error before writing invalid guardian", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "InvalidBatch", "Student", "7c")

		req := validNewStudentGuardian(fmt.Sprintf("batch-invalid-%d@example.com", time.Now().UnixNano()))
		req.Relationship.RelationshipType = "friend"

		err := service.AddGuardiansToStudent(ctx, student.ID, []users.NewStudentGuardian{req})

		require.Error(t, err)
		var validationErr *users.ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Contains(t, err.Error(), "ungültiger Beziehungstyp")
	})

	t.Run("returns link error for missing student", func(t *testing.T) {
		missingStudentID := time.Now().UnixNano()
		email := fmt.Sprintf("batch-link-missing-%d@example.com", missingStudentID)
		req := validNewStudentGuardian(email)

		err := service.AddGuardiansToStudent(ctx, missingStudentID, []users.NewStudentGuardian{req})
		t.Cleanup(func() {
			_, _ = db.NewDelete().TableExpr("users.guardian_profiles").
				Where("email = ?", email).Exec(context.Background())
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to link guardian at index 0")
	})
}

func TestGuardianService_SearchGuardiansForPicker(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns matches with linked children", func(t *testing.T) {
		token := fmt.Sprintf("picker-%d", time.Now().UnixNano())
		guardian := testpkg.CreateTestGuardianProfile(t, db, token)
		student := testpkg.CreateTestStudent(t, db, "Picker", "Child", "8a")

		_, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:          student.ID,
			GuardianProfileID:  guardian.ID,
			RelationshipType:   "parent",
			EmergencyPriority:  1,
			IsEmergencyContact: true,
		})
		require.NoError(t, err)

		matches, err := service.SearchGuardiansForPicker(ctx, token, 10)
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, guardian.ID, matches[0].Profile.ID)
		require.Len(t, matches[0].Children, 1)
		assert.Equal(t, student.ID, matches[0].Children[0].StudentID)
	})

	t.Run("returns matches without linked children", func(t *testing.T) {
		token := fmt.Sprintf("picker-unlinked-%d", time.Now().UnixNano())
		guardian := testpkg.CreateTestGuardianProfile(t, db, token)

		matches, err := service.SearchGuardiansForPicker(ctx, token, 10)
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, guardian.ID, matches[0].Profile.ID)
		assert.Empty(t, matches[0].Children)
	})

	t.Run("returns empty slice for no matches", func(t *testing.T) {
		matches, err := service.SearchGuardiansForPicker(ctx, fmt.Sprintf("missing-%d", time.Now().UnixNano()), 10)
		require.NoError(t, err)
		assert.NotNil(t, matches)
		assert.Empty(t, matches)
	})

	t.Run("returns search repository error", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		matches, err := service.SearchGuardiansForPicker(canceledCtx, "picker", 10)

		require.Error(t, err)
		assert.Nil(t, matches)
	})
}

// =============================================================================
// RemoveGuardianFromStudent Tests
// =============================================================================

func TestGuardianService_RemoveGuardianFromStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("removes guardian from student", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "to-remove")
		student := testpkg.CreateTestStudent(t, db, "RemoveGuardian", "Student", "6a")

		// Create relationship
		req := users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
		}
		_, err := service.LinkGuardianToStudent(ctx, req)
		require.NoError(t, err)
		require.NoError(t, service.SetStudentPayer(ctx, student.ID, &guardian.ID, 1))

		// ACT
		err = service.RemoveGuardianFromStudent(ctx, student.ID, guardian.ID, 1, true)

		// ASSERT
		require.NoError(t, err)

		// Verify removal
		guardians, err := service.GetStudentGuardians(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, guardians)

		count, err := db.NewSelect().
			Model((*auditModels.GuardianFinancialChange)(nil)).
			ModelTableExpr(`audit.guardian_financial_changes AS "guardian_financial_change"`).
			Where(`"guardian_financial_change".guardian_profile_id = ?`, guardian.ID).
			Where(`"guardian_financial_change".student_id = ?`, student.ID).
			Where(`"guardian_financial_change".field_name = ?`, auditModels.GuardianPaymentFieldIsPayer).
			Where(`"guardian_financial_change".new_value = ?`, "false").
			Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "removing a payer relationship must be audited")
	})

	t.Run("refuses to unlink the payer without the financial permission", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "payer-keep")
		student := testpkg.CreateTestStudent(t, db, "KeepPayer", "Student", "6c")
		_, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
		})
		require.NoError(t, err)
		require.NoError(t, service.SetStudentPayer(ctx, student.ID, &guardian.ID, 1))

		// ACT
		err = service.RemoveGuardianFromStudent(ctx, student.ID, guardian.ID, 1, false)

		// ASSERT
		require.ErrorIs(t, err, users.ErrPayerRemovalRequiresFinancial)
		guardians, err := service.GetStudentGuardians(ctx, student.ID)
		require.NoError(t, err)
		require.Len(t, guardians, 1, "the relationship must survive the refusal")
		assert.True(t, guardians[0].Relationship.IsPayer, "the payer mark must survive the refusal")

		count, err := db.NewSelect().
			Model((*auditModels.GuardianFinancialChange)(nil)).
			ModelTableExpr(`audit.guardian_financial_changes AS "guardian_financial_change"`).
			Where(`"guardian_financial_change".guardian_profile_id = ?`, guardian.ID).
			Where(`"guardian_financial_change".student_id = ?`, student.ID).
			Where(`"guardian_financial_change".field_name = ?`, auditModels.GuardianPaymentFieldIsPayer).
			Where(`"guardian_financial_change".new_value = ?`, "false").
			Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "a refused unlink must not leave a payer-removed audit row")
	})

	t.Run("unlinks a guardian who is not the payer without the financial permission", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "not-payer")
		student := testpkg.CreateTestStudent(t, db, "NotPayer", "Student", "6d")
		_, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
		})
		require.NoError(t, err)

		// ACT
		err = service.RemoveGuardianFromStudent(ctx, student.ID, guardian.ID, 1, false)

		// ASSERT
		require.NoError(t, err)
		guardians, err := service.GetStudentGuardians(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, guardians)
	})

	t.Run("returns error when relationship not found", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "NoRel", "Student", "6b")

		// ACT
		err := service.RemoveGuardianFromStudent(ctx, student.ID, 99999999, 1, true)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// ListGuardians Tests
// =============================================================================

func TestGuardianService_ListGuardians(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns list of guardians", func(t *testing.T) {
		// ARRANGE
		testpkg.CreateTestGuardianProfile(t, db, "list-1")
		testpkg.CreateTestGuardianProfile(t, db, "list-2")

		// ACT
		result, err := service.ListGuardians(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}

// =============================================================================
// GetGuardiansWithoutAccount Tests
// =============================================================================

func TestGuardianService_GetGuardiansWithoutAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns guardians without accounts", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "no-account")

		// ACT
		result, err := service.GetGuardiansWithoutAccount(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		// Verify our guardian is in the list
		found := false
		for _, g := range result {
			if g.ID == guardian.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Created guardian should be in list")
	})
}

// =============================================================================
// GetInvitableGuardians Tests
// =============================================================================

func TestGuardianService_GetInvitableGuardians(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns invitable guardians", func(t *testing.T) {
		// ARRANGE - create guardian with email (invitable)
		testpkg.CreateTestGuardianProfile(t, db, "invitable")

		// ACT
		result, err := service.GetInvitableGuardians(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		// Our guardian should be invitable (has email, no account)
	})
}

// =============================================================================
// GetPendingInvitations Tests
// =============================================================================

func TestGuardianService_GetPendingInvitations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.Ctx(t)

	t.Run("returns pending invitations after creating one", func(t *testing.T) {
		// ARRANGE - create a pending invitation
		guardianEmail := fmt.Sprintf("pending-test-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Pending",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Pending", "Teacher")

		_, _, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
		require.NoError(t, err)

		// ACT - get pending invitations
		result, err := service.GetPendingInvitations(ctx)

		// ASSERT
		require.NoError(t, err, "GetPendingInvitations should not return error")
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result), 1, "should have at least one pending invitation")
	})

	t.Run("returns empty or nil when no pending invitations", func(t *testing.T) {
		// This test just verifies no error is returned
		// Result can be nil or empty slice - both are valid
		result, err := service.GetPendingInvitations(ctx)

		require.NoError(t, err, "GetPendingInvitations should not return error")
		// nil or empty slice are both acceptable when no invitations exist
		if result != nil {
			t.Logf("Found %d pending invitations", len(result))
		}
	})
}

// =============================================================================
// CleanupExpiredInvitations Tests
// =============================================================================

func TestGuardianService_CleanupExpiredInvitations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("cleans up expired invitations", func(t *testing.T) {
		// ACT
		count, err := service.CleanupExpiredInvitations(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})
}

// =============================================================================
// Invitation Email Tests (with capturing mailer)
// =============================================================================

// setupGuardianServiceWithMailer creates a GuardianService with injected mailer for testing email flows
func setupGuardianServiceWithMailer(db *bun.DB, mailer *testpkg.CapturingMailer) *users.GuardianService {
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))

	// Create dispatcher from the capturing mailer
	dispatcher := email.NewDispatcher(mailer, slog.Default())
	// Use fast retry settings for tests
	dispatcher.SetDefaults(1, []time.Duration{10 * time.Millisecond})

	deps := users.GuardianServiceDependencies{
		GuardianProfileRepo:     repoFactory.GuardianProfile,
		GuardianPhoneNumberRepo: repoFactory.GuardianPhoneNumber,
		StudentGuardianRepo:     repoFactory.StudentGuardian,
		GuardianInvitationRepo:  repoFactory.GuardianInvitation,
		AccountRepo:             repoFactory.Account,
		AccountParentRepo:       repoFactory.AccountParent,
		AccountTenantRepo:       repoFactory.AccountTenant,
		AccountRoleRepo:         repoFactory.AccountRole,
		RoleRepo:                repoFactory.Role,
		StudentRepo:             repoFactory.Student,
		PersonRepo:              repoFactory.Person,
		Mailer:                  mailer,
		Dispatcher:              dispatcher,
		FrontendURL:             "http://localhost:3000",
		DefaultFrom:             email.NewEmail("Test", "test@example.com"),
		InvitationExpiry:        48 * time.Hour,
		DB:                      db,
	}

	return users.NewGuardianService(deps)
}

func TestGuardianService_SendInvitation_SendsEmail(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.Ctx(t)

	t.Run("sends invitation email to guardian", func(t *testing.T) {
		// ARRANGE - create guardian with email
		guardianEmail := fmt.Sprintf("invite-test-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Invite",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
			LanguagePreference:     "de",
		}
		guardian, err := service.CreateGuardian(ctx, req)
		require.NoError(t, err)

		// Create a teacher to be the inviter
		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Inviter", "Teacher")

		// ACT - send invitation
		invitation, err := service.SendInvitation(ctx, users.GuardianInvitationRequest{
			GuardianProfileID: guardian.ID,
			CreatedBy:         *teacher.Staff.Person.AccountID,
		})

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, invitation)
		assert.NotEmpty(t, invitation.Token)

		// Wait for async email dispatch
		emailSent := mailer.WaitForMessages(1, 500*time.Millisecond)
		assert.True(t, emailSent, "Expected invitation email to be sent")

		if emailSent {
			msgs := mailer.Messages()
			assert.Equal(t, "Einladung zum Eltern-Portal", msgs[0].Subject)
			assert.Equal(t, guardianEmail, msgs[0].To.Address)
		}
	})
}

func TestGuardianService_SendInvitation_GuardianNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent guardian", func(t *testing.T) {
		// ACT
		invitation, err := service.SendInvitation(ctx, users.GuardianInvitationRequest{
			GuardianProfileID: 99999999,
			CreatedBy:         1,
		})

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, invitation)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGuardianService_SendInvitation_NoEmail(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when guardian has no email", func(t *testing.T) {
		// ARRANGE - create guardian without email (phone numbers are added separately)
		req := users.GuardianCreateRequest{
			FirstName:              "NoEmail",
			LastName:               "Guardian",
			PreferredContactMethod: "phone",
		}
		guardian, err := service.CreateGuardian(ctx, req)
		require.NoError(t, err)

		// ACT
		invitation, err := service.SendInvitation(ctx, users.GuardianInvitationRequest{
			GuardianProfileID: guardian.ID,
			CreatedBy:         1,
		})

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, invitation)
		assert.Contains(t, err.Error(), "cannot be invited")
	})
}

func TestGuardianService_SendInvitation_DuplicatePending(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when guardian has pending invitation", func(t *testing.T) {
		// ARRANGE - create guardian
		guardianEmail := fmt.Sprintf("duplicate-test-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Duplicate",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}
		guardian, err := service.CreateGuardian(ctx, req)
		require.NoError(t, err)

		// Create first invitation
		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "First", "Inviter")

		_, err = service.SendInvitation(ctx, users.GuardianInvitationRequest{
			GuardianProfileID: guardian.ID,
			CreatedBy:         *teacher.Staff.Person.AccountID,
		})
		require.NoError(t, err)

		// ACT - try to send another invitation
		invitation, err := service.SendInvitation(ctx, users.GuardianInvitationRequest{
			GuardianProfileID: guardian.ID,
			CreatedBy:         *teacher.Staff.Person.AccountID,
		})

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, invitation)
		assert.Contains(t, err.Error(), "pending invitation")
	})
}

// =============================================================================
// CreateGuardianWithInvitation Tests
// =============================================================================

func TestGuardianService_CreateGuardianWithInvitation_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.Ctx(t)

	t.Run("creates guardian and sends invitation in one transaction", func(t *testing.T) {
		// ARRANGE
		guardianEmail := fmt.Sprintf("combined-test-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Combined",
			LastName:               "Test",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
			LanguagePreference:     "de",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Creator", "Teacher")

		// ACT
		profile, invitation, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
		defer func() {
		}()

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, profile)
		require.NotNil(t, invitation)
		assert.Equal(t, "Combined", profile.FirstName)
		assert.Equal(t, guardianEmail, *profile.Email)
		assert.NotEmpty(t, invitation.Token)
		assert.Equal(t, profile.ID, invitation.GuardianProfileID)

		// Verify email was sent
		emailSent := mailer.WaitForMessages(1, 500*time.Millisecond)
		assert.True(t, emailSent, "Expected invitation email to be sent")
	})
}

func TestGuardianService_CreateGuardianWithInvitation_NoEmail(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when email not provided", func(t *testing.T) {
		// ARRANGE - no email
		req := users.GuardianCreateRequest{
			FirstName: "NoEmail",
			LastName:  "Guardian",
		}

		// ACT
		profile, invitation, err := service.CreateGuardianWithInvitation(ctx, req, 1)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, profile)
		assert.Nil(t, invitation)
		assert.Contains(t, err.Error(), "email is required")
	})
}

func TestGuardianService_CreateGuardianWithInvitation_ExistingAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	mailer := testpkg.NewCapturingMailer()
	service := setupGuardianServiceWithMailer(db, mailer)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when guardian already has account", func(t *testing.T) {
		// ARRANGE - create guardian, send invitation, accept it first
		guardianEmail := fmt.Sprintf("existing-account-%d@example.com", time.Now().UnixNano())
		req := users.GuardianCreateRequest{
			FirstName:              "Existing",
			LastName:               "Account",
			Email:                  &guardianEmail,
			PreferredContactMethod: "email",
		}

		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Teacher", "One")

		// Create first guardian with invitation
		profile, _, err := service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)
		require.NoError(t, err)

		// Mark the profile as having an account (what accepting the live
		// guardian invitation flow does)
		account := testpkg.CreateTestAccount(t, db, guardianEmail)
		_, err = db.NewUpdate().
			ModelTableExpr(`users.guardian_profiles`).
			Set("account_id = ?", account.ID).
			Set("has_account = TRUE").
			Where("id = ?", profile.ID).
			Exec(ctx)
		require.NoError(t, err)

		// ACT - try to create another guardian with same email
		_, _, err = service.CreateGuardianWithInvitation(ctx, req, *teacher.Staff.Person.AccountID)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already has an account")
	})
}

// =============================================================================
// AddPhoneNumber Tests
// =============================================================================

func TestGuardianService_AddPhoneNumber_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("adds first phone number as primary by default", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "first-phone")

		req := users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 123 456789",
			PhoneType:   "mobile",
		}

		// ACT
		result, err := service.AddPhoneNumber(ctx, guardian.ID, req)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "+49 123 456789", result.PhoneNumber)
		assert.Equal(t, usermodels.PhoneTypeMobile, result.PhoneType)
		assert.True(t, result.IsPrimary, "First phone should be primary")
		assert.Equal(t, 1, result.Priority)
	})

	t.Run("adds second phone number as non-primary", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "second-phone")

		// Add first phone
		_, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 111 111111",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		// Add second phone
		req := users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 222 222222",
			PhoneType:   "work",
		}

		// ACT
		result, err := service.AddPhoneNumber(ctx, guardian.ID, req)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "+49 222 222222", result.PhoneNumber)
		assert.False(t, result.IsPrimary, "Second phone should not be primary")
		assert.Equal(t, 2, result.Priority)
	})

	t.Run("adds phone with explicit primary flag", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "explicit-primary")

		// Add first phone
		_, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 111 111111",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		// Add second phone as primary
		label := "Hauptnummer"
		req := users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 333 333333",
			PhoneType:   "home",
			Label:       &label,
			IsPrimary:   true,
		}

		// ACT
		result, err := service.AddPhoneNumber(ctx, guardian.ID, req)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, result.IsPrimary)
		assert.Equal(t, &label, result.Label)

		// Verify first phone is no longer primary
		phones, err := service.GetGuardianPhoneNumbers(ctx, guardian.ID)
		require.NoError(t, err)
		for _, phone := range phones {
			if phone.PhoneNumber == "+49 111 111111" {
				assert.False(t, phone.IsPrimary, "Old primary should be unset")
			}
		}
	})

	t.Run("defaults to mobile phone type for invalid type", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "invalid-type")

		req := users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 444 444444",
			PhoneType:   "invalid_type",
		}

		// ACT
		result, err := service.AddPhoneNumber(ctx, guardian.ID, req)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, usermodels.PhoneTypeMobile, result.PhoneType, "Should default to mobile")
	})
}

func TestGuardianService_AddPhoneNumber_Errors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when guardian not found", func(t *testing.T) {
		// ARRANGE
		req := users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 555 555555",
			PhoneType:   "mobile",
		}

		// ACT
		result, err := service.AddPhoneNumber(ctx, 99999999, req)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})
}

// =============================================================================
// UpdatePhoneNumber Tests
// =============================================================================

func TestGuardianService_UpdatePhoneNumber_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates phone number fields", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "update-fields")

		phone, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 666 666666",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		newNumber := "+49 777 777777"
		newType := "work"
		newLabel := "Büro"
		req := users.PhoneNumberUpdateRequest{
			PhoneNumber: &newNumber,
			PhoneType:   &newType,
			Label:       &newLabel,
		}

		// ACT
		err = service.UpdatePhoneNumber(ctx, phone.ID, req)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		updated, err := service.GetPhoneNumberByID(ctx, phone.ID)
		require.NoError(t, err)
		assert.Equal(t, newNumber, updated.PhoneNumber)
		assert.Equal(t, usermodels.PhoneTypeWork, updated.PhoneType)
		assert.Equal(t, &newLabel, updated.Label)
	})

	t.Run("sets phone as primary and unsets others", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "set-primary")

		// Create two phones
		phone1, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 111 111111",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		phone2, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 222 222222",
			PhoneType:   "work",
		})
		require.NoError(t, err)

		// Make phone2 primary
		isPrimary := true
		req := users.PhoneNumberUpdateRequest{
			IsPrimary: &isPrimary,
		}

		// ACT
		err = service.UpdatePhoneNumber(ctx, phone2.ID, req)

		// ASSERT
		require.NoError(t, err)

		// Verify phone2 is primary
		updated2, err := service.GetPhoneNumberByID(ctx, phone2.ID)
		require.NoError(t, err)
		assert.True(t, updated2.IsPrimary)

		// Verify phone1 is not primary
		updated1, err := service.GetPhoneNumberByID(ctx, phone1.ID)
		require.NoError(t, err)
		assert.False(t, updated1.IsPrimary, "Old primary should be unset")
	})

	t.Run("unsets primary flag", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "unset-primary")

		phone, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 888 888888",
			PhoneType:   "mobile",
			IsPrimary:   true,
		})
		require.NoError(t, err)

		isPrimary := false
		req := users.PhoneNumberUpdateRequest{
			IsPrimary: &isPrimary,
		}

		// ACT
		err = service.UpdatePhoneNumber(ctx, phone.ID, req)

		// ASSERT
		require.NoError(t, err)

		updated, err := service.GetPhoneNumberByID(ctx, phone.ID)
		require.NoError(t, err)
		assert.False(t, updated.IsPrimary)
	})

	t.Run("updates priority", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "update-priority")

		phone, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 999 999999",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		newPriority := 5
		req := users.PhoneNumberUpdateRequest{
			Priority: &newPriority,
		}

		// ACT
		err = service.UpdatePhoneNumber(ctx, phone.ID, req)

		// ASSERT
		require.NoError(t, err)

		updated, err := service.GetPhoneNumberByID(ctx, phone.ID)
		require.NoError(t, err)
		assert.Equal(t, 5, updated.Priority)
	})
}

func TestGuardianService_UpdatePhoneNumber_Errors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when phone not found", func(t *testing.T) {
		// ARRANGE
		newNumber := "+49 000 000000"
		req := users.PhoneNumberUpdateRequest{
			PhoneNumber: &newNumber,
		}

		// ACT
		err := service.UpdatePhoneNumber(ctx, 99999999, req)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// =============================================================================
// DeletePhoneNumber Tests
// =============================================================================

func TestGuardianService_DeletePhoneNumber_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes non-primary phone", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "delete-non-primary")

		phone1, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 111 111111",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		phone2, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 222 222222",
			PhoneType:   "work",
		})
		require.NoError(t, err)

		// ACT - delete non-primary phone
		err = service.DeletePhoneNumber(ctx, phone2.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify phone2 is deleted
		_, err = service.GetPhoneNumberByID(ctx, phone2.ID)
		assert.Error(t, err, "Deleted phone should not be found")

		// Verify phone1 still exists and is still primary
		phone1After, err := service.GetPhoneNumberByID(ctx, phone1.ID)
		require.NoError(t, err)
		assert.True(t, phone1After.IsPrimary)
	})

	t.Run("deletes primary phone and promotes next", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "delete-primary")

		phone1, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 333 333333",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		phone2, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 444 444444",
			PhoneType:   "work",
		})
		require.NoError(t, err)

		// Verify phone1 is primary
		assert.True(t, phone1.IsPrimary)

		// ACT - delete primary phone
		err = service.DeletePhoneNumber(ctx, phone1.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify phone1 is deleted
		_, err = service.GetPhoneNumberByID(ctx, phone1.ID)
		assert.Error(t, err)

		// Verify phone2 was promoted to primary
		phone2After, err := service.GetPhoneNumberByID(ctx, phone2.ID)
		require.NoError(t, err)
		assert.True(t, phone2After.IsPrimary, "Next phone should be promoted to primary")
	})

	t.Run("deletes last phone number", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "delete-last")

		phone, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 555 555555",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		// ACT
		err = service.DeletePhoneNumber(ctx, phone.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify guardian has no phones
		phones, err := service.GetGuardianPhoneNumbers(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Empty(t, phones)
	})
}

func TestGuardianService_DeletePhoneNumber_Errors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when phone not found", func(t *testing.T) {
		// ACT
		err := service.DeletePhoneNumber(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// =============================================================================
// SetPrimaryPhone Tests
// =============================================================================

func TestGuardianService_SetPrimaryPhone_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("sets phone as primary", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "set-primary")

		phone1, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 111 111111",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		phone2, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 222 222222",
			PhoneType:   "work",
		})
		require.NoError(t, err)

		// ACT - set phone2 as primary
		err = service.SetPrimaryPhone(ctx, phone2.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify phone2 is now primary
		phone2After, err := service.GetPhoneNumberByID(ctx, phone2.ID)
		require.NoError(t, err)
		assert.True(t, phone2After.IsPrimary)

		// Verify phone1 is no longer primary
		phone1After, err := service.GetPhoneNumberByID(ctx, phone1.ID)
		require.NoError(t, err)
		assert.False(t, phone1After.IsPrimary)
	})
}

func TestGuardianService_SetPrimaryPhone_Errors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when phone not found", func(t *testing.T) {
		// ACT
		err := service.SetPrimaryPhone(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// =============================================================================
// GetGuardianPhoneNumbers Tests
// =============================================================================

func TestGuardianService_GetGuardianPhoneNumbers_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns all phone numbers sorted by priority", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "get-phones")

		// Add three phones
		_, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 111 111111",
			PhoneType:   "mobile",
		})
		require.NoError(t, err)

		_, err = service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 222 222222",
			PhoneType:   "work",
		})
		require.NoError(t, err)

		_, err = service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 333 333333",
			PhoneType:   "home",
		})
		require.NoError(t, err)

		// ACT
		phones, err := service.GetGuardianPhoneNumbers(ctx, guardian.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, phones, 3)

		// Verify they're sorted by priority
		for i := 0; i < len(phones)-1; i++ {
			assert.LessOrEqual(t, phones[i].Priority, phones[i+1].Priority,
				"Phones should be sorted by priority")
		}

		// Verify first phone is primary
		assert.True(t, phones[0].IsPrimary, "First phone should be primary")
	})

	t.Run("returns empty list when guardian has no phones", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "no-phones")

		// ACT
		phones, err := service.GetGuardianPhoneNumbers(ctx, guardian.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, phones)
	})
}

// =============================================================================
// GetPhoneNumberByID Tests
// =============================================================================

func TestGuardianService_GetPhoneNumberByID_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns phone number by ID", func(t *testing.T) {
		// ARRANGE
		guardian := testpkg.CreateTestGuardianProfile(t, db, "get-by-id")

		label := "Hauptnummer"
		phone, err := service.AddPhoneNumber(ctx, guardian.ID, users.PhoneNumberCreateRequest{
			PhoneNumber: "+49 444 444444",
			PhoneType:   "mobile",
			Label:       &label,
		})
		require.NoError(t, err)

		// ACT
		result, err := service.GetPhoneNumberByID(ctx, phone.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, phone.ID, result.ID)
		assert.Equal(t, "+49 444 444444", result.PhoneNumber)
		assert.Equal(t, usermodels.PhoneTypeMobile, result.PhoneType)
		assert.Equal(t, &label, result.Label)
	})
}

func TestGuardianService_GetPhoneNumberByID_Errors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when phone not found", func(t *testing.T) {
		// ACT
		result, err := service.GetPhoneNumberByID(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestGetStudentGuardians_NonOpenInvitationsNotPending(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	inviter := testpkg.CreateTestAccount(t, db, "inv-states")

	cases := []struct {
		name   string
		mutate func(*authModels.GuardianInvitation)
	}{
		{"accepted", func(i *authModels.GuardianInvitation) { now := time.Now(); i.AcceptedAt = &now }},
		{"expired", func(i *authModels.GuardianInvitation) { i.ExpiresAt = time.Now().Add(-time.Hour) }},
		{"rejected", func(i *authModels.GuardianInvitation) {
			i.ApprovalStatus = authModels.GuardianInvitationApprovalRejected
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			guardian := testpkg.CreateTestGuardianProfile(t, db, "inv-"+c.name)
			student := testpkg.CreateTestStudent(t, db, "Inv", "State", "1a")
			defer func() {
				_, _ = db.NewDelete().TableExpr("users.students_guardians").Where("student_id = ?", student.ID).Exec(ctx)
				_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", guardian.ID).Exec(ctx)
			}()

			_, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
				StudentID: student.ID, GuardianProfileID: guardian.ID, RelationshipType: "parent",
			})
			require.NoError(t, err)

			inv := &authModels.GuardianInvitation{
				Token:             fmt.Sprintf("state-%s-%d", c.name, time.Now().UnixNano()),
				GuardianProfileID: guardian.ID,
				CreatedBy:         inviter.ID,
				ExpiresAt:         time.Now().Add(48 * time.Hour),
				ApprovalStatus:    authModels.GuardianInvitationApprovalNotRequired,
			}
			c.mutate(inv)
			inv.SetTenantID(testpkg.Tenant(t))
			require.NoError(t, repoFactory.GuardianInvitation.Create(ctx, inv))

			res, err := service.GetStudentGuardians(ctx, student.ID)
			require.NoError(t, err)
			require.Len(t, res, 1)
			assert.False(t, res[0].InvitationPending, c.name+" invitation must not count as pending")
		})
	}
}

// TestGuardianService_ContactWritersShareProfileLock is the serialization guard
// for review #1743 critical 4: every staff-side guardian contact writer
// (UpdateGuardian plus the phone mutators) now locks the owning
// guardian_profiles row FOR UPDATE — the SAME row the parents-portal contact
// path locks before its read-modify-write and wholesale phone replace. Holding
// that lock from a separate, uncommitted transaction must BLOCK each staff
// writer, proven deterministically with a short lock_timeout (no goroutine
// timing — mirrors the students_guardians FOR UPDATE test). Drop any of the
// LockByIDForUpdate calls and that writer races straight through, so a staff
// edit could clobber or lose a concurrent parent contact save.
func TestGuardianService_ContactWritersShareProfileLock(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	email := fmt.Sprintf("lock-guardian-%d@example.com", time.Now().UnixNano())
	profile := testpkg.CreateTestGuardianProfile(t, db, email)
	defer func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("users.guardian_phone_numbers").Where("guardian_profile_id = ?", profile.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(bg)
	}()

	// Seed one phone so the update/delete writers have a live target.
	seedPhone, err := service.AddPhoneNumber(ctx, profile.ID, users.PhoneNumberCreateRequest{
		PhoneNumber: "0151 1234567",
		PhoneType:   "mobile",
	})
	require.NoError(t, err)

	// Each writer is exercised in order; UpdatePhoneNumber and SetPrimaryPhone run
	// before DeletePhoneNumber so the seeded phone still exists for all three.
	writers := []struct {
		name string
		run  func(runCtx context.Context) error
	}{
		{"UpdateGuardian", func(runCtx context.Context) error {
			return service.UpdateGuardian(runCtx, profile.ID, users.GuardianCreateRequest{
				FirstName: "Locked", LastName: "Name",
			})
		}},
		{"AddPhoneNumber", func(runCtx context.Context) error {
			_, e := service.AddPhoneNumber(runCtx, profile.ID, users.PhoneNumberCreateRequest{
				PhoneNumber: "0151 7654321", PhoneType: "mobile",
			})
			return e
		}},
		{"UpdatePhoneNumber", func(runCtx context.Context) error {
			label := "Aktualisiert"
			return service.UpdatePhoneNumber(runCtx, seedPhone.ID, users.PhoneNumberUpdateRequest{Label: &label})
		}},
		{"SetPrimaryPhone", func(runCtx context.Context) error {
			return service.SetPrimaryPhone(runCtx, seedPhone.ID)
		}},
		{"DeletePhoneNumber", func(runCtx context.Context) error {
			return service.DeletePhoneNumber(runCtx, seedPhone.ID)
		}},
	}

	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	for _, w := range writers {
		t.Run(w.name+" blocks on a held profile lock", func(t *testing.T) {
			// Hold the profile FOR UPDATE lock from an uncommitted tx.
			holdTx, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = holdTx.Rollback() }()
			holdCtx := tenant.WithTransactionForTest(ctx, &holdTx)
			require.NoError(t, repoFactory.GuardianProfile.LockByIDForUpdate(holdCtx, profile.ID))

			// The staff writer, on a separate tx with a short lock_timeout, must
			// fail to acquire the same row lock.
			staffTx, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = staffTx.Rollback() }()
			_, err = staffTx.ExecContext(ctx, "SET LOCAL lock_timeout = ?", "250ms")
			require.NoError(t, err)
			staffCtx := tenant.WithTransactionForTest(ctx, &staffTx)

			runErr := w.run(staffCtx)
			require.Errorf(t, runErr, "%s must block on the held profile FOR UPDATE lock", w.name)
			assert.Truef(t, isProfileLockTimeout(runErr), "%s: expected lock_timeout, got: %v", w.name, runErr)
			_ = staffTx.Rollback()

			// Release the holder; the writer must now succeed on a fresh tx.
			require.NoError(t, holdTx.Rollback())
			require.NoErrorf(t, w.run(ctx), "%s must succeed once the lock holder releases", w.name)
		})
	}
}

// TestGetStudentGuardians_AccountHolderPendingUpgradeApproval verifies the
// #2172 staff-status fix: a guardian WITH a portal account counts as pending
// only for an open invitation anchored to this child (a pending-approval
// role-upgrade request) — never for a sibling's invite.
func TestGetStudentGuardians_AccountHolderPendingUpgradeApproval(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))

	guardian := testpkg.CreateTestGuardianProfile(t, db, "acct-pending")
	student := testpkg.CreateTestStudent(t, db, "AcctPending", "Student", "1a")
	sibling := testpkg.CreateTestStudent(t, db, "AcctPending", "Sibling", "1a")
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "AcctPending", "Account")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.guardian_invitations").Where("guardian_profile_id = ?", guardian.ID).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("users.students_guardians").Where("guardian_profile_id = ?", guardian.ID).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", guardian.ID).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("users.students").Where("id IN (?, ?)", student.ID, sibling.ID).Exec(context.Background())
	}()

	require.NoError(t, repoFactory.GuardianProfile.LinkAccount(ctx, guardian.ID, account.ID))
	_, err := service.LinkGuardianToStudent(ctx, users.StudentGuardianCreateRequest{
		StudentID:         student.ID,
		GuardianProfileID: guardian.ID,
		RelationshipType:  "other",
		GuardianRole:      authorize.GuardianRoleEmergency,
	})
	require.NoError(t, err)

	// Open pending-approval upgrade request anchored to the SIBLING → this
	// child's row must not read as pending.
	siblingID := sibling.ID
	inv := &authModels.GuardianInvitation{
		Token:             fmt.Sprintf("acct-pending-%d", time.Now().UnixNano()),
		GuardianProfileID: guardian.ID,
		CreatedBy:         account.ID,
		ExpiresAt:         time.Now().Add(48 * time.Hour),
		StudentID:         &siblingID,
		ApprovalStatus:    authModels.GuardianInvitationApprovalPending,
		RoleUpgrade:       true,
	}
	inv.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repoFactory.GuardianInvitation.Create(ctx, inv))

	res, err := service.GetStudentGuardians(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.False(t, res[0].InvitationPending,
		"sibling-anchored invite must not mark this child pending")

	// Re-anchored to THIS child → pending.
	studentID := student.ID
	inv.StudentID = &studentID
	require.NoError(t, repoFactory.GuardianInvitation.Update(ctx, inv))

	res, err = service.GetStudentGuardians(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.True(t, res[0].InvitationPending,
		"account holder with a pending upgrade approval for this child must read as pending")
}

// TestGuardianService_UpdateGuardianPayment_MaskedAuditCollisions pins that a
// bank-detail change whose masked representation is unchanged (a new IBAN with
// the same last four digits, a renamed account holder behind the fixed mask)
// still persists and still leaves a distinguishable, plaintext-free audit row
// (#2608).
func TestGuardianService_UpdateGuardianPayment_MaskedAuditCollisions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)

	loadAudits := func(t *testing.T, profileID int64, field string) []auditModels.GuardianFinancialChange {
		t.Helper()
		var rows []auditModels.GuardianFinancialChange
		require.NoError(t, db.NewSelect().
			Model(&rows).
			ModelTableExpr(`audit.guardian_financial_changes AS "guardian_financial_change"`).
			Where(`"guardian_financial_change".guardian_profile_id = ?`, profileID).
			Where(`"guardian_financial_change".field_name = ?`, field).
			OrderExpr(`"guardian_financial_change".id ASC`).
			Scan(ctx))
		return rows
	}

	t.Run("new IBAN with the same last four digits", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "same-tail")

		require.NoError(t, service.UpdateGuardianPayment(ctx, profile.ID, users.GuardianPaymentInput{
			IBAN: strPtr("DE89370400440532013000"),
		}, 1, ""))

		// ACT: valid IBAN, same "3000" tail, different account.
		err := service.UpdateGuardianPayment(ctx, profile.ID, users.GuardianPaymentInput{
			IBAN: strPtr("DE84120300000000203000"),
		}, 1, "")

		// ASSERT
		require.NoError(t, err)
		stored, err := service.GuardianFinancialRepo.FindByGuardianProfileID(ctx, profile.ID)
		require.NoError(t, err)
		require.NotNil(t, stored)
		require.NotNil(t, stored.IBAN)
		assert.Equal(t, "DE84120300000000203000", *stored.IBAN)

		audits := loadAudits(t, profile.ID, auditModels.GuardianPaymentFieldIBAN)
		require.Len(t, audits, 2)
		assert.Equal(t, "•••• 3000", audits[1].OldValue)
		assert.Equal(t, "•••• 3000 (geändert)", audits[1].NewValue)
		assert.NotContains(t, audits[1].NewValue, "DE84")
	})

	t.Run("renamed account holder behind the fixed mask", func(t *testing.T) {
		profile := testpkg.CreateTestGuardianProfile(t, db, "holder-rename")

		require.NoError(t, service.UpdateGuardianPayment(ctx, profile.ID, users.GuardianPaymentInput{
			IBAN:          strPtr("DE89370400440532013000"),
			AccountHolder: strPtr("Sabine Schneider"),
		}, 1, ""))

		// ACT
		err := service.UpdateGuardianPayment(ctx, profile.ID, users.GuardianPaymentInput{
			IBAN:          strPtr("DE89370400440532013000"),
			AccountHolder: strPtr("Peter Schneider"),
		}, 1, "")

		// ASSERT
		require.NoError(t, err)
		stored, err := service.GuardianFinancialRepo.FindByGuardianProfileID(ctx, profile.ID)
		require.NoError(t, err)
		require.NotNil(t, stored)
		require.NotNil(t, stored.AccountHolder)
		assert.Equal(t, "Peter Schneider", *stored.AccountHolder)

		audits := loadAudits(t, profile.ID, auditModels.GuardianPaymentFieldAccountHolder)
		require.Len(t, audits, 2)
		assert.Equal(t, "••••••••", audits[1].OldValue)
		assert.Equal(t, "•••••••• (geändert)", audits[1].NewValue)
		for _, row := range audits {
			assert.NotContains(t, row.OldValue, "Schneider")
			assert.NotContains(t, row.NewValue, "Schneider")
		}
		// The unchanged IBAN produced no extra row.
		assert.Len(t, loadAudits(t, profile.ID, auditModels.GuardianPaymentFieldIBAN), 1)
	})
}
