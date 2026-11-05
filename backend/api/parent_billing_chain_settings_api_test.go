// Router-level tests for the choices of a Träger in the holiday contribution
// (#5664, ADR 0115 addendum): which child is the first child and whether a
// cost bearer pays before the sibling discount. The preview, the public quote
// and the confirmation use the same choice, a later change reaches no frozen
// contribution, the right "Einstellungen" guards them and no other Träger is
// affected.
package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

func setHolidayChoice(t *testing.T, f reviewFixture, key, value string) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, "/traeger/settings/values/"+key, map[string]any{"value": value})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// submitYoungerDearer sends Lia (born 2018, week 1 "bis 16 Uhr", 67,50 €) and
// Ben (born 2016, week 1 "bis 13 Uhr", 45,00 €): the younger child is dearer.
func (f reviewFixture) submitYoungerDearer(t *testing.T, email string) int64 {
	t.Helper()
	body := siblingsRegistration(f.offer, email)
	ben := body["children"].([]map[string]any)[1]
	ben["bookings"] = []map[string]any{{"week_id": f.offer.Weeks[0].ID, "time_model_id": f.offer.Weeks[0].TimeModels[0].ID}}
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID
}

func TestOldestChildIsTheFirstChildInPreviewQuoteAndConfirmation(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Rang Alter")
	createContributionRule(t, f, traegerColleague(t, f, identityaccess.TraegerRoleBilling), siblingHalf())
	id := f.submitYoungerDearer(t, "rang.alter@example.org")

	standard := f.contribution(t, id)
	assert.Equal(t, []string{"base:6750"}, lineKinds(standard.Children[0].Lines), "by default the dearer child pays in full")
	assert.Equal(t, []string{"base:4500", "sibling:-2250"}, lineKinds(standard.Children[1].Lines))

	setHolidayChoice(t, f, configModels.KeyParentBillingSiblingRank, "oldest_child")
	preview := f.contribution(t, id)
	assert.Equal(t, []string{"base:6750", "sibling:-3375"}, lineKinds(preview.Children[0].Lines), "Lia is younger")
	assert.Equal(t, []string{"base:4500"}, lineKinds(preview.Children[1].Lines), "Ben is older and pays in full")

	quote := func(lia, ben int) contributionBody {
		children := []map[string]any{
			{"bookings": []map[string]any{{"week_id": f.offer.Weeks[0].ID, "time_model_id": f.offer.Weeks[0].TimeModels[1].ID}}, "age_rank": lia},
			{"bookings": []map[string]any{{"week_id": f.offer.Weeks[0].ID, "time_model_id": f.offer.Weeks[0].TimeModels[0].ID}}, "age_rank": ben},
		}
		rec := publicJSON(t, f.traeger, http.MethodPost, holidayQuotePath(f.period.PublicToken), map[string]any{"children": children})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[contributionBody](t, rec.Body.Bytes())
	}
	assert.Equal(t, preview.TotalCents, quote(2, 1).TotalCents, "the form shows what the confirmation freezes")
	assert.Equal(t, int64(6750+2250), quote(1, 2).TotalCents)

	rec := f.confirmWith(t, id, map[string]any{})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, preview.TotalCents, f.contribution(t, id).TotalCents)

	setHolidayChoice(t, f, configModels.KeyParentBillingSiblingRank, "highest_contribution")
	assert.Equal(t, preview.TotalCents, f.contribution(t, id).TotalCents, "a later change reaches no frozen contribution")

	foreign := newReviewFixture(t, "Träger Rang Fremd")
	createContributionRule(t, foreign, traegerColleague(t, foreign, identityaccess.TraegerRoleBilling), siblingHalf())
	setHolidayChoice(t, f, configModels.KeyParentBillingSiblingRank, "oldest_child")
	other := foreign.contribution(t, foreign.submitYoungerDearer(t, "rang.fremd@example.org"))
	assert.Equal(t, []string{"base:6750"}, lineKinds(other.Children[0].Lines), "the choice of one Träger does not reach another")
}

