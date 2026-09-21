package compose

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

type storedGuardianProfile struct {
	TenantID               int64   `bun:"tenant_id"`
	FirstName              string  `bun:"first_name"`
	LastName               string  `bun:"last_name"`
	Email                  *string `bun:"email"`
	AddressStreet          *string `bun:"address_street"`
	AddressCity            *string `bun:"address_city"`
	AddressPostalCode      *string `bun:"address_postal_code"`
	PreferredContactMethod string  `bun:"preferred_contact_method"`
	LanguagePreference     string  `bun:"language_preference"`
	HasAccount             bool    `bun:"has_account"`
	AccountID              *int64  `bun:"account_id"`
	PortalLocale           *string `bun:"portal_locale"`
	Notes                  *string `bun:"notes"`
}

func readGuardianProfile(t *testing.T, db *bun.DB, id int64) storedGuardianProfile {
	t.Helper()
	var row storedGuardianProfile
	require.NoError(t, db.NewRaw(`SELECT tenant_id, first_name, last_name, email, address_street, address_city,
		address_postal_code, preferred_contact_method, language_preference, has_account, account_id, portal_locale, notes
		FROM users.guardian_profiles WHERE id = ?`, id).Scan(context.Background(), &row))
	return row
}

type storedGuardianPhone struct {
	ID          int64   `bun:"id"`
	TenantID    int64   `bun:"tenant_id"`
	PhoneNumber string  `bun:"phone_number"`
	PhoneType   string  `bun:"phone_type"`
	Label       *string `bun:"label"`
	IsPrimary   bool    `bun:"is_primary"`
	Priority    int     `bun:"priority"`
}

func readGuardianPhones(t *testing.T, db *bun.DB, guardianID int64) []storedGuardianPhone {
	t.Helper()
	var rows []storedGuardianPhone
	require.NoError(t, db.NewRaw(`SELECT id, tenant_id, phone_number, phone_type::text AS phone_type, label, is_primary, priority
		FROM users.guardian_phone_numbers WHERE guardian_profile_id = ? ORDER BY priority, id`, guardianID).
		Scan(context.Background(), &rows))
	return rows
}

type storedGuardianLink struct {
	ID                 int64          `bun:"id"`
	TenantID           int64          `bun:"tenant_id"`
	RelationshipType   string         `bun:"relationship_type"`
	GuardianRole       string         `bun:"guardian_role"`
	IsPrimary          bool           `bun:"is_primary"`
	IsEmergencyContact bool           `bun:"is_emergency_contact"`
	CanPickup          bool           `bun:"can_pickup"`
	PickupNotes        *string        `bun:"pickup_notes"`
	EmergencyPriority  int            `bun:"emergency_priority"`
	IsPayer            bool           `bun:"is_payer"`
	Permissions        map[string]any `bun:"permissions,type:jsonb"`
}

func readGuardianLink(t *testing.T, db *bun.DB, studentID, guardianID int64) storedGuardianLink {
	t.Helper()
	var row storedGuardianLink
	require.NoError(t, db.NewRaw(`SELECT id, tenant_id, relationship_type, guardian_role, is_primary, is_emergency_contact,
		can_pickup, pickup_notes, emergency_priority, is_payer, permissions
		FROM users.students_guardians WHERE student_id = ? AND guardian_profile_id = ?`, studentID, guardianID).
		Scan(context.Background(), &row))
	return row
}

func textPtr(value string) *string { return &value }
func flagPtr(value bool) *bool     { return &value }

