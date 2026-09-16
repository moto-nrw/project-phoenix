package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/require"
)

// The identity chain an account needs to be usable as personnel (#2222):
// users.persons -> users.staff -> (for caregiver roles) users.teachers. These
// cases moved with EnsureSchoolIdentity out of services/auth (#3225) and keep
// asserting the same outcomes.

const identityAccount int64 = 9001

// Role tiers as auth.roles.base_role stores them.
const (
	tierAdmin    = "admin"
	tierUser     = "user"
	tierGuardian = "guardian"
)

func customRole(name, base string) *domain.RoleFacts {
	tenantID := lifecycleTenant
	role := &domain.RoleFacts{ID: 501, Name: name, TenantID: &tenantID}
	if base != "" {
		role.BaseRole = &base
	}
	return role
}

func identityInput(role *domain.RoleFacts) domain.SchoolIdentityInput {
	return domain.SchoolIdentityInput{
		AccountID:    identityAccount,
		TenantID:     lifecycleTenant,
		Role:         role,
		FirstName:    "Ada",
		LastName:     "Lovelace",
		CreatePerson: true,
	}
}

func identityInputWithTag(role *domain.RoleFacts, tagID string) domain.SchoolIdentityInput {
	in := identityInput(role)
	in.TagID = &tagID
	return in
}

func ensureIdentity(t *testing.T, f *lifecycleFixture, in domain.SchoolIdentityInput) (*domain.SchoolIdentity, error) {
	t.Helper()
	return f.lifecycle.EnsureSchoolIdentity(tenantContext(), in)
}

func (f *lifecycleFixture) seedAccountPerson(first, last string, tagID *string) domain.PersonRecord {
	accountID := identityAccount
	return f.staff.addPerson(domain.PersonRecord{FirstName: first, LastName: last, AccountID: &accountID, TagID: tagID})
}

// The bug of #2222: an account invited with a school's own role received a
// person and no staff record, which breaks every screen behind GetCurrentStaff.
func TestEnsureSchoolIdentity_CustomAdminRoleGetsStaffWithoutCaregiverProfile(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	identity, err := ensureIdentity(t, f, identityInput(customRole("OGS-Leitung", tierAdmin)))
	require.NoError(t, err)
	require.NotNil(t, identity)

	person := f.staff.persons[identity.PersonID]
	require.Equal(t, lifecycleTenant, person.TenantID)
	require.NotNil(t, person.AccountID)
	require.Equal(t, identityAccount, *person.AccountID)
	require.Len(t, f.staff.persons, 1)

	require.Len(t, f.staff.staffRows(), 1)
	require.Equal(t, lifecycleTenant, f.staff.staffRows()[0].TenantID)

	require.Zero(t, identity.TeacherID)
	require.Empty(t, f.staff.teachers, "an admin-tier role runs without a caregiver profile")
}

// A school's own caregiver-tier role gets the same caregiver profile the
// platform user role gets.
func TestEnsureSchoolIdentity_CustomUserRoleGetsCaregiverProfile(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	in := identityInput(customRole("OGS-Kraft", tierUser))
	in.Position = "Gruppenleitung"

	identity, err := ensureIdentity(t, f, in)
	require.NoError(t, err)
	require.NotNil(t, identity)
	require.NotZero(t, identity.TeacherID)

	require.Len(t, f.staff.teachers, 1)
	require.Equal(t, identity.StaffID, f.staff.teachers[identity.TeacherID].StaffID)
	require.Equal(t, []string{"Gruppenleitung"}, f.staff.positions)
}

// base_role is NULL on school roles created before migration 1.15.31. Unknown
// tier counts as personnel.
func TestEnsureSchoolIdentity_UnknownTierIsStaff(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	identity, err := ensureIdentity(t, f, identityInput(customRole("Alt-Rolle", "")))
	require.NoError(t, err)
	require.NotNil(t, identity)
	require.Len(t, f.staff.staffRows(), 1)
	require.Empty(t, f.staff.teachers)
}

func TestEnsureSchoolIdentity_GuardianTierProvisionsNothing(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	identity, err := ensureIdentity(t, f, identityInput(customRole("Sorgeberechtigt", tierGuardian)))
	require.NoError(t, err)
	require.Nil(t, identity)
	require.Empty(t, f.staff.persons)
	require.Empty(t, f.staff.staffRows())
}

