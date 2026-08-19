package parent_test

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// buildGuardianService wires a parent service with the guardian feature enabled
// (the registry default). Use buildGuardianServiceFeature to exercise the gate.
func buildGuardianService(t *testing.T) (parentService.Service, *bun.DB) {
	return buildGuardianServiceFeature(t, true)
}

// buildGuardianServiceFeature wires a parent service with the repos the guardian
// contact/pickup methods need, with the management feature toggle set explicitly.
func buildGuardianServiceFeature(t *testing.T, mgmtEnabled bool) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:     repos.ParentChild,
		StatusDayRepo: repos.StudentStatusDay,
		StudentRepo:   repos.Student,
		Settings: parentSettingsStub{
			boolValues: map[string]bool{configModels.KeyParentGuardianManagementEnabled: mgmtEnabled},
		},
		GuardianInvites:         &stubInvites{},
		GuardianInviteRepo:      repos.GuardianInvitation,
		StudentGuardianRepo:     repos.StudentGuardian,
		GuardianProfileRepo:     repos.GuardianProfile,
		GuardianPhoneRepo:       repos.GuardianPhoneNumber,
		GuardianChangeAuditRepo: repos.GuardianChange,
		DB:                      db,
		Logger:                  slog.Default(),
	})
	return svc, db
}

// linkContactOnlyGuardian creates a contact-only guardian profile (no portal
// account) linked to the student and returns its profile id. The caller must
// clean up via the returned cleanup func.
func linkContactOnlyGuardian(t *testing.T, db *bun.DB, studentID int64, emailSeed string) (int64, func()) {
	t.Helper()
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	profile := testpkg.CreateTestGuardianProfile(t, db, emailSeed)
	link := &userModels.StudentGuardian{
		StudentID:         studentID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "relative",
		EmergencyPriority: 2,
	}
	link.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.StudentGuardian.Create(ctx, link))
	cleanup := func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("users.guardian_phone_numbers").Where("guardian_profile_id = ?", profile.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("users.students_guardians").Where("guardian_profile_id = ?", profile.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(bg)
	}
	return profile.ID, cleanup
}

func TestListChildGuardians_ReturnsDetailAndCapabilities(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-list")
	defer cleanup()

	guardians, err := svc.ListChildGuardians(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, guardians, 2)

	byID := map[int64]*parentService.ChildGuardian{}
	for _, g := range guardians {
		byID[g.GuardianProfileID] = g
	}

	self := byID[chain.GuardianProfileID]
	require.NotNil(t, self)
	assert.True(t, self.IsSelf)
	assert.True(t, self.HasAccount)
	assert.True(t, self.CanEditContact, "own profile is editable")
	// Pickup authority is never self-managed through the portal: the caller holds
	// pickup.manage but is an account holder, so they may not toggle their OWN
	// can_pickup / emergency flags (closes self-grant in a custody dispute).
	assert.False(t, self.CanManagePickup, "account holder cannot manage own pickup flags")

	contact := byID[contactID]
	require.NotNil(t, contact)
	assert.False(t, contact.HasAccount)
	assert.True(t, contact.CanEditContact, "contact-only guardian is editable by full guardian")
	assert.True(t, contact.CanManagePickup, "pickup flags of a non-account helper are manageable")
}