func TestGuardianContactWritesNormalizeLikeEveryGuardianWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	id, err := module.CreateGuardianContact(ctx, peopledirectory.GuardianContactRecord{
		FirstName: "  Anna ", LastName: " Portal  ", Email: textPtr("  Anna.Portal@Example.DE "),
		AddressCity: textPtr("Köln"), PreferredContactMethod: "email",
	})
	require.NoError(t, err)
	require.Positive(t, id)
	stored := readGuardianProfile(t, db, id)
	assert.Equal(t, testpkg.Tenant(t), stored.TenantID)
	assert.Equal(t, "Anna", stored.FirstName)
	assert.Equal(t, "Portal", stored.LastName)
	require.NotNil(t, stored.Email)
	assert.Equal(t, "anna.portal@example.de", *stored.Email)
	assert.Equal(t, "Köln", *stored.AddressCity)
	assert.Equal(t, "email", stored.PreferredContactMethod)
	assert.Equal(t, "de", stored.LanguagePreference, "an empty language takes the column default")
	assert.False(t, stored.HasAccount)
	assert.Nil(t, stored.AccountID)
	assert.Nil(t, stored.PortalLocale)

	defaults, err := module.CreateGuardianContact(ctx, peopledirectory.GuardianContactRecord{FirstName: "Bernd", LastName: "Portal"})
	require.NoError(t, err)
	assert.Equal(t, "phone", readGuardianProfile(t, db, defaults).PreferredContactMethod)

	_, err = db.NewRaw(`UPDATE users.guardian_profiles SET notes = 'staff note', portal_locale = 'en' WHERE id = ?`, id).
		Exec(context.Background())
	require.NoError(t, err)
	err = module.UpdateGuardianContact(ctx, id, peopledirectory.GuardianContactRecord{
		FirstName: "Anne", LastName: "Portal", Email: nil, AddressStreet: textPtr("Ring 1"),
		PreferredContactMethod: "phone", LanguagePreference: "en",
	})
	require.NoError(t, err)
	stored = readGuardianProfile(t, db, id)
	assert.Equal(t, "Anne", stored.FirstName)
	assert.Nil(t, stored.Email)
	assert.Equal(t, "Ring 1", *stored.AddressStreet)
	assert.Nil(t, stored.AddressCity)
	assert.Equal(t, "phone", stored.PreferredContactMethod)
	assert.Equal(t, "en", stored.LanguagePreference)
	assert.Equal(t, "staff note", *stored.Notes, "columns outside the contact slice stay")
	assert.Equal(t, "en", *stored.PortalLocale)

	err = module.UpdateGuardianContact(ctx, id, peopledirectory.GuardianContactRecord{FirstName: "A", LastName: "B", Email: textPtr("not-an-address")})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian)
	err = module.UpdateGuardianContact(ctx, id, peopledirectory.GuardianContactRecord{FirstName: "A", LastName: "B", PreferredContactMethod: "pigeon"})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian)

	var missing int64
	require.NoError(t, db.NewRaw(`SELECT nextval('users.guardian_profiles_id_seq')`).Scan(context.Background(), &missing))
	err = module.UpdateGuardianContact(ctx, missing, peopledirectory.GuardianContactRecord{FirstName: "A", LastName: "B"})
	require.ErrorIs(t, err, peopledirectory.ErrGuardianNotFound)
}

func TestGuardianContactEmailConflictStaysDetectable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	first, err := module.CreateGuardianContact(ctx, peopledirectory.GuardianContactRecord{FirstName: "A", LastName: "One", Email: textPtr("same@example.de")})
	require.NoError(t, err)
	second, err := module.CreateGuardianContact(ctx, peopledirectory.GuardianContactRecord{FirstName: "B", LastName: "Two"})
	require.NoError(t, err)

	assertEmailTaken := func(err error) {
		t.Helper()
		require.ErrorIs(t, err, peopledirectory.ErrGuardianEmailTaken)
		var pgErr pgdriver.Error
		require.True(t, errors.As(err, &pgErr), "the raw database error stays in the chain")
		assert.Equal(t, "23505", pgErr.Field('C'))
		assert.Equal(t, "idx_guardian_profiles_tenant_email", pgErr.Field('n'))
	}
	_, err = module.CreateGuardianContact(ctx, peopledirectory.GuardianContactRecord{FirstName: "C", LastName: "Three", Email: textPtr("SAME@example.de")})
	assertEmailTaken(err)
	assertEmailTaken(module.UpdateGuardianContact(ctx, second, peopledirectory.GuardianContactRecord{FirstName: "B", LastName: "Two", Email: textPtr("Same@Example.de")}))
	require.NoError(t, module.UpdateGuardianContact(ctx, first, peopledirectory.GuardianContactRecord{FirstName: "A", LastName: "One", Email: textPtr("same@example.de")}),
		"a profile keeps its own address")

	// Another school may use the same address.
	otherCtx, _ := otherTenantContext(t, db)
	_, err = module.CreateGuardianContact(otherCtx, peopledirectory.GuardianContactRecord{FirstName: "D", LastName: "Four", Email: textPtr("same@example.de")})
	require.NoError(t, err)
}

