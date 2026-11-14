package api

import (
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
