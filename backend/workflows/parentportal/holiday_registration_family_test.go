package parentportal_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/api"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	parentService "github.com/moto-ogs/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-ogs/project-phoenix/workflows/parentportal/compose"
)

func buildHolidayRegistrationFamilyService(t *testing.T, wishes ...parentService.HolidayWishStatements) (*parentService.Portal, *bun.DB) {
	t.Helper()
	return buildHolidayRegistrationFamilyServiceWithProofs(t, nil, wishes...)
}

func buildHolidayRegistrationFamilyServiceWithProofs(t *testing.T, proofs parentService.HolidayAccountProofs, wishes ...parentService.HolidayWishStatements) (*parentService.Portal, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	people := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	var wishStatements parentService.HolidayWishStatements
	if len(wishes) > 0 {
		wishStatements = wishes[0]
	}
	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		Memberships:          parentMemberships(t, db),
		StudentGuardianRepo:  guardStudentGuardians(t, db),
		StudentRepo:          guardStudents(t, db),
		HolidayWishes:        wishStatements,
		HolidayAccountProofs: proofs,
		GuardianChanges:      owners.ParentGuardianChanges(),
		DeparturePolicy:      owners.ParentDeparturePolicy(),
		GuardianPolicy:       owners.NewParentGuardianPolicy(),
		Fingerprints:         owners.SecurityFingerprints{},
		Declarations:         owners.ParentDeclarationPolicy(),
		Locales:              owners.ParentPortalLocales(),
		Runtime:              owners.ParentPortalRuntime{},
		ChildRepo:            repos.ParentChildReads(),
		PersonRepo:           owners.NewParentPersonReads(people.Person),
		GuardianProfileRepo:  owners.NewParentGuardianProfileReads(people.GuardianProfile),
		GuardianPhoneRepo:    owners.NewParentGuardianPhoneReads(people.GuardianPhoneNumber),
		ChildDataAccess:      owners.NewParentChildDataAccess(owners.NewTestAuditPersistence(testpkg.NewDatabase(t, db), nil).DataAccessLog),
		Logger:               slog.Default(),
		CarePlanPolicy:       owners.NewParentCarePlanPolicy(),
	})
	return svc, db
}