// The Lehrkraft role is class_day-read-only by design (#1772) and keeps the
// staff record without a caregiver profile, even though its tier is 'user'.
func TestEnsureSchoolIdentity_LehrkraftGetsStaffWithoutCaregiverProfile(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	base := tierUser
	lehrkraft := &domain.RoleFacts{ID: 55, Name: "lehrkraft", IsSystem: true, BaseRole: &base}

	identity, err := ensureIdentity(t, f, identityInput(lehrkraft))
	require.NoError(t, err)
	require.NotNil(t, identity)
	require.Len(t, f.staff.staffRows(), 1)
	require.Empty(t, f.staff.teachers)
}

// Re-granting access must reuse the existing chain.
func TestEnsureSchoolIdentity_IsIdempotent(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	in := identityInput(customRole("OGS-Kraft", tierUser))

	first, err := ensureIdentity(t, f, in)
	require.NoError(t, err)
	second, err := ensureIdentity(t, f, in)
	require.NoError(t, err)

	require.Equal(t, *first, *second)
	require.Len(t, f.staff.persons, 1)
	require.Len(t, f.staff.staffRows(), 1)
	require.Len(t, f.staff.teachers, 1)
}

// An account with a person but no staff record is exactly the state the
// invitation flow produced. Provisioning attaches the staff record to it.
func TestEnsureSchoolIdentity_AdoptsExistingPerson(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	existing := f.seedAccountPerson("Bestehend", "Person", nil)

	identity, err := ensureIdentity(t, f, identityInput(customRole("OGS-Leitung", tierAdmin)))
	require.NoError(t, err)

	require.Equal(t, existing.ID, identity.PersonID)
	require.Len(t, f.staff.persons, 1, "the existing person must be reused")
	require.Len(t, f.staff.staffRows(), 1)
	require.Equal(t, existing.ID, f.staff.staffRows()[0].PersonID)
}

func TestEnsureSchoolIdentity_RefusesToInventAnIdentity(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	in := identityInput(customRole("OGS-Leitung", tierAdmin))
	in.FirstName = ""

	_, err := ensureIdentity(t, f, in)
	require.ErrorIs(t, err, domain.ErrSchoolIdentityNamesRequired)
	require.Empty(t, f.staff.staffRows())
}

func TestEnsureSchoolIdentity_WithoutCreatePersonLeavesAccountAlone(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	in := identityInput(customRole("OGS-Leitung", tierAdmin))
	in.CreatePerson = false

	identity, err := ensureIdentity(t, f, in)
	require.NoError(t, err)
	require.Nil(t, identity)
	require.Empty(t, f.staff.persons)
	require.Empty(t, f.staff.staffRows())
}

// Building the chain on a child's person would file the child as personnel,
// and nothing could repair that afterwards.
func TestEnsureSchoolIdentity_RefusesPersonThatIsAStudent(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	child := f.seedAccountPerson("Kind", "Datensatz", nil)
	f.staff.students[child.ID] = true

	_, err := ensureIdentity(t, f, identityInput(customRole("OGS-Leitung", tierAdmin)))
	require.ErrorIs(t, err, domain.ErrSchoolIdentityPersonIsStudent)

	require.Empty(t, f.staff.staffRows(), "no staff record may be built on a child's person")
	require.Empty(t, f.staff.teachers)
	require.Len(t, f.staff.persons, 1, "the child's record itself stays untouched")
}

func TestEnsureSchoolIdentity_ExistingNonStudentPersonPasses(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	existing := f.seedAccountPerson("Bestehend", "Person", nil)

	identity, err := ensureIdentity(t, f, identityInput(customRole("OGS-Leitung", tierAdmin)))
	require.NoError(t, err)
	require.Equal(t, existing.ID, identity.PersonID)
	require.Len(t, f.staff.staffRows(), 1)
}

// An unknown tag used to reach the foreign key and come back as a 500.
func TestEnsureSchoolIdentity_RefusesUnknownTag(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.store.cards["KNOWNCARD1"] = true

	_, err := ensureIdentity(t, f, identityInputWithTag(customRole("OGS-Leitung", tierAdmin), "FOREIGNCARD"))
	require.ErrorIs(t, err, domain.ErrSchoolIdentityTagUnknown)

	require.Empty(t, f.staff.persons, "nothing is written when the tag is refused")
	require.Empty(t, f.staff.staffRows())
}

func TestEnsureSchoolIdentity_AssignsKnownTagToNewPerson(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.store.cards["KNOWNCARD1"] = true

	identity, err := ensureIdentity(t, f, identityInputWithTag(customRole("OGS-Leitung", tierAdmin), "KNOWNCARD1"))
	require.NoError(t, err)
	person := f.staff.persons[identity.PersonID]
	require.NotNil(t, person.TagID)
	require.Equal(t, "KNOWNCARD1", *person.TagID)
	require.Len(t, f.staff.persons, 1)
}

