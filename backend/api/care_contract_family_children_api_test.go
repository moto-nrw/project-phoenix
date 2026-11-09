package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The children of the family through the one-time link of a contract mail
// and taken over from the registration with the contract (#6335).

type linkFamilyChildrenBody struct {
	StatedChildren *int `json:"stated_children"`
	CanState       bool `json:"can_state"`
}

// familyNumber reads the stated number of a family of the Träger and the
// contract it hangs on; found is false for no row.
func familyNumber(t *testing.T, f careContractFixture, organization int64, family string) (children *int, contract int64, found bool) {
	t.Helper()
	rows, err := f.db.QueryContext(context.Background(), `SELECT stated_children, care_contract_id FROM parent_billing.family_child_counts
		WHERE organization_id = ? AND family_email = ?`, organization, family)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, 0, false
	}
	require.NoError(t, rows.Scan(&children, &contract))
	return children, contract, true
}

func familyEventChannels(t *testing.T, f careContractFixture, contract int64) []string {
	t.Helper()
	var channels []string
	require.NoError(t, f.db.NewRaw(`SELECT action || ':' || COALESCE(channel, '') || ':' || COALESCE(actor_account_id::text, '')
		FROM parent_billing.family_child_count_events WHERE organization_id = ? AND care_contract_id = ? ORDER BY id`,
		f.office.organization, contract).Scan(context.Background(), &channels))
	return channels
}