func TestGuardianPhoneCommandsWriteTheGivenRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, testpkg.Tenant(t), "Petra", "Phone", "portal-phone")

	require.NoError(t, module.ReplaceGuardianPhones(ctx, guardian.ID, []peopledirectory.GuardianPhoneRecord{
		{PhoneNumber: " 0170 1234567 ", PhoneType: peopledirectory.PhoneTypeMobile, Label: textPtr("  "), IsPrimary: true, Priority: 1},
		{PhoneNumber: "+49 (221) 555-01", PhoneType: peopledirectory.PhoneTypeWork, Label: textPtr(" Büro "), Priority: 0},
	}))
	phones := readGuardianPhones(t, db, guardian.ID)
	require.Len(t, phones, 2)
	assert.Equal(t, testpkg.Tenant(t), phones[0].TenantID)
	assert.Equal(t, "0170 1234567", phones[0].PhoneNumber)
	assert.Equal(t, "mobile", phones[0].PhoneType)
	assert.Nil(t, phones[0].Label, "a blank label is stored as NULL")
	assert.True(t, phones[0].IsPrimary)
	assert.Equal(t, "+49 (221) 555-01", phones[1].PhoneNumber)
	assert.Equal(t, "Büro", *phones[1].Label)
	assert.Equal(t, 1, phones[1].Priority, "a priority below one is stored as one")
	assert.False(t, phones[1].IsPrimary)

	require.NoError(t, module.ReplaceGuardianPhones(ctx, guardian.ID, []peopledirectory.GuardianPhoneRecord{
		{PhoneNumber: "0221 999999", PhoneType: peopledirectory.PhoneTypeHome, IsPrimary: true, Priority: 1},
	}))
	phones = readGuardianPhones(t, db, guardian.ID)
	require.Len(t, phones, 1, "the replace drops every earlier row")
	assert.Equal(t, "home", phones[0].PhoneType)

	err := module.ReplaceGuardianPhones(ctx, guardian.ID, []peopledirectory.GuardianPhoneRecord{
		{PhoneNumber: "0221 111111", PhoneType: peopledirectory.PhoneTypeHome},
		{PhoneNumber: "call me", PhoneType: peopledirectory.PhoneTypeHome},
	})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian)
	assert.Len(t, readGuardianPhones(t, db, guardian.ID), 1, "an invalid row refuses the whole replace before writing")
	_, err = module.AddGuardianPhoneRecord(ctx, guardian.ID, peopledirectory.GuardianPhoneRecord{PhoneNumber: "0170 12", PhoneType: "fax"})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian)
	_, err = module.AddGuardianPhoneRecord(ctx, guardian.ID, peopledirectory.GuardianPhoneRecord{PhoneNumber: "1-2", PhoneType: peopledirectory.PhoneTypeHome})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian, "at least three digits")

	added, err := module.AddGuardianPhoneRecord(ctx, guardian.ID, peopledirectory.GuardianPhoneRecord{
		PhoneNumber: "0170 7654321", PhoneType: peopledirectory.PhoneTypeMobile, IsPrimary: false, Priority: 2,
	})
	require.NoError(t, err)
	require.Positive(t, added)

	require.NoError(t, module.SetGuardianPhoneNumber(ctx, added, " 0170 0000000 "))
	phones = readGuardianPhones(t, db, guardian.ID)
	require.Len(t, phones, 2)
	assert.Equal(t, added, phones[1].ID)
	assert.Equal(t, "0170 0000000", phones[1].PhoneNumber)
	assert.Equal(t, 2, phones[1].Priority, "only the number changes")
	assert.Equal(t, "mobile", phones[1].PhoneType)
	require.ErrorIs(t, module.SetGuardianPhoneNumber(ctx, added, "abc"), peopledirectory.ErrInvalidGuardian)

	require.NoError(t, module.DeleteGuardianPhoneRecord(ctx, added))
	assert.Len(t, readGuardianPhones(t, db, guardian.ID), 1)
	require.ErrorIs(t, module.DeleteGuardianPhoneRecord(ctx, added), peopledirectory.ErrGuardianPhoneNotFound)
	require.ErrorIs(t, module.SetGuardianPhoneNumber(ctx, added, "0170 1111111"), peopledirectory.ErrGuardianPhoneNotFound)
}