// The reuse path used to drop the submitted transponder on the floor.
func TestEnsureSchoolIdentity_AssignsTagToExistingPersonWithoutOne(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.store.cards["KNOWNCARD1"] = true
	existing := f.seedAccountPerson("Bestehend", "Person", nil)

	identity, err := ensureIdentity(t, f, identityInputWithTag(customRole("OGS-Leitung", tierAdmin), "KNOWNCARD1"))
	require.NoError(t, err)
	require.Equal(t, existing.ID, identity.PersonID)
	person := f.staff.persons[existing.ID]
	require.NotNil(t, person.TagID)
	require.Equal(t, "KNOWNCARD1", *person.TagID)
}

// A transponder already in daily use is not silently swapped out.
func TestEnsureSchoolIdentity_RefusesConflictingTagOnExistingPerson(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.store.cards["KNOWNCARD1"] = true
	f.store.cards["OTHERCARD1"] = true
	worn := "OTHERCARD1"
	existing := f.seedAccountPerson("Bestehend", "Person", &worn)

	_, err := ensureIdentity(t, f, identityInputWithTag(customRole("OGS-Leitung", tierAdmin), "KNOWNCARD1"))
	require.ErrorIs(t, err, domain.ErrSchoolIdentityTagConflict)

	require.Equal(t, "OTHERCARD1", *f.staff.persons[existing.ID].TagID, "the transponder in use is left alone")
	require.Empty(t, f.staff.staffRows(), "the request is refused as a whole")
}

func TestEnsureSchoolIdentity_SameTagOnExistingPersonIsIdempotent(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.store.cards["KNOWNCARD1"] = true
	worn := "KNOWNCARD1"
	existing := f.seedAccountPerson("Bestehend", "Person", &worn)

	identity, err := ensureIdentity(t, f, identityInputWithTag(customRole("OGS-Leitung", tierAdmin), "KNOWNCARD1"))
	require.NoError(t, err)
	require.Equal(t, existing.ID, identity.PersonID)
	require.Len(t, f.staff.staffRows(), 1)
}

// An empty tag_id is what the HTTP layer sends for every request that did not
// ask for a bracelet.
func TestEnsureSchoolIdentity_EmptyTagIsNotALookup(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	identity, err := ensureIdentity(t, f, identityInputWithTag(customRole("OGS-Leitung", tierAdmin), "   "))
	require.NoError(t, err)
	require.Nil(t, f.staff.persons[identity.PersonID].TagID)
	require.Len(t, f.staff.persons, 1)
	require.Len(t, f.staff.staffRows(), 1)
}

// A transponder somebody else already wears would lose them door access.
func TestEnsureSchoolIdentity_RefusesTagWornBySomebodyElse(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.store.cards["KNOWNCARD1"] = true
	worn := "KNOWNCARD1"
	colleague := f.staff.addPerson(domain.PersonRecord{FirstName: "Andere", LastName: "Person", TagID: &worn})

	_, err := ensureIdentity(t, f, identityInputWithTag(customRole("OGS-Leitung", tierAdmin), "KNOWNCARD1"))
	require.ErrorIs(t, err, domain.ErrSchoolIdentityTagTaken)

	require.Len(t, f.staff.persons, 1, "no person is created for a refused request")
	require.Empty(t, f.staff.staffRows())
	require.Equal(t, "KNOWNCARD1", *f.staff.persons[colleague.ID].TagID, "the wearer keeps their transponder")
}

func TestEnsureSchoolIdentity_RefusesTakenTagForExistingPerson(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.store.cards["KNOWNCARD1"] = true
	worn := "KNOWNCARD1"
	f.staff.addPerson(domain.PersonRecord{FirstName: "Andere", LastName: "Person", TagID: &worn})
	existing := f.seedAccountPerson("Bestehend", "Person", nil)

	_, err := ensureIdentity(t, f, identityInputWithTag(customRole("OGS-Leitung", tierAdmin), "KNOWNCARD1"))
	require.ErrorIs(t, err, domain.ErrSchoolIdentityTagTaken)

	require.Nil(t, f.staff.persons[existing.ID].TagID, "the account's person is left without a transponder")
	require.Empty(t, f.staff.staffRows(), "the request is refused as a whole")
}