func TestCreateGuardianContact_AddsAccountlessPickupContact(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	email := "oma.neu@example.test"
	created, err := svc.CreateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, parentService.CreateGuardianContactInput{
		Contact: parentService.GuardianContactInput{
			FirstName: "Helga",
			LastName:  "Schneider",
			Email:     &email,
			Phones: []parentService.GuardianPhoneInput{
				{PhoneNumber: "0151 12345678", PhoneType: "mobile", IsPrimary: true},
			},
		},
		RelationshipType:   "relative",
		CanPickup:          true,
		IsEmergencyContact: true,
		PickupNotes:        ptr("Bitte Ausweis mitbringen"),
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	defer func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("audit.guardian_changes").Where("guardian_profile_id = ?", created.GuardianProfileID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("users.guardian_phone_numbers").Where("guardian_profile_id = ?", created.GuardianProfileID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("users.students_guardians").Where("guardian_profile_id = ?", created.GuardianProfileID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", created.GuardianProfileID).Exec(bg)
	}()

	assert.False(t, created.HasAccount)
	assert.True(t, created.CanPickup)
	assert.True(t, created.IsEmergencyContact)
	assert.Equal(t, "Bitte Ausweis mitbringen", created.PickupNotes)
	require.Len(t, created.Phones, 1)

	var role string
	err = db.NewSelect().TableExpr("users.students_guardians").ColumnExpr("guardian_role").
		Where("student_id = ? AND guardian_profile_id = ?", chain.StudentID, created.GuardianProfileID).
		Scan(context.Background(), &role)
	require.NoError(t, err)
	assert.Equal(t, authorize.GuardianRolePickupOnly, role)
}

func TestCreateGuardianContact_RequiresPickupPermissionForFlags(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	editorID, cleanup := linkAccountGuardian(t, db, chain.StudentID, "contact-creator", map[string]interface{}{
		authorize.GuardianPermissionPortalAccess: true,
		authorize.GuardianPermissionGuardianEdit: true,
	})
	defer cleanup()

	_, err := svc.CreateGuardianContact(context.Background(), accountIDForProfile(t, db, editorID), chain.StudentID, parentService.CreateGuardianContactInput{
		Contact: parentService.GuardianContactInput{
			FirstName: "Helga",
			LastName:  "Schneider",
		},
		RelationshipType: "relative",
		CanPickup:        true,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)
}

func TestUpdateGuardianContact_EditsContactOnlyGuardian(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-edit")
	defer cleanup()

	email := "neue.oma@example.com"
	updated, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianContactInput{
		FirstName:     "Helga",
		LastName:      "Schneider",
		Email:         &email,
		AddressStreet: ptr("Hauptstr. 1"),
		Phones: []parentService.GuardianPhoneInput{
			{PhoneNumber: "0151 12345678", PhoneType: "mobile", IsPrimary: true},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Helga", updated.FirstName)
	assert.Equal(t, email, updated.Email)
	require.Len(t, updated.Phones, 1)
	assert.Equal(t, "0151 12345678", updated.Phones[0].PhoneNumber)

	// Re-read to confirm persistence.
	reread, err := svc.ListChildGuardians(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	var found *parentService.ChildGuardian
	for _, g := range reread {
		if g.GuardianProfileID == contactID {
			found = g
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, "Helga", found.FirstName)
	require.Len(t, found.Phones, 1)
}

func TestUpdateGuardianContact_PromotesFirstPhoneWhenNoPrimarySubmitted(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-no-primary")
	defer cleanup()

	updated, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianContactInput{
		FirstName: "Helga",
		LastName:  "Schneider",
		Phones: []parentService.GuardianPhoneInput{
			{PhoneNumber: "0151 12345678", PhoneType: "mobile"},
			{PhoneNumber: "0221 555123", PhoneType: "home"},
		},
	})
	require.NoError(t, err)
	require.Len(t, updated.Phones, 2)
	assert.True(t, updated.Phones[0].IsPrimary)
	assert.Equal(t, "0151 12345678", updated.Phones[0].PhoneNumber)

	assertExactlyOnePrimaryPhone(t, db, contactID)
}

func TestUpdateGuardianContact_KeepsOnlyFirstSubmittedPrimaryPhone(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-many-primary")
	defer cleanup()

	updated, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianContactInput{
		FirstName: "Helga",
		LastName:  "Schneider",
		Phones: []parentService.GuardianPhoneInput{
			{PhoneNumber: "0151 12345678", PhoneType: "mobile", IsPrimary: true},
			{PhoneNumber: "0221 555123", PhoneType: "home", IsPrimary: true},
			{PhoneNumber: "0221 555124", PhoneType: "work", IsPrimary: true},
		},
	})
	require.NoError(t, err)
	require.Len(t, updated.Phones, 3)
	assert.True(t, updated.Phones[0].IsPrimary)
	assert.Equal(t, "0151 12345678", updated.Phones[0].PhoneNumber)
	assert.False(t, updated.Phones[1].IsPrimary)
	assert.False(t, updated.Phones[2].IsPrimary)

	assertExactlyOnePrimaryPhone(t, db, contactID)
}

func TestUpdateGuardianContact_RejectsInvalidPhoneBeforeRepository(t *testing.T) {
	t.Parallel()

	svc, _ := buildGuardianService(t)

	_, err := svc.UpdateGuardianContact(context.Background(), 1, 1, 1, parentService.GuardianContactInput{
		FirstName: "Helga",
		LastName:  "Schneider",
		Phones: []parentService.GuardianPhoneInput{
			{PhoneNumber: "bitte anrufen", PhoneType: "mobile", IsPrimary: true},
		},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, parentService.ErrGuardianContactInvalid)
}

func TestUpdateGuardianContact_MapsDuplicateEmailToConflict(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	firstID, firstCleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-dupe-a")
	defer firstCleanup()
	secondID, secondCleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-dupe-b")
	defer secondCleanup()

	email := guardianEmailForProfile(t, db, firstID)
	_, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, secondID, parentService.GuardianContactInput{
		FirstName: "Duplicate",
		LastName:  "Email",
		Email:     &email,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, parentService.ErrGuardianEmailConflict)
}

// TestUpdateGuardianContact_RejectsCaseInsensitiveDuplicateEmail pins the
// contract that a mixed-case variant of an email another guardian already owns
// is rejected as a conflict (never silently accepted as a case-variant
// duplicate). Guardian email matching is case-insensitive (LOWER(email)) across
// invite/account flows, so two rows differing only in case would break it. The
// service enforces this with a FindByEmail (LOWER=LOWER) precheck mirroring the
// staff-side guardian service; under a case-sensitive collation that precheck is
// the only guard, so it must stay.
func TestUpdateGuardianContact_RejectsCaseInsensitiveDuplicateEmail(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	firstID, firstCleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-case-a")
	defer firstCleanup()
	secondID, secondCleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-case-b")
	defer secondCleanup()

	// Submit an UPPER-CASED variant of the first guardian's (lowercase) email for
	// the second. Byte-distinct from the stored value, so the case-sensitive index
	// would accept it; the LOWER(email) precheck must still reject it as a conflict.
	mixedCase := strings.ToUpper(guardianEmailForProfile(t, db, firstID))
	_, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, secondID, parentService.GuardianContactInput{
		FirstName: "Mixed",
		LastName:  "Case",
		Email:     &mixedCase,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, parentService.ErrGuardianEmailConflict)
}

func TestUpdateGuardianContact_RejectsAccountHolder(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// Another guardian WITH their own portal account, linked to the same child.
	otherID, cleanup := linkAccountGuardian(t, db, chain.StudentID, "other-parent", map[string]interface{}{
		authorize.GuardianPermissionPortalAccess: true,
	})
	defer cleanup()

	_, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, otherID, parentService.GuardianContactInput{
		FirstName: "Hacked",
		LastName:  "Name",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, parentService.ErrGuardianHasOwnAccount)

	// The listing reflects the lock so the UI can explain why the account
	// holder is read-only — but only to this caller, who DOES hold edit rights.
	guardians, err := svc.ListChildGuardians(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	var other *parentService.ChildGuardian
	for _, g := range guardians {
		if g.GuardianProfileID == otherID {
			other = g
		}
	}
	require.NotNil(t, other)
	assert.False(t, other.CanEditContact, "account holder is not editable by another parent")
	assert.True(t, other.ContactLockedOwnAccount, "lock reason is surfaced to a caller with edit rights")
}

func TestUpdateGuardianContact_RejectsCrossFamilySharedProfile(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-shared")
	defer cleanup()

	// The SAME contact profile also serves a child of another family — a student
	// the caller is not a guardian of. Editing the profile would propagate to that
	// other family, so the edit must be refused.
	other := testpkg.CreateTestStudent(t, db, "Fremd", "Kind", "3c")
	defer func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("users.students_guardians").Where("student_id = ?", other.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("users.students").Where("id = ?", other.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("users.persons").Where("id = ?", other.PersonID).Exec(bg)
	}()
	otherLink := &userModels.StudentGuardian{
		StudentID:         other.ID,
		GuardianProfileID: contactID,
		RelationshipType:  "relative",
		EmergencyPriority: 2,
	}
	otherLink.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(db).StudentGuardian.Create(testpkg.Ctx(t), otherLink))

	_, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianContactInput{
		FirstName: "Helga",
		LastName:  "Schneider",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, parentService.ErrGuardianSharedAcrossFamilies)

	// The listing marks the shared contact as locked (not editable) for this
	// caller, so the UI can explain why the edit affordance is absent.
	guardians, err := svc.ListChildGuardians(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	var shared *parentService.ChildGuardian
	for _, g := range guardians {
		if g.GuardianProfileID == contactID {
			shared = g
		}
	}
	require.NotNil(t, shared)
	assert.False(t, shared.CanEditContact, "shared contact is not editable by this caller")
	assert.True(t, shared.ContactLockedShared, "shared lock reason is surfaced")
}

func TestUpdateGuardianRelationship_PickupManageGate(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-pickup")
	defer cleanup()

	// The chain's primary guardian HAS pickup.manage → may set can_pickup.
	canPickup := true
	updated, err := svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianRelationshipInput{
		CanPickup:   &canPickup,
		PickupNotes: ptr("Kommt mit dem Fahrrad"),
	})
	require.NoError(t, err)
	assert.True(t, updated.CanPickup)
	assert.Equal(t, "Kommt mit dem Fahrrad", updated.PickupNotes)

	// A caller WITH guardian.edit but WITHOUT pickup.manage may NOT flip the flag.
	editorID, editorCleanup := linkAccountGuardian(t, db, chain.StudentID, "editor-parent", map[string]interface{}{
		authorize.GuardianPermissionPortalAccess: true,
		authorize.GuardianPermissionGuardianEdit: true,
	})
	defer editorCleanup()
	editorAccountID := accountIDForProfile(t, db, editorID)

	_, err = svc.UpdateGuardianRelationship(context.Background(), editorAccountID, chain.StudentID, contactID, parentService.GuardianRelationshipInput{
		CanPickup: &canPickup,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)

	// ...but the same caller MAY edit the low-stakes pickup note.
	_, err = svc.UpdateGuardianRelationship(context.Background(), editorAccountID, chain.StudentID, contactID, parentService.GuardianRelationshipInput{
		PickupNotes: ptr("Wird abgeholt"),
	})
	require.NoError(t, err)
}

// TestUpdateGuardianRelationship_NoteOnlyEditLeavesFlagsUntouched pins the
// write-set contract behind the concurrency fix: a relationship edit writes ONLY
// the columns the request actually supplied. A note-only edit must therefore not
// re-write can_pickup / is_emergency_contact, so it can never revert a flag value
// the request did not touch (e.g. a concurrent staff toggle of those flags). The
// true cross-actor lost-update race needs real concurrency a sequential test
// can't stage; this guards the observable half — the flags a note-only edit
// leaves behind are exactly the stored ones — and would fail on a regression that
// writes the flag columns unconditionally (e.g. with a zero-value default).
func TestUpdateGuardianRelationship_NoteOnlyEditLeavesFlagsUntouched(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-noteonly")
	defer cleanup()

	// The chain primary (pickup.manage) raises both safety flags on the helper.
	canPickup := true
	emergency := true
	_, err := svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianRelationshipInput{
		CanPickup:          &canPickup,
		IsEmergencyContact: &emergency,
	})
	require.NoError(t, err)

	// A guardian.edit-only caller (no pickup.manage) edits ONLY the note.
	editorID, editorCleanup := linkAccountGuardian(t, db, chain.StudentID, "note-editor", map[string]interface{}{
		authorize.GuardianPermissionPortalAccess: true,
		authorize.GuardianPermissionGuardianEdit: true,
	})
	defer editorCleanup()
	editorAccountID := accountIDForProfile(t, db, editorID)

	updated, err := svc.UpdateGuardianRelationship(context.Background(), editorAccountID, chain.StudentID, contactID, parentService.GuardianRelationshipInput{
		PickupNotes: ptr("Wird von der Oma abgeholt"),
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Wird von der Oma abgeholt", updated.PickupNotes)

	// The note edit must not have written the safety flags back.
	gotPickup, gotEmergency := relationshipFlags(t, db, chain.StudentID, contactID)
	assert.True(t, gotPickup, "note-only edit must not clear can_pickup")
	assert.True(t, gotEmergency, "note-only edit must not clear is_emergency_contact")
}

func TestUpdateGuardianRelationship_PickupManageWithoutEditFlipsFlags(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-manage-only")
	defer cleanup()

	// A guardian holding pickup.manage but NOT guardian.edit. ListChildGuardians
	// advertises can_manage_pickup for them, so the write must honor it instead of
	// 403-ing on a missing guardian.edit (the bug this fix closes).
	managerID, managerCleanup := linkAccountGuardian(t, db, chain.StudentID, "manager-parent", map[string]interface{}{
		authorize.GuardianPermissionPortalAccess: true,
		authorize.GuardianPermissionPickupManage: true,
	})
	defer managerCleanup()
	managerAccountID := accountIDForProfile(t, db, managerID)

	// The advertised capability and the write must agree: the manager can manage
	// pickup for the non-account helper, even without guardian.edit. (Their own
	// flags are not self-manageable - they hold an account.)
	guardians, err := svc.ListChildGuardians(context.Background(), managerAccountID, chain.StudentID)
	require.NoError(t, err)
	var helper, self *parentService.ChildGuardian
	for _, g := range guardians {
		switch g.GuardianProfileID {
		case contactID:
			helper = g
		case managerID:
			self = g
		}
	}
	require.NotNil(t, helper)
	require.NotNil(t, self)
	assert.True(t, helper.CanManagePickup, "pickup.manage can manage a non-account helper")
	assert.False(t, helper.CanEditContact, "no guardian.edit -> contact not editable")
	assert.False(t, self.CanManagePickup, "account holder cannot manage own pickup flags")

	canPickup := true
	updated, err := svc.UpdateGuardianRelationship(context.Background(), managerAccountID, chain.StudentID, contactID, parentService.GuardianRelationshipInput{
		CanPickup: &canPickup,
	})
	require.NoError(t, err)
	assert.True(t, updated.CanPickup)

	// ...but without guardian.edit the same caller may NOT touch the pickup note.
	_, err = svc.UpdateGuardianRelationship(context.Background(), managerAccountID, chain.StudentID, contactID, parentService.GuardianRelationshipInput{
		PickupNotes: ptr("darf ich nicht"),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)
}

func TestUpdateGuardianRelationship_RejectsFlagsOnAccountHolders(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// Another guardian WITH their own portal account, linked to the same child.
	otherID, cleanup := linkAccountGuardian(t, db, chain.StudentID, "other-parent", map[string]interface{}{
		authorize.GuardianPermissionPortalAccess: true,
	})
	defer cleanup()

	// can_pickup / is_emergency_contact are safety-critical authority. They may
	// only be set for guardians WITHOUT their own portal account. The chain
	// primary holds pickup.manage but may neither change another account holder's
	// standing nor grant it to themselves through the portal (custody-dispute
	// griefing). Contact DATA editing is separately blocked; see
	// TestUpdateGuardianContact_RejectsAccountHolder.
	canPickup := true
	emergency := true

	// (a) cannot change another account holder's pickup/emergency flags.
	_, err := svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, otherID, parentService.GuardianRelationshipInput{
		CanPickup: &canPickup,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, parentService.ErrGuardianHasOwnAccount)

	_, err = svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, otherID, parentService.GuardianRelationshipInput{
		IsEmergencyContact: &emergency,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, parentService.ErrGuardianHasOwnAccount)

	// (b) cannot grant pickup authority to themselves (caller is an account holder).
	_, err = svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, chain.GuardianProfileID, parentService.GuardianRelationshipInput{
		CanPickup: &canPickup,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, parentService.ErrGuardianHasOwnAccount)
}

func TestUpdateGuardianRelationship_WritesPickupAuditRow(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-audit")
	defer cleanup()
	// Clean audit rows explicitly (belt-and-suspenders; the student/guardian FKs
	// also cascade, and actor_account_id is ON DELETE SET NULL).
	defer func() {
		_, _ = db.NewDelete().TableExpr("audit.guardian_changes").Where("student_id = ?", chain.StudentID).Exec(context.Background())
	}()

	canPickup := true
	_, err := svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianRelationshipInput{
		CanPickup: &canPickup,
	})
	require.NoError(t, err)

	rows, err := repositories.NewFactory(db).GuardianChange.ListByStudentID(context.Background(), chain.StudentID)
	require.NoError(t, err)
	require.Len(t, rows, 1, "exactly one audit row for the single flag change")
	assert.Equal(t, "pickup", rows[0].ChangeType)
	assert.Equal(t, "can_pickup", rows[0].FieldName)
	require.NotNil(t, rows[0].OldValue)
	assert.Equal(t, "false", *rows[0].OldValue)
	require.NotNil(t, rows[0].NewValue)
	assert.Equal(t, "true", *rows[0].NewValue)
	require.NotNil(t, rows[0].ActorAccountID)
	assert.Equal(t, chain.AccountID, *rows[0].ActorAccountID)
	assert.Equal(t, contactID, rows[0].GuardianProfileID)

	// A no-op write (same value) must not append a second row.
	_, err = svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianRelationshipInput{
		CanPickup: &canPickup,
	})
	require.NoError(t, err)
	rows, err = repositories.NewFactory(db).GuardianChange.ListByStudentID(context.Background(), chain.StudentID)
	require.NoError(t, err)
	require.Len(t, rows, 1, "unchanged value must not write a new audit row")
}

func TestUpdateGuardianContact_WritesContactAuditRows(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-contact-audit")
	defer cleanup()
	defer func() {
		_, _ = db.NewDelete().TableExpr("audit.guardian_changes").Where("student_id = ?", chain.StudentID).Exec(context.Background())
	}()

	email := "neue.oma@example.com"
	_, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianContactInput{
		FirstName: "Helga",
		LastName:  "Schneider",
		Email:     &email,
		Phones: []parentService.GuardianPhoneInput{
			{PhoneNumber: "0151 12345678", PhoneType: "mobile", IsPrimary: true},
		},
	})
	require.NoError(t, err)

	rows, err := repositories.NewFactory(db).GuardianChange.ListByStudentID(context.Background(), chain.StudentID)
	require.NoError(t, err)
	require.NotEmpty(t, rows, "a fresh contact edit must write audit rows")

	byField := make(map[string]*auditModels.GuardianChange, len(rows))
	for _, r := range rows {
		assert.Equal(t, "contact", r.ChangeType)
		assert.Equal(t, contactID, r.GuardianProfileID)
		require.NotNil(t, r.ActorAccountID)
		assert.Equal(t, chain.AccountID, *r.ActorAccountID)
		// Contact field values are third-party PII and are deliberately NOT
		// persisted: the trail records which field changed, not its before/after
		// value (mirrors the pickup-notes decision; the live profile holds the
		// current value).
		assert.Nil(t, r.OldValue, "contact audit rows must not store the old value (PII)")
		assert.Nil(t, r.NewValue, "contact audit rows must not store the new value (PII)")
		byField[r.FieldName] = r
	}
	// The changed fields are recorded by name (email + phones both changed here).
	require.Contains(t, byField, "email")
	require.Contains(t, byField, "phones")

	// A no-op re-save with identical data must not append new rows.
	before := len(rows)
	_, err = svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianContactInput{
		FirstName: "Helga",
		LastName:  "Schneider",
		Email:     &email,
		Phones: []parentService.GuardianPhoneInput{
			{PhoneNumber: "0151 12345678", PhoneType: "mobile", IsPrimary: true},
		},
	})
	require.NoError(t, err)
	rows, err = repositories.NewFactory(db).GuardianChange.ListByStudentID(context.Background(), chain.StudentID)
	require.NoError(t, err)
	assert.Len(t, rows, before, "an unchanged contact re-save must not write new audit rows")
}

func TestGuardianManagement_FeatureDisabled(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianServiceFeature(t, false)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-disabled")
	defer cleanup()

	// Contact edits are refused when the school disabled guardian management.
	_, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianContactInput{
		FirstName: "Helga",
		LastName:  "Schneider",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, parentService.ErrGuardianManagementDisabled)

	// Pickup edits likewise.
	canPickup := true
	_, err = svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianRelationshipInput{
		CanPickup: &canPickup,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, parentService.ErrGuardianManagementDisabled)

	// The listing still returns guardians but suppresses all edit affordances so
	// the UI hides them.
	guardians, err := svc.ListChildGuardians(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.NotEmpty(t, guardians)
	for _, g := range guardians {
		assert.False(t, g.CanEditContact, "contact edit suppressed when feature disabled")
		assert.False(t, g.CanManagePickup, "pickup management suppressed when feature disabled")
	}
}

// linkAccountGuardian creates a guardian profile WITH its own portal account and
// active tenant mapping, linked to the student with the given permission set.
func linkAccountGuardian(t *testing.T, db *bun.DB, studentID int64, seed string, permissions map[string]interface{}) (int64, func()) {
	t.Helper()
	ctx := context.Background()
	account := testpkg.CreateTestAccount(t, db, "parent")
	profile := &userModels.GuardianProfile{
		FirstName:              "Other",
		LastName:               "Guardian",
		Email:                  &account.Email,
		AccountID:              &account.ID,
		HasAccount:             true,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	profile.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(profile).ModelTableExpr(`users.guardian_profiles`).Exec(ctx)
	require.NoError(t, err)

	link := &userModels.StudentGuardian{
		StudentID:         studentID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "parent",
		EmergencyPriority: 3,
		Permissions:       permissions,
	}
	link.SetTenantID(testpkg.Tenant(t))
	_, err = db.NewInsert().Model(link).ModelTableExpr(`users.students_guardians`).Exec(ctx)
	require.NoError(t, err)

	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   account.ID,
		TenantID:    testpkg.Tenant(t),
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	_, err = db.NewInsert().Model(mapping).ModelTableExpr(`auth.account_tenants`).Exec(ctx)
	require.NoError(t, err)

	cleanup := func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("users.students_guardians").Where("guardian_profile_id = ?", profile.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("auth.account_tenants").Where("account_id = ?", account.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(bg)
	}
	return profile.ID, cleanup
}

func accountIDForProfile(t *testing.T, db *bun.DB, profileID int64) int64 {
	t.Helper()
	var accountID int64
	err := db.NewSelect().
		TableExpr("users.guardian_profiles").
		ColumnExpr("account_id").
		Where("id = ?", profileID).
		Scan(context.Background(), &accountID)
	require.NoError(t, err)
	return accountID
}

func guardianEmailForProfile(t *testing.T, db *bun.DB, profileID int64) string {
	t.Helper()
	var email string
	err := db.NewSelect().
		TableExpr("users.guardian_profiles").
		ColumnExpr("email").
		Where("id = ?", profileID).
		Scan(context.Background(), &email)
	require.NoError(t, err)
	return email
}

// relationshipFlags reads the stored can_pickup / is_emergency_contact of a
// student-guardian link straight from the row, bypassing the service, so a test
// can assert what a write actually persisted.
func relationshipFlags(t *testing.T, db *bun.DB, studentID, profileID int64) (canPickup, emergency bool) {
	t.Helper()
	err := db.NewSelect().
		TableExpr("users.students_guardians").
		ColumnExpr("can_pickup, is_emergency_contact").
		Where("student_id = ? AND guardian_profile_id = ?", studentID, profileID).
		Scan(context.Background(), &canPickup, &emergency)
	require.NoError(t, err)
	return canPickup, emergency
}

func assertExactlyOnePrimaryPhone(t *testing.T, db *bun.DB, profileID int64) {
	t.Helper()
	repo := repositories.NewFactory(db).GuardianPhoneNumber
	phones, err := repo.FindByGuardianID(testpkg.Ctx(t), profileID)
	require.NoError(t, err)
	require.NotEmpty(t, phones)
	primaryCount := 0
	for _, phone := range phones {
		if phone.IsPrimary {
			primaryCount++
		}
	}
	assert.Equal(t, 1, primaryCount)
}

func ptr(s string) *string { return &s }

// linkRoleGuardian creates a contact-only guardian (no portal account) with the
// given guardian role, an address and one phone, linked to the student. Used to
// exercise role-based contact redaction. Caller cleans up via the returned func.
func linkRoleGuardian(t *testing.T, db *bun.DB, studentID int64, emailSeed, role string) (int64, func()) {
	t.Helper()
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	profile := testpkg.CreateTestGuardianProfile(t, db, emailSeed)
	profile.AddressStreet = ptr("Amtsstr. 5")
	require.NoError(t, repos.GuardianProfile.Update(ctx, profile))

	phone := &userModels.GuardianPhoneNumber{
		GuardianProfileID: profile.ID,
		PhoneNumber:       "0151 99999999",
		PhoneType:         userModels.PhoneTypeMobile,
		IsPrimary:         true,
		Priority:          1,
	}
	phone.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.GuardianPhoneNumber.Create(ctx, phone))

	link := &userModels.StudentGuardian{
		StudentID:         studentID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "relative",
		GuardianRole:      role,
		EmergencyPriority: 2,
	}
	link.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.StudentGuardian.Create(ctx, link))

	cleanup := func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("users.guardian_phone_numbers").Where("guardian_profile_id = ?", profile.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("users.students_guardians").Where("guardian_profile_id = ?", profile.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(bg)
	}
	return profile.ID, cleanup
}

// TestListChildGuardians_RedactsSocialWorkerContact verifies a social worker's
// personal contact data is hidden from a reading guardian (GDPR Datenminimierung)
// while the name and care arrangement remain visible.
func TestListChildGuardians_RedactsSocialWorkerContact(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	swID, cleanup := linkRoleGuardian(t, db, chain.StudentID, "sozialarbeiter", authorize.GuardianRoleSocialWorker)
	defer cleanup()

	guardians, err := svc.ListChildGuardians(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	var sw *parentService.ChildGuardian
	for _, g := range guardians {
		if g.GuardianProfileID == swID {
			sw = g
		}
	}
	require.NotNil(t, sw)
	assert.Empty(t, sw.Email, "social worker email is redacted")
	assert.Empty(t, sw.AddressStreet, "social worker address is redacted")
	assert.Empty(t, sw.Phones, "social worker phones are redacted")
	assert.NotEmpty(t, sw.LastName, "name remains visible")
	assert.False(t, sw.CanEditContact, "social worker contact is not editable")
	assert.False(t, sw.CanManagePickup, "social worker pickup flags are school-managed")
	assert.True(t, sw.ContactLockedSocialWorker, "lock reason surfaced")
}

// TestUpdateGuardianContact_RejectsSocialWorker verifies a parent cannot rewrite
// a school-managed social worker's contact data.
func TestUpdateGuardianContact_RejectsSocialWorker(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	swID, cleanup := linkRoleGuardian(t, db, chain.StudentID, "sw-edit", authorize.GuardianRoleSocialWorker)
	defer cleanup()

	_, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, swID, parentService.GuardianContactInput{
		FirstName: "Neuer",
		LastName:  "Name",
	})
	require.ErrorIs(t, err, parentService.ErrGuardianSocialWorkerManaged)
}

// TestUpdateGuardianRelationship_RejectsFlagsOnSocialWorker verifies a parent
// cannot toggle a social worker's pickup/emergency flags.
func TestUpdateGuardianRelationship_RejectsFlagsOnSocialWorker(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	swID, cleanup := linkRoleGuardian(t, db, chain.StudentID, "sw-flag", authorize.GuardianRoleSocialWorker)
	defer cleanup()

	canPickup := true
	_, err := svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, swID, parentService.GuardianRelationshipInput{
		CanPickup: &canPickup,
	})
	require.ErrorIs(t, err, parentService.ErrGuardianSocialWorkerManaged)
}

// TestListChildGuardians_LocksFullGuardianWithoutAccount verifies a full guardian
// (legal/co/primary) WITHOUT their own portal account is read-only to another
// parent: contact stays VISIBLE (not redacted, unlike a social worker) but the
// edit/pickup affordances are gone and the lock reason is surfaced (#1667).
func TestListChildGuardians_LocksFullGuardianWithoutAccount(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	legalID, cleanup := linkRoleGuardian(t, db, chain.StudentID, "co-parent", authorize.GuardianRoleLegalGuardian)
	defer cleanup()

	guardians, err := svc.ListChildGuardians(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	var legal *parentService.ChildGuardian
	for _, g := range guardians {
		if g.GuardianProfileID == legalID {
			legal = g
		}
	}
	require.NotNil(t, legal)
	assert.NotEmpty(t, legal.Phones, "full guardian contact stays visible (not redacted)")
	assert.False(t, legal.CanEditContact, "full guardian without account is not editable by another parent")
	assert.False(t, legal.CanManagePickup, "full guardian pickup flags are not parent-manageable")
	assert.True(t, legal.ContactLockedFullGuardian, "full-guardian lock reason surfaced")
	assert.False(t, legal.ContactLockedSocialWorker, "not a social-worker lock")
}

// TestUpdateGuardianContact_RejectsFullGuardian verifies a parent cannot rewrite
// a non-registered full guardian's (legal/co) contact data.
func TestUpdateGuardianContact_RejectsFullGuardian(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	legalID, cleanup := linkRoleGuardian(t, db, chain.StudentID, "co-parent-edit", authorize.GuardianRoleCoGuardian)
	defer cleanup()

	_, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, legalID, parentService.GuardianContactInput{
		FirstName: "Neuer",
		LastName:  "Name",
	})
	require.ErrorIs(t, err, parentService.ErrGuardianRoleManaged)
}

// TestUpdateGuardianRelationship_RejectsFullGuardian verifies a parent cannot
// toggle a non-registered full guardian's pickup/emergency flags.
func TestUpdateGuardianRelationship_RejectsFullGuardian(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	legalID, cleanup := linkRoleGuardian(t, db, chain.StudentID, "co-parent-flag", authorize.GuardianRoleLegalGuardian)
	defer cleanup()

	canPickup := true
	_, err := svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, legalID, parentService.GuardianRelationshipInput{
		CanPickup: &canPickup,
	})
	require.ErrorIs(t, err, parentService.ErrGuardianRoleManaged)
}

// TestUpdateGuardianContact_AllowsHelperRoleWithoutAccount is the regression
// guard for the role tightening: a genuine helper role (pickup_only) without an
// account stays editable — only full guardian roles are newly protected.
func TestUpdateGuardianContact_AllowsHelperRoleWithoutAccount(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	helperID, cleanup := linkRoleGuardian(t, db, chain.StudentID, "helper-pickup", authorize.GuardianRolePickupOnly)
	defer cleanup()

	updated, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, helperID, parentService.GuardianContactInput{
		FirstName: "Helfer",
		LastName:  "Bearbeitet",
	})
	require.NoError(t, err)
	assert.Equal(t, "Helfer", updated.FirstName)
	assert.True(t, updated.CanEditContact, "helper role without account stays editable")
	assert.True(t, updated.CanManagePickup, "helper role pickup flags stay manageable")
	assert.False(t, updated.ContactLockedFullGuardian)
}

// TestUpdateGuardianRelationship_ClearsPickupNote verifies an empty-string note
// clears a previously set pickup note (review #1743 finding 1): the UI sends ""
// for a deleted note, which the service trims to NULL. This is distinct from an
// OMITTED (nil) note, which leaves the stored value untouched — so "clear" must
// be expressible and must actually persist.
func TestUpdateGuardianRelationship_ClearsPickupNote(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-clearnote")
	defer cleanup()

	// Set a note.
	updated, err := svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianRelationshipInput{
		PickupNotes: ptr("Kommt um 15 Uhr"),
	})
	require.NoError(t, err)
	require.Equal(t, "Kommt um 15 Uhr", updated.PickupNotes)

	// Clear it with an empty string — what the UI sends for a deleted note.
	updated, err = svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianRelationshipInput{
		PickupNotes: ptr(""),
	})
	require.NoError(t, err)
	assert.Empty(t, updated.PickupNotes, "empty-string note clears the stored note")

	// Confirm it persisted as NULL in the row.
	var note *string
	err = db.NewSelect().TableExpr("users.students_guardians").ColumnExpr("pickup_notes").
		Where("student_id = ? AND guardian_profile_id = ?", chain.StudentID, contactID).
		Scan(context.Background(), &note)
	require.NoError(t, err)
	assert.Nil(t, note, "cleared note stored as NULL")
}

// TestUpdateGuardianRelationship_RejectsNoteOnContactLockedAccountHolder is the
// regression guard for
// the contact-coupled note rule (review #1743 finding 2, Option B): the per-child
// pickup note follows CONTACT-edit eligibility. A guardian whose contact is
// read-only (here an account holder) is also note-read-only — the listing shows
// can_edit_contact=false, and BOTH a note-only write and a flag write are
// rejected. This closes the UI-vs-service split by tightening the backend instead
// of widening the UI.
func TestUpdateGuardianRelationship_RejectsNoteOnContactLockedAccountHolder(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	// Another guardian WITH their own portal account (contact read-only), linked
	// to the same child with a non-full-guardian relationship.
	otherID, cleanup := linkAccountGuardian(t, db, chain.StudentID, "note-acct-holder", map[string]interface{}{
		authorize.GuardianPermissionPortalAccess: true,
	})
	defer cleanup()

	// The chain primary holds guardian.edit + pickup.manage.
	guardians, err := svc.ListChildGuardians(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	var other *parentService.ChildGuardian
	for _, g := range guardians {
		if g.GuardianProfileID == otherID {
			other = g
		}
	}
	require.NotNil(t, other)
	assert.False(t, other.CanEditContact, "account holder contact is read-only")
	assert.False(t, other.CanManagePickup, "account holder pickup flags are not parent-manageable")

	// A note edit on the contact-locked account holder is rejected (note follows
	// contact-edit eligibility), not silently accepted.
	_, err = svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, otherID, parentService.GuardianRelationshipInput{
		PickupNotes: ptr("Bringt das Kind dienstags"),
	})
	require.ErrorIs(t, err, parentService.ErrGuardianHasOwnAccount)

	// A flag write on the same account holder is rejected for the same reason.
	canPickup := true
	_, err = svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, otherID, parentService.GuardianRelationshipInput{
		CanPickup: &canPickup,
	})
	require.ErrorIs(t, err, parentService.ErrGuardianHasOwnAccount)
}

// TestUpdateGuardianRelationship_RejectsNoteOnFullGuardian verifies the per-child
// note is NOT editable for a full legal guardian (school/self-managed): the
// listing shows can_edit_contact=false and a note write is rejected.
func TestUpdateGuardianRelationship_RejectsNoteOnFullGuardian(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	legalID, cleanup := linkRoleGuardian(t, db, chain.StudentID, "co-parent-note", authorize.GuardianRoleLegalGuardian)
	defer cleanup()

	guardians, err := svc.ListChildGuardians(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	for _, g := range guardians {
		if g.GuardianProfileID == legalID {
			assert.False(t, g.CanEditContact, "full guardian contact is read-only (note follows it)")
		}
	}

	_, err = svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, legalID, parentService.GuardianRelationshipInput{
		PickupNotes: ptr("nicht erlaubt"),
	})
	require.ErrorIs(t, err, parentService.ErrGuardianRoleManaged)
}

// TestUpdateGuardianRelationship_RejectsNoteOnSocialWorker verifies a note edit
// on a school-managed social worker is rejected (note follows contact, which is
// redacted for social workers).
func TestUpdateGuardianRelationship_RejectsNoteOnSocialWorker(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	swID, cleanup := linkRoleGuardian(t, db, chain.StudentID, "sw-note", authorize.GuardianRoleSocialWorker)
	defer cleanup()

	_, err := svc.UpdateGuardianRelationship(context.Background(), chain.AccountID, chain.StudentID, swID, parentService.GuardianRelationshipInput{
		PickupNotes: ptr("nicht erlaubt"),
	})
	require.ErrorIs(t, err, parentService.ErrGuardianSocialWorkerManaged)
}

// TestUpdateGuardianContact_LabelOnlyPhoneEditWritesAuditRow is the regression
// guard for review #1743 finding 5: an accepted phone edit that changes ONLY the
// label (same number/type/primary) must still write a phones audit row. Before
// the fix the audit rendering ignored the label, so a label-only change diffed
// identically and produced no audit trail.
func TestUpdateGuardianContact_LabelOnlyPhoneEditWritesAuditRow(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-label-audit")
	defer cleanup()
	defer func() {
		_, _ = db.NewDelete().TableExpr("audit.guardian_changes").Where("student_id = ?", chain.StudentID).Exec(context.Background())
	}()

	// Initial contact with one label-less phone.
	_, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianContactInput{
		FirstName: "Helga",
		LastName:  "Schneider",
		Phones: []parentService.GuardianPhoneInput{
			{PhoneNumber: "0151 12345678", PhoneType: "mobile", IsPrimary: true},
		},
	})
	require.NoError(t, err)

	// Drop the audit rows from the initial create so we measure only the label edit.
	_, err = db.NewDelete().TableExpr("audit.guardian_changes").Where("student_id = ?", chain.StudentID).Exec(context.Background())
	require.NoError(t, err)

	// Change ONLY the phone label; number, type, and primary are identical.
	_, err = svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianContactInput{
		FirstName: "Helga",
		LastName:  "Schneider",
		Phones: []parentService.GuardianPhoneInput{
			{PhoneNumber: "0151 12345678", PhoneType: "mobile", Label: ptr("Notfall"), IsPrimary: true},
		},
	})
	require.NoError(t, err)

	rows, err := repositories.NewFactory(db).GuardianChange.ListByStudentID(context.Background(), chain.StudentID)
	require.NoError(t, err)
	phoneRows := 0
	for _, r := range rows {
		if r.FieldName == "phones" {
			phoneRows++
		}
	}
	assert.Equal(t, 1, phoneRows, "a label-only phone edit must write a phones audit row")
}

// TestUpdateGuardianContact_CaseVariantEmailRaceLeavesSingleWinner is the
// concurrency regression guard for review #1743 finding 4. Two writers
// concurrently set DIFFERENT guardian profiles to case variants of the SAME
// address (race-N@… and RACE-N@…). Both FindByEmail(LOWER=LOWER) prechecks can
// pass before either commits, so the precheck alone cannot close the race. The
// unique index on (tenant_id, LOWER(email)) (migration 1.15.145) is the atomic
// backstop: exactly one writer commits, the other's UPDATE hits 23505 and is
// mapped to ErrGuardianEmailConflict.
//
// The assertion is interleaving-independent — whoever loses (the precheck path
// if it commits second, the index path if both prechecked first) must get the
// conflict, and the table must never end up with two rows sharing LOWER(email).
// Looping a handful of rounds biases the scheduler toward exercising the index
// branch without making the assertion flaky: every round must hold the same
// invariant regardless of which path fired. Drop the LOWER(email) index and this
// fails — both writers commit and two rows share LOWER(email).
func TestUpdateGuardianContact_CaseVariantEmailRaceLeavesSingleWinner(t *testing.T) {
	t.Parallel()

	svc, db := buildGuardianService(t)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	firstID, firstCleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-race-a")
	defer firstCleanup()
	secondID, secondCleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-race-b")
	defer secondCleanup()

	const rounds = 8
	for i := 0; i < rounds; i++ {
		lower := fmt.Sprintf("race-%d@example.com", i)
		upper := strings.ToUpper(lower)

		// Release both goroutines at the same instant so the two transactions
		// genuinely overlap instead of running back to back.
		var start sync.WaitGroup
		start.Add(1)
		var done sync.WaitGroup
		done.Add(2)
		errs := make([]error, 2) // distinct indices: no shared element, -race clean

		run := func(idx int, profileID int64, email string) {
			defer done.Done()
			start.Wait()
			_, errs[idx] = svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, profileID, parentService.GuardianContactInput{
				FirstName: "Race",
				LastName:  "Winner",
				Email:     &email,
				Phones: []parentService.GuardianPhoneInput{
					{PhoneNumber: fmt.Sprintf("0151 0000%04d", idx), PhoneType: "mobile", IsPrimary: true},
				},
			})
		}
		go run(0, firstID, lower)
		go run(1, secondID, upper)
		start.Done()
		done.Wait()

		// Exactly one writer wins; the loser gets a conflict (precheck OR index).
		successes, conflicts := 0, 0
		for _, err := range errs {
			if err == nil {
				successes++
				continue
			}
			require.ErrorIsf(t, err, parentService.ErrGuardianEmailConflict, "round %d: the loser must be ErrGuardianEmailConflict, got %v", i, err)
			conflicts++
		}
		require.Equalf(t, 1, successes, "round %d: exactly one writer must commit", i)
		require.Equalf(t, 1, conflicts, "round %d: exactly one writer must lose with a conflict", i)

		// The table must never hold two rows sharing LOWER(email) — the invariant
		// the unique LOWER(email) index exists to protect.
		count, cerr := db.NewSelect().
			TableExpr("users.guardian_profiles").
			Where("id IN (?, ?)", firstID, secondID).
			Where("LOWER(email) = ?", lower).
			Count(context.Background())
		require.NoError(t, cerr)
		require.Equalf(t, 1, count, "round %d: exactly one profile may hold LOWER(email)=%s", i, lower)
	}
}