func TestGuardianLinkCommandsInsertOnceAndPatchOnlySuppliedColumns(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Lina", "Link", "1a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Lars", "Link", "portal-link")

	link := peopledirectory.GuardianContactLink{
		StudentID: student.ID, GuardianProfileID: guardian.ID, RelationshipType: "Relative",
		IsEmergencyContact: true, CanPickup: false, PickupNotes: textPtr("nur dienstags"),
	}
	linkID, inserted, err := module.LinkGuardianContact(ctx, link)
	require.NoError(t, err)
	require.True(t, inserted)
	stored := readGuardianLink(t, db, student.ID, guardian.ID)
	assert.Equal(t, stored.ID, linkID, "the new relationship's ID is returned")
	assert.Equal(t, tenantID, stored.TenantID)
	assert.Equal(t, "relative", stored.RelationshipType)
	assert.Equal(t, "custom", stored.GuardianRole, "an empty role is stored as custom")
	assert.Equal(t, 1, stored.EmergencyPriority)
	assert.Empty(t, stored.Permissions)
	assert.True(t, stored.IsEmergencyContact)
	assert.False(t, stored.CanPickup)
	assert.Equal(t, "nur dienstags", *stored.PickupNotes)

	link.GuardianRole = "Pickup_Only"
	link.Permissions = json.RawMessage(`{"parent_portal.access": true}`)
	linkID, inserted, err = module.LinkGuardianContact(ctx, link)
	require.NoError(t, err)
	assert.False(t, inserted, "an existing pair is left alone")
	assert.Zero(t, linkID)
	assert.Equal(t, "custom", readGuardianLink(t, db, student.ID, guardian.ID).GuardianRole)

	other := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Olga", "Link", "portal-link-other")
	linkID, inserted, err = module.LinkGuardianContact(ctx, peopledirectory.GuardianContactLink{
		StudentID: student.ID, GuardianProfileID: other.ID, RelationshipType: "parent", GuardianRole: " Primary_Guardian ",
		EmergencyPriority: 2, Permissions: json.RawMessage(`{"parent_portal.access": true, "parent_portal.messages.read": true}`),
	})
	require.NoError(t, err)
	require.True(t, inserted)
	otherLink := readGuardianLink(t, db, student.ID, other.ID)
	assert.Equal(t, otherLink.ID, linkID)
	assert.Equal(t, "primary_guardian", otherLink.GuardianRole)
	assert.Equal(t, 2, otherLink.EmergencyPriority)
	assert.Equal(t, map[string]any{"parent_portal.access": true, "parent_portal.messages.read": true}, otherLink.Permissions)

	_, _, err = module.LinkGuardianContact(ctx, peopledirectory.GuardianContactLink{StudentID: student.ID, GuardianProfileID: other.ID, RelationshipType: "neighbour"})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian)
	for _, malformed := range []string{`[true]`, `null`, `{"parent_portal.access":`} {
		_, _, err = module.LinkGuardianContact(ctx, peopledirectory.GuardianContactLink{
			StudentID: student.ID, GuardianProfileID: other.ID, RelationshipType: "parent", Permissions: json.RawMessage(malformed),
		})
		require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian, "permissions %s must be a JSON object", malformed)
	}

	affected, err := module.PatchGuardianLinkPickup(ctx, peopledirectory.GuardianLinkPickupPatch{LinkID: stored.ID, CanPickup: flagPtr(true)})
	require.NoError(t, err)
	assert.EqualValues(t, 1, affected)
	patched := readGuardianLink(t, db, student.ID, guardian.ID)
	assert.True(t, patched.CanPickup)
	assert.True(t, patched.IsEmergencyContact, "an unsupplied flag is untouched")
	assert.Equal(t, "nur dienstags", *patched.PickupNotes, "unsupplied notes are untouched")

	affected, err = module.PatchGuardianLinkPickup(ctx, peopledirectory.GuardianLinkPickupPatch{
		LinkID: stored.ID, IsEmergencyContact: flagPtr(false), SetPickupNotes: true,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, affected)
	patched = readGuardianLink(t, db, student.ID, guardian.ID)
	assert.False(t, patched.IsEmergencyContact)
	assert.Nil(t, patched.PickupNotes, "supplied nil notes clear the column")
	assert.True(t, patched.CanPickup)

	_, err = module.PatchGuardianLinkPickup(ctx, peopledirectory.GuardianLinkPickupPatch{LinkID: stored.ID})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian)
	_, err = db.NewRaw(`DELETE FROM users.students_guardians WHERE id = ?`, stored.ID).Exec(context.Background())
	require.NoError(t, err)
	affected, err = module.PatchGuardianLinkPickup(ctx, peopledirectory.GuardianLinkPickupPatch{LinkID: stored.ID, CanPickup: flagPtr(false)})
	require.NoError(t, err)
	assert.Zero(t, affected, "a vanished relationship reports zero rows")
}