func prefillReads(t *testing.T, db *bun.DB, studentID int64) int {
	t.Helper()
	var reads int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM audit.data_access_log
		WHERE resource_type = 'holiday_prefill' AND student_id = ?`, studentID).Scan(context.Background(), &reads))
	return reads
}

// TestHolidayRegistrationFamilyOffersOnlyPermittedOwnChildren pins #4662: the
// prefill offers a child only where the account's relationship grants
// parent_portal.enrollment.submit at a school of the Träger, and records each
// read in the school's data access log. A child of another family, a child of
// the account without the permission and a child at another Träger's school
// are not offered and not read.
func TestHolidayRegistrationFamilyOffersOnlyPermittedOwnChildren(t *testing.T) {
	t.Parallel()
	svc, db := buildHolidayRegistrationFamilyService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	_, err := db.ExecContext(ctx, `UPDATE users.persons SET birthday = '2018-03-01' WHERE id = ?`, chain.PersonID)
	require.NoError(t, err)

	pickupOnly := testpkg.CreateTestStudent(t, db, "Mia", "Schneider", "2b")
	testpkg.CreateTestStudentGuardianLink(t, db, pickupOnly.ID, chain.GuardianProfileID, "pickup_only")
	foreign := testpkg.CreateTestStudent(t, db, "Tom", "Fremd", "1a")
	foreignGuardian := testpkg.CreateTestGuardianProfileNamed(t, db, "Frieda", "Fremd", "frieda.fremd")
	testpkg.CreateTestStudentGuardianLink(t, db, foreign.ID, foreignGuardian.ID, "primary_guardian")

	family, err := svc.HolidayRegistrationFamily(ctx, chain.AccountID, parentService.HolidayRegistrationScope{OrganizationID: 42, SchoolIDs: []int64{chain.TenantID}})
	require.NoError(t, err)
	require.Len(t, family.Children, 1, "only the child whose relationship grants the registration")
	child := family.Children[0]
	assert.Equal(t, chain.StudentID, child.StudentID)
	assert.Equal(t, chain.TenantID, child.SchoolID)
	assert.Equal(t, "Felix", child.FirstName)
	assert.Equal(t, "Schneider", child.LastName)
	require.NotNil(t, child.BirthDate)
	assert.Equal(t, calendar.NewDate(2018, 3, 1), *child.BirthDate)
	require.NotNil(t, family.Guardian)
	assert.Equal(t, "Sabine", family.Guardian.FirstName)
	assert.Equal(t, chain.Email, family.Guardian.Email)

	assert.Equal(t, 1, prefillReads(t, db, chain.StudentID), "the read of the offered child is in the school's audit")
	assert.Zero(t, prefillReads(t, db, pickupOnly.ID), "a child without the permission is not read")
	assert.Zero(t, prefillReads(t, db, foreign.ID), "a child of another family is not read")

	other, err := svc.HolidayRegistrationFamily(ctx, chain.AccountID, parentService.HolidayRegistrationScope{OrganizationID: 43, SchoolIDs: []int64{chain.TenantID + 1_000_000}})
	require.NoError(t, err)
	assert.Empty(t, other.Children, "a child at a school of another Träger is not offered")
	assert.Nil(t, other.Guardian)
	assert.Equal(t, 1, prefillReads(t, db, chain.StudentID), "nothing is read for another Träger")
}

// fixedWishes states every child's wish as yes and records the claims.
type fixedWishes struct {
	claims *[]parentService.HolidayWishClaim
}

func (w fixedWishes) HolidayWishStatement(_ context.Context, claim parentService.HolidayWishClaim) (string, string, error) {
	*w.claims = append(*w.claims, claim)
	return "yes", "sealed-statement", nil
}

// TestHolidayRegistrationFamilyStatesTheWishOnlyWhenThePeriodAsks pins the
// prefill half of #4703: the wish of a child comes only with the child the
// relationship right offers, only for a period that takes it into account,
// for the link's period and the child's school, and its read is in the
// school's data access log. A child without the right gets no statement.
func TestHolidayRegistrationFamilyStatesTheWishOnlyWhenThePeriodAsks(t *testing.T) {
	t.Parallel()
	var claims []parentService.HolidayWishClaim
	svc, db := buildHolidayRegistrationFamilyService(t, fixedWishes{claims: &claims})
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	_, err := db.ExecContext(ctx, `UPDATE users.persons SET birthday = '2018-03-01' WHERE id = ?`, chain.PersonID)
	require.NoError(t, err)
	pickupOnly := testpkg.CreateTestStudent(t, db, "Mia", "Schneider", "2b")
	testpkg.CreateTestStudentGuardianLink(t, db, pickupOnly.ID, chain.GuardianProfileID, "pickup_only")

	off, err := svc.HolidayRegistrationFamily(ctx, chain.AccountID, parentService.HolidayRegistrationScope{OrganizationID: 42, PeriodID: 7, SchoolIDs: []int64{chain.TenantID}})
	require.NoError(t, err)
	require.Len(t, off.Children, 1)
	assert.Empty(t, off.Children[0].HolidayWish, "a period without the rule states no wish")
	assert.Empty(t, claims)

	scope := parentService.HolidayRegistrationScope{
		OrganizationID: 42, PeriodID: 7, SchoolIDs: []int64{chain.TenantID}, HolidayWishRule: true, FirstDay: calendar.NewDate(2027, 7, 5),
	}
	on, err := svc.HolidayRegistrationFamily(ctx, chain.AccountID, scope)
	require.NoError(t, err)
	require.Len(t, on.Children, 1, "only the child with the relationship right")
	assert.Equal(t, "yes", on.Children[0].HolidayWish)
	assert.Equal(t, "sealed-statement", on.Children[0].HolidayWishProof)
	require.Len(t, claims, 1, "no statement for the child without the right")
	assert.Equal(t, parentService.HolidayWishClaim{
		OrganizationID: 42, PeriodID: 7, SchoolID: chain.TenantID, StudentID: chain.StudentID,
		FirstName: "Felix", LastName: "Schneider", BirthDate: calendar.NewDate(2018, 3, 1), On: calendar.NewDate(2027, 7, 5),
	}, claims[0])

	var wishReads int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM audit.data_access_log
		WHERE resource_type = 'holiday_prefill' AND student_id = ? AND metadata->>'holiday_wish' = 'true'`, chain.StudentID).Scan(context.Background(), &wishReads))
	assert.Equal(t, 1, wishReads, "the read of the wish is in the school's audit")
	assert.Zero(t, prefillReads(t, db, pickupOnly.ID))

	_, err = buildHolidayRegistrationFamilyServiceWithoutWishes(t).HolidayRegistrationFamily(ctx, chain.AccountID, scope)
	require.Error(t, err, "a period with the rule needs the wish statements")
}

func buildHolidayRegistrationFamilyServiceWithoutWishes(t *testing.T) *parentService.Portal {
	t.Helper()
	svc, _ := buildHolidayRegistrationFamilyService(t)
	return svc
}

// recordedAccountProofs seals a readable proof and records what it sealed.
type recordedAccountProofs struct {
	sealed *[][3]int64
}

