package parent_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// buildGuardianService wires a parent service with the repos the guardian
// contact/pickup methods need. Settings are unused by these methods (no feature
// gate), so a zero stub suffices.
func buildGuardianService(t *testing.T) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		NoteRepo:            repos.StudentParentNote,
		Settings:            relAcctSettings{},
		GuardianInvites:     &stubInvites{},
		GuardianInviteRepo:  repos.GuardianInvitation,
		StudentGuardianRepo: repos.StudentGuardian,
		GuardianProfileRepo: repos.GuardianProfile,
		GuardianPhoneRepo:   repos.GuardianPhoneNumber,
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, db
}

// linkContactOnlyGuardian creates a contact-only guardian profile (no portal
// account) linked to the student and returns its profile id. The caller must
// clean up via the returned cleanup func.
func linkContactOnlyGuardian(t *testing.T, db *bun.DB, studentID int64, emailSeed string) (int64, func()) {
	t.Helper()
	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(1)
	profile := testpkg.CreateTestGuardianProfile(t, db, emailSeed)
	link := &userModels.StudentGuardian{
		StudentID:         studentID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "relative",
		EmergencyPriority: 2,
	}
	link.SetTenantID(1)
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
	svc, db := buildGuardianService(t)
	defer func() { _ = db.Close() }()

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
	assert.True(t, self.CanManagePickup, "primary guardian has pickup.manage")
	assert.True(t, self.CanEditContact, "own profile is editable")

	contact := byID[contactID]
	require.NotNil(t, contact)
	assert.False(t, contact.HasAccount)
	assert.True(t, contact.CanEditContact, "contact-only guardian is editable by full guardian")
}

func TestUpdateGuardianContact_EditsContactOnlyGuardian(t *testing.T) {
	svc, db := buildGuardianService(t)
	defer func() { _ = db.Close() }()

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
	svc, db := buildGuardianService(t)
	defer func() { _ = db.Close() }()

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
	svc, db := buildGuardianService(t)
	defer func() { _ = db.Close() }()

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
	svc, db := buildGuardianService(t)
	defer func() { _ = db.Close() }()

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
	svc, db := buildGuardianService(t)
	defer func() { _ = db.Close() }()

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

func TestUpdateGuardianContact_RejectsAccountHolder(t *testing.T) {
	svc, db := buildGuardianService(t)
	defer func() { _ = db.Close() }()

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
	svc, db := buildGuardianService(t)
	defer func() { _ = db.Close() }()

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
	otherLink.SetTenantID(1)
	require.NoError(t, repositories.NewFactory(db).StudentGuardian.Create(testpkg.TenantContext(1), otherLink))

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
	svc, db := buildGuardianService(t)
	defer func() { _ = db.Close() }()

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
	profile.SetTenantID(1)
	_, err := db.NewInsert().Model(profile).ModelTableExpr(`users.guardian_profiles`).Exec(ctx)
	require.NoError(t, err)

	link := &userModels.StudentGuardian{
		StudentID:         studentID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "parent",
		EmergencyPriority: 3,
		Permissions:       permissions,
	}
	link.SetTenantID(1)
	_, err = db.NewInsert().Model(link).ModelTableExpr(`users.students_guardians`).Exec(ctx)
	require.NoError(t, err)

	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   account.ID,
		TenantID:    1,
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

func assertExactlyOnePrimaryPhone(t *testing.T, db *bun.DB, profileID int64) {
	t.Helper()
	repo := repositories.NewFactory(db).GuardianPhoneNumber
	phones, err := repo.FindByGuardianID(testpkg.TenantContext(1), profileID)
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
