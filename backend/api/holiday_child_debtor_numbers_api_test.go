package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-ogs/project-phoenix/modules/settings/records"
)

func holidayFamilyChildren(t *testing.T, f reviewFixture, family string) []listedChild {
	t.Helper()
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	rec := traegerJSON(t, f.traeger, billing, http.MethodGet, "/traeger/billing-families/children", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return childrenOf(decodeSchoolPortalData[[]childListedFamily](t, rec.Body.Bytes()), family)
}

func TestHolidayRepeatedChildInSiblingGroupGetsOneNumber(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Wiederholtes Ferienkind")
	const family = "wiederholt@example.org"
	first := f.submitChild(t, f.period, f.offer, family, "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, family, "MIA", 0)
	for _, id := range []int64{first, second} {
		rec := f.confirmWith(t, id, map[string]any{"debtor_number": "70001"})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	setTraegerValue(t, f, configModels.KeyParentBillingDebtorNumberScope, "child")
	rec := f.merge(t, f.office.token, first, second)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	children := holidayFamilyChildren(t, f, family)
	require.Len(t, children, 1, "normalized keys identify one child across registrations")
	assert.Equal(t, "70001", children[0].DebtorNumber)
}

func TestHolidaySiblingGroupDoesNotTeachChildrenItsSharedNumber(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"confirm", "change", "merge"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			f := newReviewFixture(t, "Gemeinsame Geschwisternummer "+action)
			const family = "geschwister@example.org"
			first := f.submitChild(t, f.period, f.offer, family, "Mia", 0)
			second := f.submitChild(t, f.period, f.offer, family, "Noah", 0)
			rec := f.confirmWith(t, first, map[string]any{"debtor_number": "70001"})
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			if action != "confirm" {
				rec = f.confirmWith(t, second, map[string]any{"debtor_number": "70001"})
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			}
			if action != "merge" {
				rec = f.merge(t, f.office.token, first, second)
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			}
			setTraegerValue(t, f, configModels.KeyParentBillingDebtorNumberScope, "child")
			switch action {
			case "confirm":
				rec = f.confirmWith(t, second, map[string]any{"debtor_number": "70001"})
			case "change":
				rec = f.changeDebtorNumber(t, f.office.token, first, "70002")
			case "merge":
				rec = f.merge(t, f.office.token, first, second)
			}
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			children := holidayFamilyChildren(t, f, family)
			require.Len(t, children, 2)
			for _, child := range children {
				assert.Empty(t, child.DebtorNumber, "a shared group number names no single child")
			}
		})
	}
}

func TestHolidayFirstConfirmedSiblingDoesNotLearnTheGroupNumber(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Erste Bestätigung der Geschwister")
	const family = "erste-geschwister@example.org"
	first := f.submitChild(t, f.period, f.offer, family, "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, family, "Noah", 0)
	setTraegerValue(t, f, configModels.KeyParentBillingDebtorNumberScope, "child")
	rec := f.merge(t, f.office.token, first, second)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = f.confirmWith(t, first, map[string]any{"debtor_number": "70001"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	children := holidayFamilyChildren(t, f, family)
	require.Len(t, children, 1, "only the confirmed child is remembered")
	assert.Empty(t, children[0].DebtorNumber, "an unconfirmed sibling also makes the group number ambiguous")
}

func TestHolidayChildrenAreEditableAfterSwitchingFromPayerToChild(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Ferienkinder nach Umstellung")
	rec := f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "70001"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	setTraegerValue(t, f, configModels.KeyParentBillingDebtorNumberScope, "child")
	children := holidayFamilyChildren(t, f, "eva.muster@example.org")
	require.Len(t, children, 2, "confirmed holiday children stay known across scope changes")
	for _, child := range children {
		assert.Empty(t, child.DebtorNumber, "the payer number names neither child")
	}
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, childDebtorNumberPath, map[string]any{
		"family_email": "eva.muster@example.org", "child_first_name": "Lia", "child_last_name": "Muster",
		"child_birth_date": "2018-03-01", "debtor_number": "70002"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "70001", f.familyNumber(t, "eva.muster@example.org"), "the payer number stays unchanged")
}

func TestHolidayPreservedChildNumberKeepsItsLaterBookingDate(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Ferienkind spätere Buchung")
	setTraegerValue(t, f, configModels.KeyParentBillingDebtorNumberScope, "child")
	first := f.submitChild(t, f.period, f.offer, "papa.muster@example.org", "Lia", 0)
	rec := f.confirmWith(t, first, map[string]any{"debtor_number": "80001"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, err := f.db.ExecContext(context.Background(), `UPDATE parent_billing.child_debtor_numbers SET last_used_on = '2020-01-01' WHERE organization_id = ?`, f.office.organization)
	require.NoError(t, err)
	rec = f.confirmWith(t, f.again, map[string]any{"debtor_number": "90001"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	children := holidayFamilyChildren(t, f, "papa.muster@example.org")
	require.Len(t, children, 1)
	assert.Equal(t, "80001", children[0].DebtorNumber, "a shared registration number never replaces an individual number")
	var keptThroughBooking bool
	require.NoError(t, f.db.NewRaw(`SELECT number.last_used_on >= (SELECT max(ends_on) FROM enrollment.holiday_period_weeks WHERE holiday_period_id = ?)
 FROM parent_billing.child_debtor_numbers AS number WHERE organization_id = ?`, f.period.ID, f.office.organization).Scan(context.Background(), &keptThroughBooking))
	assert.True(t, keptThroughBooking, "the preserved number remains needed through the booked holiday period")
}
