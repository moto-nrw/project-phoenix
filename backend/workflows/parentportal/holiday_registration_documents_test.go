package parentportal_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	parentService "github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

// TestDocumentInboxOpensTheTraegerOfTheAccountsHolidayRegistration pins ADR
// 0216 point 9 on the inbox of ADR 0126: an account without a school opens
// the inbox of a Träger at which it has a linked holiday registration, and
// still sees only the documents addressed to it. An account with neither a
// permitted child nor a linked registration stays refused; a Träger without
// the account's registration stays closed, and another account's linked
// registration opens nothing for it.
func TestDocumentInboxOpensTheTraegerOfTheAccountsHolidayRegistration(t *testing.T) {
	t.Parallel()
	portal, _, _, db := documentInbox(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	newTraeger := func(name string) (int64, int64) {
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
			Name: name, Slug: fmt.Sprintf("ferien-postfach-%d", testpkg.UniqueTestTenantID(t)), Active: true,
		}).ID
		return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	}
	own, ownSchool := newTraeger("Träger Ferienpostfach")
	foreign, foreignSchool := newTraeger("Träger Fremdes Ferienpostfach")
	eva := testpkg.CreateTestAccount(t, db, "ferien-postfach-eva")
	tom := testpkg.CreateTestAccount(t, db, "ferien-postfach-tom")
	nobody := testpkg.CreateTestAccount(t, db, "ferien-postfach-niemand")
	link := func(organization, school int64, account *testpkg.AccountFixture) {
		registration := testpkg.CreateTestHolidayRegistration(t, db, organization, school, "Herbstferien 2029", account.Email, "Lia")
		_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET parent_account_id = ? WHERE id = ?`, account.ID, registration.RegistrationID)
		require.NoError(t, err)
	}
	link(own, ownSchool, eva)
	link(foreign, foreignSchool, tom)

	ownLetters := newInboxTraeger(t, db, own)
	letter := ownLetters.document(eva.Email, "Lia", true)
	otherFamily := ownLetters.document(tom.Email, "Tim", true)
	foreignLetter := newInboxTraeger(t, db, foreign).document(eva.Email, "Lia", true)

	documents, err := portal.ListBillingDocuments(ctx, eva.ID, parentService.DocumentInboxLetters)
	require.NoError(t, err, "the linked registration opens the inbox of its Träger")
	require.Len(t, documents, 1, "only the letter to the account's address at the Träger of its registration")
	assert.Equal(t, letter, documents[0].ID)
	assert.Equal(t, own, documents[0].OrganizationID)
	assert.Equal(t, "Träger Ferienpostfach", documents[0].TraegerName)
	file, err := portal.BillingDocumentLetter(ctx, eva.ID, own, letter)
	require.NoError(t, err)
	assert.NotEmpty(t, file.Data)
	for name, request := range map[string]struct{ organization, document int64 }{
		"letter of another family":                    {own, otherFamily},
		"letter of a Träger without the registration": {foreign, foreignLetter},
	} {
		_, err := portal.BillingDocumentLetter(ctx, eva.ID, request.organization, request.document)
		assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound, name)
	}
	contracts, err := portal.ListBillingDocuments(ctx, eva.ID, parentService.DocumentInboxContracts)
	require.NoError(t, err, "the contract list opens with the letters")
	assert.Empty(t, contracts)

	tomDocuments, err := portal.ListBillingDocuments(ctx, tom.ID, parentService.DocumentInboxLetters)
	require.NoError(t, err)
	assert.Empty(t, tomDocuments, "another account's registration elsewhere opens no letter of this Träger")

	_, err = portal.ListBillingDocuments(ctx, nobody.ID, parentService.DocumentInboxLetters)
	assert.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied, "neither a permitted child nor a linked registration")
	_, err = portal.BillingDocumentLetter(ctx, nobody.ID, own, letter)
	assert.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)
}