// Names are needed to CREATE a person, never to reuse one.
func TestEnsureSchoolIdentity_ReusesExistingPersonWithoutNames(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	existing := f.seedAccountPerson("Bestehend", "Person", nil)

	in := identityInput(customRole("OGS-Leitung", tierAdmin))
	in.FirstName = ""
	in.LastName = ""

	identity, err := ensureIdentity(t, f, in)
	require.NoError(t, err)
	require.Equal(t, existing.ID, identity.PersonID)
	require.Equal(t, "Bestehend", f.staff.persons[existing.ID].FirstName, "the person's own name is untouched")
	require.Len(t, f.staff.staffRows(), 1)

	// With nothing to reuse, the name is genuinely missing.
	fresh := newLifecycleFixture(t)
	_, err = ensureIdentity(t, fresh, in)
	require.ErrorIs(t, err, domain.ErrSchoolIdentityNamesRequired)
	require.Empty(t, fresh.staff.staffRows())
}

// The staff import files the person before the invitation goes out (#2600):
// accepting completes the chain on that person instead of filing a twin.
func TestEnsureSchoolIdentity_AdoptsHintedImportedPerson(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	imported := f.staff.addPerson(domain.PersonRecord{FirstName: "Importierte", LastName: "Person"})
	staffRow := f.staff.addStaff(imported.ID)

	in := identityInput(customRole("Betreuung", "user"))
	in.FirstName, in.LastName = "Anderer", "Name"
	in.PersonID = &imported.ID

	identity, err := ensureIdentity(t, f, in)
	require.NoError(t, err)
	require.NotNil(t, identity)
	require.Equal(t, imported.ID, identity.PersonID, "the hinted person is reused")
	person := f.staff.persons[imported.ID]
	require.NotNil(t, person.AccountID)
	require.Equal(t, in.AccountID, *person.AccountID)
	require.Equal(t, staffRow.ID, identity.StaffID, "the imported staff row is reused")
	require.Len(t, f.staff.staffRows(), 1, "no second staff row")
	require.NotZero(t, identity.TeacherID, "the caregiver profile is completed on acceptance")
}

func TestEnsureSchoolIdentity_IgnoresHintOwnedByAnotherAccount(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	owner := int64(4242)
	other := f.staff.addPerson(domain.PersonRecord{FirstName: "Fremde", LastName: "Person", AccountID: &owner})

	in := identityInput(customRole("Betreuung", "user"))
	in.PersonID = &other.ID

	identity, err := ensureIdentity(t, f, in)
	require.NoError(t, err)
	require.NotNil(t, identity)
	require.NotEqual(t, other.ID, identity.PersonID, "somebody else's person is never taken over")
	require.Equal(t, "Ada", f.staff.persons[identity.PersonID].FirstName)
	require.Len(t, f.staff.staffRows(), 1)
}

func TestEnsureSchoolIdentity_IgnoresHintThatIsAStudent(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	child := f.staff.addPerson(domain.PersonRecord{FirstName: "Kind", LastName: "Datensatz"})
	f.staff.students[child.ID] = true

	in := identityInput(customRole("Betreuung", "user"))
	in.PersonID = &child.ID

	identity, err := ensureIdentity(t, f, in)
	require.NoError(t, err)
	require.NotEqual(t, child.ID, identity.PersonID, "a child's record is never adopted")
	require.Nil(t, f.staff.persons[child.ID].AccountID)
}

func TestEnsureSchoolIdentity_ProvisioningFailuresSurface(t *testing.T) {
	t.Parallel()

	t.Run("staff", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.staff.createStaffErr = errLifecycleBoom
		_, err := ensureIdentity(t, f, identityInput(customRole("OGS-Leitung", tierAdmin)))
		require.ErrorIs(t, err, errLifecycleBoom)
		require.ErrorContains(t, err, "create staff")
	})

	t.Run("teacher", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.staff.createTeacherErr = errLifecycleBoom
		_, err := ensureIdentity(t, f, identityInput(customRole("OGS-Kraft", tierUser)))
		require.ErrorIs(t, err, errLifecycleBoom)
		require.ErrorContains(t, err, "create teacher")
	})
}

func TestEnsureSchoolIdentity_CaregiverUpgradeProvisionsProfile(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	in := identityInput(customRole("OGS-Leitung", tierAdmin))
	in.CaregiverUpgrade = true

	identity, err := f.lifecycle.EnsureSchoolIdentity(context.Background(), in)
	require.NoError(t, err)
	require.NotZero(t, identity.TeacherID)
}