func TestGuardianPortalLocaleOnlyTouchesTheTenantInContext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	accountID, here, _, _ := guardianRows(t, db, testpkg.Tenant(t), "primary_guardian")
	otherCtx, otherTenant := otherTenantContext(t, db)
	there := testpkg.CreateTestGuardianProfileForTenant(t, db, otherTenant, "Sabine", "Elsewhere", "portal-locale")
	_, err := db.NewRaw(`UPDATE users.guardian_profiles SET account_id = ?, has_account = TRUE WHERE id = ?`, accountID, there.ID).
		Exec(context.Background())
	require.NoError(t, err)

	updated, err := module.SetGuardianPortalLocale(ctx, accountID, "en")
	require.NoError(t, err)
	assert.EqualValues(t, 1, updated)
	assert.Equal(t, "en", *readGuardianProfile(t, db, here).PortalLocale)
	assert.Nil(t, readGuardianProfile(t, db, there.ID).PortalLocale, "the other school's profile keeps its locale")

	updated, err = module.SetGuardianPortalLocale(otherCtx, accountID, "tr")
	require.NoError(t, err)
	assert.EqualValues(t, 1, updated)
	assert.Equal(t, "tr", *readGuardianProfile(t, db, there.ID).PortalLocale)
	assert.Equal(t, "en", *readGuardianProfile(t, db, here).PortalLocale)

	stranger := testpkg.CreateTestAccount(t, db, "portal-locale-stranger")
	updated, err = module.SetGuardianPortalLocale(ctx, stranger.ID, "en")
	require.NoError(t, err)
	assert.Zero(t, updated, "an account without a profile here updates nothing")

	updated, err = module.SetGuardianPortalLocale(ctx, accountID, "xx-unchecked")
	require.NoError(t, err, "the locale is the caller's rule")
	assert.EqualValues(t, 1, updated)
	assert.Equal(t, "xx-unchecked", *readGuardianProfile(t, db, here).PortalLocale)
}

func TestStudentPhotoConsentWritesTheGivenSlice(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	actor := testpkg.CreateTestAccount(t, db, "portal-consent")
	student := testpkg.CreateTestStudentForTenant(t, db, testpkg.Tenant(t), "Pia", "Photo", "1a")
	type photoRow struct {
		PhotoPath *string    `bun:"photo_path"`
		GivenAt   *time.Time `bun:"photo_consent_given_at"`
		GivenBy   *int64     `bun:"photo_consent_given_by"`
	}
	read := func() photoRow {
		var row photoRow
		require.NoError(t, db.NewRaw(`SELECT photo_path, photo_consent_given_at, photo_consent_given_by
			FROM users.student_profiles WHERE id = ?`, student.ID).Scan(context.Background(), &row))
		return row
	}
	_, err := db.NewRaw(`UPDATE users.student_profiles SET photo_path = '/photos/pia.jpg' WHERE id = ?`, student.ID).Exec(context.Background())
	require.NoError(t, err)

	grantedAt := time.Date(2026, 9, 21, 8, 30, 0, 0, time.UTC)
	require.NoError(t, module.SetStudentPhotoConsent(ctx, peopledirectory.StudentPhotoState{
		StudentID: student.ID, PhotoPath: textPtr("/photos/pia.jpg"), PhotoConsentGivenAt: &grantedAt, PhotoConsentGivenBy: &actor.ID,
	}))
	row := read()
	assert.Equal(t, "/photos/pia.jpg", *row.PhotoPath)
	require.NotNil(t, row.GivenAt)
	assert.True(t, grantedAt.Equal(*row.GivenAt))
	assert.Equal(t, actor.ID, *row.GivenBy)

	require.NoError(t, module.SetStudentPhotoConsent(ctx, peopledirectory.StudentPhotoState{StudentID: student.ID}))
	row = read()
	assert.Nil(t, row.PhotoPath)
	assert.Nil(t, row.GivenAt)
	assert.Nil(t, row.GivenBy)

	err = module.SetStudentPhotoConsent(ctx, peopledirectory.StudentPhotoState{StudentID: student.ID, PhotoConsentGivenAt: &grantedAt})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidStudent)
	var missing int64
	require.NoError(t, db.NewRaw(`SELECT nextval(pg_get_serial_sequence('users.student_profiles', 'id'))`).Scan(context.Background(), &missing))
	require.ErrorIs(t, module.SetStudentPhotoConsent(ctx, peopledirectory.StudentPhotoState{StudentID: missing}), peopledirectory.ErrStudentNotFound)
}