func TestGroupWithAFrozenContributionKeepsItsChoiceAfterAChange(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Rang Gruppe")
	createContributionRule(t, f, traegerColleague(t, f, identityaccess.TraegerRoleBilling), siblingHalf())
	dearer := f.submitChild(t, f.period, f.offer, "gruppe.eins@example.org", "Mara", 1)
	cheaper := f.submitChild(t, f.period, f.offer, "gruppe.zwei@example.org", "Tom", 0)
	rec := f.merge(t, f.office.token, dearer, cheaper)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = f.confirmWith(t, dearer, map[string]any{})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	setHolidayChoice(t, f, configModels.KeyParentBillingSiblingRank, "oldest_child")
	setHolidayChoice(t, f, configModels.KeyParentBillingCostTakeoverOrder, "before_discount")
	preview := f.contribution(t, cheaper)
	assert.Equal(t, []string{"base:4500", "sibling:-2250"}, lineKinds(preview.Children[0].Lines), "the group ranks as it froze")
	rec = f.confirmWith(t, cheaper, map[string]any{})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []int64{6750}, f.contributionVersions(t, dearer), "confirming a sibling changes the frozen contribution of the other")
	assert.Equal(t, []int64{2250}, f.contributionVersions(t, cheaper))
}

func TestCostBearerPaysTheFullPriceBeforeTheDiscountAcrossAGroup(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Vor Rabatt")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())
	office := createCostBearer(t, f, billing, youthOffice())
	setHolidayChoice(t, f, configModels.KeyParentBillingCostTakeoverOrder, "before_discount")
	dearer := f.submitChild(t, f.period, f.offer, "vor.eins@example.org", "Mara", 1)
	cheaper := f.submitChild(t, f.period, f.offer, "vor.zwei@example.org", "Tom", 0)
	rec := f.merge(t, f.office.token, dearer, cheaper)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = f.takeoverPreview(t, cheaper, fmt.Sprintf("?cost_bearer_id=%d", office.ID))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[contributionBody](t, rec.Body.Bytes())
	assert.Equal(t, []string{"base:4500", "cost_takeover:-4500"}, lineKinds(preview.Children[0].Lines),
		"the second child of the group: the cost bearer pays the full price, no discount")

	rec = f.confirmWith(t, cheaper, map[string]any{"cost_bearer_id": office.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	frozen := f.contribution(t, cheaper)
	assert.Equal(t, []string{"base:4500", "cost_takeover:-4500"}, lineKinds(frozen.Children[0].Lines), "the confirmation freezes what the preview showed")
	assert.Equal(t, int64(6750), f.contribution(t, dearer).TotalCents, "the first child pays in full")
}

func TestDiscountOfTheOtherChildStaysWhenTheFirstChildHasACostBearer(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Vor Rabatt Erstes")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())
	office := createCostBearer(t, f, billing, youthOffice())
	setHolidayChoice(t, f, configModels.KeyParentBillingCostTakeoverOrder, "before_discount")
	dearer := f.submitChild(t, f.period, f.offer, "erstes.eins@example.org", "Mara", 1)
	cheaper := f.submitChild(t, f.period, f.offer, "erstes.zwei@example.org", "Tom", 0)
	rec := f.merge(t, f.office.token, dearer, cheaper)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = f.confirmWith(t, dearer, map[string]any{"cost_bearer_id": office.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, []string{"base:6750", "cost_takeover:-6750"}, lineKinds(f.contribution(t, dearer).Children[0].Lines))
	assert.Equal(t, []string{"base:4500", "sibling:-2250"}, lineKinds(f.contribution(t, cheaper).Children[0].Lines),
		"the child of the cost bearer still counts as a sibling")
}

func TestHolidayChoicesNeedTheSettingsRightAndAKnownValue(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Rang Recht")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	for key, valid := range map[string]string{
		configModels.KeyParentBillingSiblingRank:       "oldest_child",
		configModels.KeyParentBillingCostTakeoverOrder: "before_discount",
	} {
		rec := traegerJSON(t, f.traeger, billing, http.MethodPut, "/traeger/settings/values/"+key, map[string]any{"value": valid})
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s needs the right Einstellungen: %s", key, rec.Body.String())
		rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, "/traeger/settings/values/"+key, map[string]any{"value": "unbekannt"})
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s refuses an unknown value: %s", key, rec.Body.String())
	}
}