// The link reads and states only the number of the family of its own
// contract, while it is open: a link of another Träger, a forged, an
// expired and a used link are refused and write nothing.
func TestFamilyChildrenThroughTheMailLinkStayInItsContractsFamily(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Kinderzahl Link")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	contract, link := sentCareContract(t, f, f.child)
	other := careContractApplication(t, f.db, f.school, f.phase, f.offering, "Ole", "olga.andere@example.org")
	otherContract, otherLink := sentCareContract(t, f, other)

	rec := publicSigning(t, f.traeger, http.MethodGet, link+"/family-children", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	read := decodeSchoolPortalData[linkFamilyChildrenBody](t, rec.Body.Bytes())
	assert.True(t, read.CanState)
	assert.Nil(t, read.StatedChildren)

	rec = publicSigning(t, f.traeger, http.MethodPut, link+"/family-children", map[string]any{"children": 3})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	stated := decodeSchoolPortalData[linkFamilyChildrenBody](t, rec.Body.Bytes())
	require.NotNil(t, stated.StatedChildren)
	assert.Equal(t, 3, *stated.StatedChildren)
	children, on, found := familyNumber(t, f, f.office.organization, "erika.muster@example.org")
	require.True(t, found)
	assert.Equal(t, 3, *children)
	assert.Equal(t, contract, on, "the number hangs on the link's contract")
	_, _, found = familyNumber(t, f, f.office.organization, "olga.andere@example.org")
	assert.False(t, found, "the other family has no number")
	assert.Equal(t, []string{"read:link:", "state:link:"}, familyEventChannels(t, f, contract),
		"the trail names the link and no account")

	for _, body := range []map[string]any{{"children": 0}, {"children": 21}} {
		rec = publicSigning(t, f.traeger, http.MethodPut, link+"/family-children", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "care_contract.field_invalid")
	}

	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Kinderzahl Link Fremd", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCareContracts(t, f.db, foreign.organization)
	_, token, _ := strings.Cut(link, ".")
	for name, forged := range map[string]string{
		"other Träger": fmt.Sprintf("%d.%s", foreign.organization, token),
		"forged token": fmt.Sprintf("%d.%s", f.office.organization, strings.Repeat("A", len(token))),
		"no Träger":    token,
	} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			rec = publicSigning(t, f.traeger, method, forged+"/family-children", map[string]any{"children": 5})
			assert.Equal(t, http.StatusGone, rec.Code, "%s %s: %s", name, method, rec.Body.String())
			assert.Contains(t, rec.Body.String(), "care_contract.signing_link_invalid", name)
		}
	}
	_, _, found = familyNumber(t, f, foreign.organization, "erika.muster@example.org")
	assert.False(t, found, "the other Träger has no number")

	_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_signing_links
		SET created_at = NOW() - INTERVAL '40 days', expires_at = NOW() - INTERVAL '1 day' WHERE contract_id = ?`, otherContract)
	require.NoError(t, err)
	rec = publicSigning(t, f.traeger, http.MethodPut, otherLink+"/family-children", map[string]any{"children": 4})
	assert.Equal(t, http.StatusGone, rec.Code, "an expired link: %s", rec.Body.String())
	_, _, found = familyNumber(t, f, f.office.organization, "olga.andere@example.org")
	assert.False(t, found)

	rec = publicSigning(t, f.traeger, http.MethodPost, link+"/sign",
		map[string]any{"method": "drawn", "image_png": signatureImage(t), "withdrawal_notice_acknowledged": true})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = publicSigning(t, f.traeger, http.MethodPut, link+"/family-children", map[string]any{"children": 6})
	assert.Equal(t, http.StatusGone, rec.Code, "the signature closes the link for the number too: %s", rec.Body.String())
	children, _, _ = familyNumber(t, f, f.office.organization, "erika.muster@example.org")
	assert.Equal(t, 3, *children)
}

// The registration's number reaches the Elternabrechnung with the contract
// of its accepted child, keyed by the contract's billing recipient. An
// older registration never replaces a newer statement of the family.
func TestFamilyChildrenOfTheRegistrationComeWithTheContract(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Kinderzahl Anmeldung")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	stateOnRegistration := func(child int64, children int, ago string) {
		_, err := f.db.ExecContext(context.Background(), `UPDATE enrollment.requests SET family_children = ?,
			family_children_stated_at = NOW() - ?::interval
			WHERE id = (SELECT request_id FROM enrollment.request_children WHERE id = ?)`, children, ago, child)
		require.NoError(t, err)
	}
	stateOnRegistration(f.child, 2, "3 days")

	contract, link := sentCareContract(t, f, f.child)
	children, on, found := familyNumber(t, f, f.office.organization, "erika.muster@example.org")
	require.True(t, found, "the contract took the registration's number over")
	assert.Equal(t, 2, *children)
	assert.Equal(t, contract, on)
	assert.Equal(t, []string{"state:enrollment:"}, familyEventChannels(t, f, contract), "the trail names the registration")

	rec := publicSigning(t, f.traeger, http.MethodPut, link+"/family-children", map[string]any{"children": 4})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// A sibling's older registration of the same family comes later.
	sibling := careContractApplication(t, f.db, f.school, f.phase, f.offering, "Ben", "Erika.Muster@example.org")
	stateOnRegistration(sibling, 1, "2 days")
	siblingContract, _ := sentCareContract(t, f, sibling)
	children, on, _ = familyNumber(t, f, f.office.organization, "erika.muster@example.org")
	assert.Equal(t, 4, *children, "the older registration does not replace the newer statement")
	assert.Equal(t, contract, on)
	assert.Empty(t, familyEventChannels(t, f, siblingContract), "nothing was taken over, nothing recorded")

	// Without a statement on the registration nothing is taken over.
	third := careContractApplication(t, f.db, f.school, f.phase, f.offering, "Pia", "pia.ohne@example.org")
	sentCareContract(t, f, third)
	_, _, found = familyNumber(t, f, f.office.organization, "pia.ohne@example.org")
	assert.False(t, found)
}

// The registration asks for the children of the family through the root's
// binding only where a care sibling rule of the school's Träger counts them
// for that school (#6335): a rule that counts only contracts asks nothing,
// and neither does a school the rule does not name.
func TestRegistrationAsksWhereTheTraegersRuleCountsTheFamily(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Kinderzahl Anmeldung fragen")
	school := f.office.schools[0]
	purpose := owners.NewEnrollmentFamilyChildrenPurpose(NewOrganizationTestRows(testpkg.NewDatabase(t, f.db)).School)
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, f.db))
	asks := func(schoolID int64) bool {
		t.Helper()
		asked, err := purpose.FamilyChildrenAsked(ctx, schoolID)
		require.NoError(t, err)
		return asked
	}

	f.saveRule(t, careRule("sibling", "Geschwister nur Verträge", []int64{school},
		map[string]any{"sibling_from_child": 2, "discount_basis_points": 5000}))
	assert.False(t, asks(school), "a rule that counts only contracts")

	other := testpkg.CreateTestSchoolInOrganization(t, f.db, f.office.organization, "OGS mit Familienregel")
	f.saveRule(t, careRule("sibling", "Geschwister mit Familie", []int64{other},
		map[string]any{"sibling_from_child": 2, "discount_basis_points": 5000, "counts_family_children": true}))
	assert.True(t, asks(other), "a rule that counts the family at the school")
	assert.False(t, asks(school), "a school the counting rule does not name")
}