// TestGuardianPortalCommandsStayInsideTheTenant writes from another school's
// context against every row of this test's tenant: nothing may change.
func TestGuardianPortalCommandsStayInsideTheTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Tara", "Tenant", "1a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Tom", "Tenant", "portal-tenant")
	link := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, student.ID, guardian.ID, "primary_guardian")
	phoneID, err := module.AddGuardianPhoneRecord(ctx, guardian.ID, peopledirectory.GuardianPhoneRecord{
		PhoneNumber: "0170 1234567", PhoneType: peopledirectory.PhoneTypeMobile, IsPrimary: true, Priority: 1,
	})
	require.NoError(t, err)
	before := readGuardianProfile(t, db, guardian.ID)
	otherCtx, _ := otherTenantContext(t, db)

	require.ErrorIs(t, module.UpdateGuardianContact(otherCtx, guardian.ID, peopledirectory.GuardianContactRecord{FirstName: "Hijacked", LastName: "Name"}),
		peopledirectory.ErrGuardianNotFound)
	require.ErrorIs(t, module.SetGuardianPhoneNumber(otherCtx, phoneID, "0170 9999999"), peopledirectory.ErrGuardianPhoneNotFound)
	require.ErrorIs(t, module.DeleteGuardianPhoneRecord(otherCtx, phoneID), peopledirectory.ErrGuardianPhoneNotFound)
	require.NoError(t, module.ReplaceGuardianPhones(otherCtx, guardian.ID, nil), "the delete matches nothing in another school")
	require.Error(t, module.ReplaceGuardianPhones(otherCtx, guardian.ID, []peopledirectory.GuardianPhoneRecord{
		{PhoneNumber: "0170 5555555", PhoneType: peopledirectory.PhoneTypeMobile},
	}), "a phone cannot attach to another school's profile")
	_, err = module.AddGuardianPhoneRecord(otherCtx, guardian.ID, peopledirectory.GuardianPhoneRecord{PhoneNumber: "0170 5555555", PhoneType: peopledirectory.PhoneTypeMobile})
	require.Error(t, err)
	_, _, err = module.LinkGuardianContact(otherCtx, peopledirectory.GuardianContactLink{
		StudentID: student.ID, GuardianProfileID: guardian.ID, RelationshipType: "parent",
	})
	require.Error(t, err, "a relationship cannot join another school's rows")
	affected, err := module.PatchGuardianLinkPickup(otherCtx, peopledirectory.GuardianLinkPickupPatch{LinkID: link.ID, CanPickup: flagPtr(false)})
	require.NoError(t, err)
	assert.Zero(t, affected)
	require.ErrorIs(t, module.SetStudentPhotoConsent(otherCtx, peopledirectory.StudentPhotoState{StudentID: student.ID}), peopledirectory.ErrStudentNotFound)

	assert.Equal(t, before, readGuardianProfile(t, db, guardian.ID))
	phones := readGuardianPhones(t, db, guardian.ID)
	require.Len(t, phones, 1)
	assert.Equal(t, "0170 1234567", phones[0].PhoneNumber)
	assert.True(t, readGuardianLink(t, db, student.ID, guardian.ID).CanPickup)
}

func TestGuardianPortalCommandsRefuseWithoutTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, testpkg.Tenant(t), "Nina", "Tenantless", "portal-tenantless")
	record := peopledirectory.GuardianContactRecord{FirstName: "Nina", LastName: "Tenantless"}

	noTenant := testpkg.WithPackageTenantRuntime(context.Background())
	_, err := module.CreateGuardianContact(noTenant, record)
	require.Error(t, err)
	require.Error(t, module.UpdateGuardianContact(noTenant, guardian.ID, record))

	require.NoError(t, tenant.WithinAdmin(testpkg.Ctx(t), func(adminCtx context.Context) error {
		_, err := module.CreateGuardianContact(adminCtx, record)
		require.Error(t, err, "an admin transaction is not a tenant transaction")
		require.Error(t, module.UpdateGuardianContact(adminCtx, guardian.ID, record))
		require.Error(t, module.ReplaceGuardianPhones(adminCtx, guardian.ID, nil))
		_, err = module.SetGuardianPortalLocale(adminCtx, testpkg.CreateTestAccount(t, db, "portal-admin").ID, "en")
		require.Error(t, err)
		return nil
	}))
	assert.Equal(t, "Nina", readGuardianProfile(t, db, guardian.ID).FirstName)
}
