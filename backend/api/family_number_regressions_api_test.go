package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestMergingConfirmedSiblingsRemembersInheritedFamilyNumber(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Familie Zusammenführen")
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "20815"}).Code)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.again, nil).Code)
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, f.siblings, f.again).Code)
	assert.Equal(t, "20815", f.debtorNumberDetail(t, f.again).DebtorNumber)
	assert.Equal(t, "20815", f.familyNumber(t, "papa.muster@example.org"), "the family remembers the inherited group number before any invoice")
	assert.Equal(t, "20815", f.proposedNumber(t, f.submitFamily(t, "papa.muster@example.org")))
}

func TestKeepingFamilyNumberRefreshesHolidayRetention(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Familie Frist behalten")
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "20815"}).Code)
	_, err := f.db.ExecContext(context.Background(), `UPDATE parent_billing.family_billing SET last_used_on = '2020-01-01'
  WHERE organization_id = ? AND family_email = 'eva.muster@example.org'`, f.office.organization)
	require.NoError(t, err)
	next := f.submitFamily(t, "eva.muster@example.org")
	rec := f.confirmWith(t, next, map[string]any{"debtor_number": "30100", "family_debtor_number": "keep"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var family struct {
		Number string        `bun:"debtor_number"`
		Origin int64         `bun:"debtor_number_registration_id"`
		Used   calendar.Date `bun:"last_used_on,type:date"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT debtor_number, debtor_number_registration_id, last_used_on
  FROM parent_billing.family_billing WHERE organization_id = ? AND family_email = 'eva.muster@example.org'`, f.office.organization).Scan(context.Background(), &family))
	want := calendar.NewDate(2027, time.January, 15)
	assert.Equal(t, "20815", family.Number)
	assert.Equal(t, f.siblings, family.Origin, "keep preserves the remembered number's origin")
	// Today may be later than the booked days; retention must cover both.
	assert.False(t, family.Used.Before(want), "keep must not retain less than the latest booked day")
}

func TestHolidayOnlyUsesActualContractsWithAndWithoutDocuments(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Familie Vertragszuordnung")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	testpkg.OwnTraegerCareContracts(t, f.db, f.office.organization)
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Lia", careMonth)
	_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_parties SET email = 'EVA.Muster@Example.org' WHERE contract_id = ?`, contract)
	require.NoError(t, err)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Vertrag Fremd", identityaccess.TraegerRoleBilling)
	testpkg.OwnTraegerCareContracts(t, f.db, foreign.organization)
	other := careContract(t, f.db, foreign.organization, foreign.schools[0], "Tom", careMonth)
	_, err = f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_parties SET email = 'papa.muster@example.org' WHERE contract_id = ?`, other)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "20815"}).Code)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.again, map[string]any{"debtor_number": "30100"}).Code)
	list := func() {
		t.Helper()
		rec := traegerJSON(t, f.traeger, billing, http.MethodGet, billingFamiliesPath, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var own, holiday bool
		for _, row := range decodeSchoolPortalData[[]struct {
			ID          int64  `json:"id"`
			FamilyEmail string `json:"family_email"`
			HolidayOnly bool   `json:"holiday_only"`
		}](t, rec.Body.Bytes()) {
			switch row.FamilyEmail {
			case "eva.muster@example.org":
				own = true
				assert.False(t, row.HolidayOnly, "a contract counts before any monthly invoice or deviation")
			case "papa.muster@example.org":
				holiday = true
				assert.True(t, row.HolidayOnly, "a foreign contract does not count")
			}
			if row.ID != 0 {
				detail := traegerJSON(t, f.traeger, billing, http.MethodGet, fmt.Sprintf("%s/%d", billingFamiliesPath, row.ID), nil)
				require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())
				body := decodeSchoolPortalData[struct {
					HolidayOnly bool `json:"holiday_only"`
				}](t, detail.Body.Bytes())
				assert.Equal(t, row.HolidayOnly, body.HolidayOnly, "detail and list use the same actual membership")
			}
		}
		assert.True(t, own)
		assert.True(t, holiday)
	}
	list()
	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	list()
}

func TestMergingSiblingsKeepsAnIndependentlyRememberedNumber(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Familie Zusammenführen behalten")
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "30100"}).Code)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.again, map[string]any{"debtor_number": "20815"}).Code)
	require.Equal(t, http.StatusOK, f.changeDebtorNumber(t, f.office.token, f.siblings, "").Code)
	rec := f.merge(t, f.office.token, f.siblings, f.again)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "20815", f.debtorNumberDetail(t, f.siblings).DebtorNumber, "the group adopts one number")
	assert.Equal(t, "30100", f.familyNumber(t, "eva.muster@example.org"), "a merge does not authorize replacing the remembered number")
}