func (p recordedAccountProofs) HolidayParentAccountProof(organizationID, periodID, accountID int64) (string, error) {
	*p.sealed = append(*p.sealed, [3]int64{organizationID, periodID, accountID})
	return fmt.Sprintf("proof-%d-%d-%d", organizationID, periodID, accountID), nil
}

// TestHolidayRegistrationFamilyCarriesTheAccountProofForEveryAccount pins
// ADR 0216 point 2: the prefill seals the signed-in account for the link's
// Träger and period, also for an account the link offers no child, so a
// parent without a school links the registration at once.
func TestHolidayRegistrationFamilyCarriesTheAccountProofForEveryAccount(t *testing.T) {
	t.Parallel()
	var sealed [][3]int64
	svc, db := buildHolidayRegistrationFamilyServiceWithProofs(t, recordedAccountProofs{sealed: &sealed})
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	withoutChild := testpkg.CreateTestAccount(t, db, "ferien-ohne-kind")

	family, err := svc.HolidayRegistrationFamily(ctx, chain.AccountID, parentService.HolidayRegistrationScope{OrganizationID: 42, PeriodID: 420, SchoolIDs: []int64{chain.TenantID}})
	require.NoError(t, err)
	require.Len(t, family.Children, 1)
	assert.Equal(t, fmt.Sprintf("proof-42-420-%d", chain.AccountID), family.AccountProof)

	empty, err := svc.HolidayRegistrationFamily(ctx, withoutChild.ID, parentService.HolidayRegistrationScope{OrganizationID: 42, PeriodID: 420, SchoolIDs: []int64{chain.TenantID}})
	require.NoError(t, err)
	assert.Empty(t, empty.Children, "the link offers this account no child")
	assert.Nil(t, empty.Guardian)
	assert.Equal(t, fmt.Sprintf("proof-42-420-%d", withoutChild.ID), empty.AccountProof, "the proof is there for every account")
	assert.Equal(t, [][3]int64{{42, 420, chain.AccountID}, {42, 420, withoutChild.ID}}, sealed)
}