// driftedGuardianInsertErr attempts to persist a guardian profile whose
// account_id and has_account columns are out of sync and returns the resulting
// insert error. Since migration 1.15.146 the database enforces
// has_account = (account_id IS NOT NULL) via the
// chk_guardian_has_account_matches_account_id CHECK constraint, so any such
// insert must be rejected. If a regression ever lets the row persist, it is
// deleted again so the test does not leak a drifted row into later cases.
func driftedGuardianInsertErr(t *testing.T, db *bun.DB, accountID *int64, hasAccount bool, emailSeed string) error {
	t.Helper()
	ctx := context.Background()
	email := emailSeed + "@drift.test"
	profile := &userModels.GuardianProfile{
		FirstName:              "Drifted",
		LastName:               "Helper",
		Email:                  &email,
		AccountID:              accountID,
		HasAccount:             hasAccount,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	profile.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(profile).ModelTableExpr(`users.guardian_profiles`).Exec(ctx)
	if err == nil {
		_, _ = db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(ctx)
	}
	return err
}

// TestGuardianProfile_AccountStateCannotDrift is the regression guard for review
// #1743 critical 2, hardened from an app-level check into a DB invariant
// (migration 1.15.146). has_account and account_id encode the same fact; the two
// could previously drift apart (nothing kept them in sync) and authorization code
// reads one column or the other, so a drifted row was a real privilege-escalation
// risk. The CHECK constraint now makes BOTH inconsistent directions impossible to
// persist, so no authorization path can ever see a disagreeing pair. The
// app-level HasPortalAccount() helper stays as defense-in-depth, but the database
// is the primary guarantee — which is why this test asserts the write is rejected
// rather than simulating a drifted row (it can no longer exist).
func TestGuardianProfile_AccountStateCannotDrift(t *testing.T) {
	t.Parallel()

	_, db := buildGuardianService(t)

	// Direction 1: an account is linked (account_id set) but has_account drifted
	// to false. The FK requires a real account, so create one.
	account := testpkg.CreateTestAccount(t, db, "drift-linked")
	defer func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	}()
	err := driftedGuardianInsertErr(t, db, &account.ID, false, "drift-linked")
	require.Error(t, err, "account_id set with has_account=false must be rejected")
	assert.Contains(t, err.Error(), "SQLSTATE=23514", "expected a CHECK violation")
	assert.Contains(t, err.Error(), "chk_guardian_has_account_matches_account_id")

	// Direction 2: has_account=true with no linked account (account_id NULL).
	err = driftedGuardianInsertErr(t, db, nil, true, "drift-unlinked")
	require.Error(t, err, "has_account=true with NULL account_id must be rejected")
	assert.Contains(t, err.Error(), "SQLSTATE=23514", "expected a CHECK violation")
	assert.Contains(t, err.Error(), "chk_guardian_has_account_matches_account_id")
}