// TestHolidayRegistrationFamilyOffersAddressAndContacts pins #6682 (option
// A): the prefill carries the parent's own postal address and, per offered
// child, the other guardians the school knows: a custodian for the second
// custodian and the emergency and pickup contacts, each with the email and
// phone the school knows, also a guardian with an own account. A social
// worker is no family contact and stays out.
func TestHolidayRegistrationFamilyOffersAddressAndContacts(t *testing.T) {
	t.Parallel()
	svc, db := buildHolidayRegistrationFamilyService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	_, err := db.ExecContext(ctx, `UPDATE users.guardian_profiles SET address_street = 'Lindenweg 4', address_postal_code = '49143', address_city = 'Bissendorf' WHERE id = ?`, chain.GuardianProfileID)
	require.NoError(t, err)

	coParent := testpkg.CreateTestGuardianProfileNamed(t, db, "Jonas", "Schneider", "jonas.schneider")
	testpkg.CreateTestStudentGuardianLink(t, db, chain.StudentID, coParent.ID, "co_guardian")
	_, err = db.ExecContext(ctx, `INSERT INTO users.guardian_phone_numbers (tenant_id, guardian_profile_id, phone_number, phone_type, is_primary) VALUES (?, ?, '+49 5402 1234', 'mobile', true)`, chain.TenantID, coParent.ID)
	require.NoError(t, err)
	grandma := testpkg.CreateTestGuardianProfileNamed(t, db, "Erika", "Brandt", "erika.brandt")
	testpkg.CreateTestStudentGuardianLinkOfType(t, db, chain.TenantID, chain.StudentID, grandma.ID, "relative", "pickup_only")
	_, err = db.ExecContext(ctx, `INSERT INTO users.guardian_phone_numbers (tenant_id, guardian_profile_id, phone_number, phone_type, is_primary) VALUES (?, ?, '+49 5402 9876', 'home', true)`, chain.TenantID, grandma.ID)
	require.NoError(t, err)
	ownAccount := testpkg.CreateTestAccount(t, db, "ferien-eigenes-konto")
	stepParent := testpkg.CreateTestGuardianProfileNamed(t, db, "Lena", "Vogt", "lena.vogt")
	_, err = db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = ?, has_account = true WHERE id = ?`, ownAccount.ID, stepParent.ID)
	require.NoError(t, err)
	testpkg.CreateTestStudentGuardianLink(t, db, chain.StudentID, stepParent.ID, "emergency_contact")
	_, err = db.ExecContext(ctx, `INSERT INTO users.guardian_phone_numbers (tenant_id, guardian_profile_id, phone_number, phone_type, is_primary) VALUES (?, ?, '+49 5402 5555', 'mobile', true)`, chain.TenantID, stepParent.ID)
	require.NoError(t, err)
	socialWorker := testpkg.CreateTestGuardianProfileNamed(t, db, "Paul", "Amt", "paul.amt")
	testpkg.CreateTestStudentGuardianLinkOfType(t, db, chain.TenantID, chain.StudentID, socialWorker.ID, "other", "social_worker")

	family, err := svc.HolidayRegistrationFamily(ctx, chain.AccountID, parentService.HolidayRegistrationScope{OrganizationID: 42, SchoolIDs: []int64{chain.TenantID}})
	require.NoError(t, err)
	require.NotNil(t, family.Guardian)
	assert.Equal(t, &parentService.HolidayRegistrationAddress{Street: "Lindenweg 4", PostalCode: "49143", City: "Bissendorf"}, family.Guardian.Address)
	require.Len(t, family.Children, 1)

	byProfile := map[int64]parentService.HolidayRegistrationContact{}
	for _, contact := range family.Children[0].Contacts {
		byProfile[contact.GuardianProfileID] = contact
	}
	assert.Len(t, byProfile, 3, "co-parent, grandma and step-parent; not the caller, not the social worker")
	assert.NotContains(t, byProfile, chain.GuardianProfileID, "the caller fills in the own fields")
	assert.NotContains(t, byProfile, socialWorker.ID, "a social worker is no family contact")

	co := byProfile[coParent.ID]
	assert.True(t, co.FullGuardian, "a co-guardian is the second custodian")
	assert.Equal(t, "Jonas", co.FirstName)
	assert.Equal(t, *coParent.Email, co.Email)
	assert.Equal(t, "+49 5402 1234", co.Phone)

	oma := byProfile[grandma.ID]
	assert.False(t, oma.FullGuardian)
	assert.True(t, oma.Pickup)
	assert.True(t, oma.Emergency)
	assert.Equal(t, "relative", oma.RelationshipType)
	assert.Equal(t, "+49 5402 9876", oma.Phone)

	step := byProfile[stepParent.ID]
	assert.Equal(t, "Lena", step.FirstName)
	assert.Equal(t, *stepParent.Email, step.Email, "an own account does not hide the contact data from the registration")
	assert.Equal(t, "+49 5402 5555", step.Phone)

	// Without parent_portal.access the guardian list stays closed. The
	// account's child list already drops such a child, so the prefill offers
	// neither the child nor its contacts, even with the registration right.
	_, err = db.ExecContext(ctx, `UPDATE auth.guardian_student_access a SET permissions = '{"parent_portal.enrollment.submit": true}'::jsonb
		FROM users.student_guardian_relationships r
		WHERE a.tenant_id = r.tenant_id AND a.relationship_id = r.id AND r.student_id = ? AND r.guardian_profile_id = ?`, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)
	closed, err := svc.HolidayRegistrationFamily(ctx, chain.AccountID, parentService.HolidayRegistrationScope{OrganizationID: 42, SchoolIDs: []int64{chain.TenantID}})
	require.NoError(t, err)
	assert.Empty(t, closed.Children, "no contacts beyond what the guardian list shows")
}

// TestHolidayRegistrationFamilyOrdersContactsByEmergencyPriority pins the
// order of the contacts (#6682): the form keeps the first three emergency
// contacts, so the school's priority decides who they are, not the order
// the rows happen to come back in.
func TestHolidayRegistrationFamilyOrdersContactsByEmergencyPriority(t *testing.T) {
	t.Parallel()
	svc, db := buildHolidayRegistrationFamilyService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	names := []string{"Dritte", "Erste", "Zweite"}
	for i, priority := range []int{3, 1, 2} {
		profile := testpkg.CreateTestGuardianProfileNamed(t, db, names[i], "Kontakt", fmt.Sprintf("prio.%d", priority))
		testpkg.CreateTestStudentGuardianLinkOfType(t, db, chain.TenantID, chain.StudentID, profile.ID, "relative", "emergency_contact")
		_, err := db.ExecContext(ctx, `UPDATE users.student_guardian_relationships SET is_emergency_contact = true, emergency_priority = ? WHERE student_id = ? AND guardian_profile_id = ?`, priority, chain.StudentID, profile.ID)
		require.NoError(t, err)
	}

	family, err := svc.HolidayRegistrationFamily(ctx, chain.AccountID, parentService.HolidayRegistrationScope{OrganizationID: 42, SchoolIDs: []int64{chain.TenantID}})
	require.NoError(t, err)
	require.Len(t, family.Children, 1)
	var order []string
	for _, contact := range family.Children[0].Contacts {
		order = append(order, contact.FirstName)
	}
	assert.Equal(t, []string{"Erste", "Zweite", "Dritte"}, order)
}