// emailLookupFailingGuardianRepo wraps the real guardian-profile repository and
// fails FindByEmail with a fixed error, delegating everything else. It lets the
// email-lookup-error test inject a DB/RLS-style failure on exactly that call.
type emailLookupFailingGuardianRepo struct {
	userModels.GuardianProfileRepository
	err error
}

func (r emailLookupFailingGuardianRepo) FindByEmail(context.Context, string) (*userModels.GuardianProfile, error) {
	return nil, r.err
}

// TestUpdateGuardianContact_PropagatesEmailLookupError is the regression guard
// for review #1743 critical 3: an UNEXPECTED FindByEmail error (DB/RLS failure,
// not the clean not-found sentinel) must abort the contact edit, not be
// swallowed as "no conflicting profile". Swallowing it would let the write
// commit while a genuine duplicate could exist.
func TestUpdateGuardianContact_PropagatesEmailLookupError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	lookupErr := fmt.Errorf("simulated guardian email lookup failure")
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:     repos.ParentChild,
		StatusDayRepo: repos.StudentStatusDay,
		StudentRepo:   repos.Student,
		Settings: parentSettingsStub{
			boolValues: map[string]bool{configModels.KeyParentGuardianManagementEnabled: true},
		},
		GuardianInvites:         &stubInvites{},
		GuardianInviteRepo:      repos.GuardianInvitation,
		StudentGuardianRepo:     repos.StudentGuardian,
		GuardianProfileRepo:     emailLookupFailingGuardianRepo{GuardianProfileRepository: repos.GuardianProfile, err: lookupErr},
		GuardianPhoneRepo:       repos.GuardianPhoneNumber,
		GuardianChangeAuditRepo: repos.GuardianChange,
		DB:                      db,
		Logger:                  slog.Default(),
	})

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	contactID, cleanup := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-emailerr")
	defer cleanup()

	before := guardianFirstName(t, db, contactID)

	newEmail := "neue-oma@example.test"
	_, err := svc.UpdateGuardianContact(context.Background(), chain.AccountID, chain.StudentID, contactID, parentService.GuardianContactInput{
		FirstName: "Changed",
		LastName:  "Name",
		Email:     &newEmail,
	})
	require.Error(t, err)
	// The raw lookup failure propagates — NOT swallowed, NOT misreported as a
	// duplicate-email conflict.
	assert.ErrorIs(t, err, lookupErr)
	assert.NotErrorIs(t, err, parentService.ErrGuardianEmailConflict)

	// The edit rolled back: the lookup error aborted before the write.
	assert.Equal(t, before, guardianFirstName(t, db, contactID), "a failed email lookup must abort the contact edit, not commit it")
}

func guardianFirstName(t *testing.T, db *bun.DB, profileID int64) string {
	t.Helper()
	var name string
	err := db.NewSelect().
		TableExpr("users.guardian_profiles").
		ColumnExpr("first_name").
		Where("id = ?", profileID).
		Scan(context.Background(), &name)
	require.NoError(t, err)
	return name
}
