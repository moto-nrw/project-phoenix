// Router-level tests for the Elternabrechnung (#4664, ADR 0115): the
// contribution rules of a Träger behind the right "Abrechnung", the public
// contribution of the holiday registration form, the contribution of a
// registration and its confirmation, which freezes it, the exemption with
// its note, the rights and the Träger boundary.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

const contributionRulesPath = "/traeger/contribution-rules"

type contributionRuleBody struct {
	ID                  int64   `json:"id"`
	Kind                string  `json:"kind"`
	ValidFrom           string  `json:"valid_from"`
	ValidUntil          *string `json:"valid_until"`
	SiblingFromChild    *int    `json:"sibling_from_child"`
	DiscountBasisPoints *int    `json:"discount_basis_points"`
	DiscountCents       *int64  `json:"discount_cents"`
	Note                string  `json:"note"`
}

type contributionLineBody struct {
	Kind                string  `json:"kind"`
	AmountCents         int64   `json:"amount_cents"`
	WeekStartsOn        *string `json:"week_starts_on"`
	TimeModelName       string  `json:"time_model_name"`
	ChildRank           *int    `json:"child_rank"`
	DiscountBasisPoints *int    `json:"discount_basis_points"`
	Note                string  `json:"note"`
}

type contributionBody struct {
	Children []struct {
		ChildID    int64                  `json:"child_id"`
		Position   int                    `json:"position"`
		Lines      []contributionLineBody `json:"lines"`
		TotalCents int64                  `json:"total_cents"`
	} `json:"children"`
	TotalCents         int64   `json:"total_cents"`
	ExemptionAvailable bool    `json:"exemption_available"`
	BenefitsExemption  bool    `json:"benefits_exemption"`
	Frozen             bool    `json:"frozen"`
	ConfirmedAt        *string `json:"confirmed_at"`
}

type confirmationBody struct {
	ID           int64            `json:"id"`
	Status       string           `json:"status"`
	Contribution contributionBody `json:"contribution"`
}

func holidayContributionPath(periodID, registrationID int64) string {
	return fmt.Sprintf("/traeger/holiday-contributions/%d/registrations/%d", periodID, registrationID)
}

func holidayConfirmPath(periodID, registrationID int64) string {
	return holidayContributionPath(periodID, registrationID) + "/confirm"
}

func holidayQuotePath(token string) string { return "/traeger/holiday-quotes/" + token }

// traegerColleague signs in a further person of the Träger-Büro of office
// with role and returns the session's token.
func traegerColleague(t *testing.T, f reviewFixture, role string) string {
	t.Helper()
	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, role)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	return answer.AccessToken
}

func createContributionRule(t *testing.T, f reviewFixture, token string, body map[string]any) contributionRuleBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, contributionRulesPath, body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[contributionRuleBody](t, rec.Body.Bytes())
}

func siblingHalf() map[string]any {
	return map[string]any{"kind": "sibling", "valid_from": "2027-01-01", "sibling_from_child": 2, "discount_basis_points": 5000}
}

func benefitsExemption() map[string]any {
	return map[string]any{"kind": "benefits_exemption", "valid_from": "2027-01-01", "note": "Beitragsfrei wegen Leistungsbezug (BuT)"}
}

// quoteChildren are the bookings of siblingsRegistration: Lia books week 1
// "bis 16 Uhr" (67,50 €), Ben both weeks "bis 13 Uhr" (2 × 45,00 €).
func quoteChildren(offer holidayOfferBody) []map[string]any {
	return []map[string]any{
		{"bookings": []map[string]any{{"week_id": offer.Weeks[0].ID, "time_model_id": offer.Weeks[0].TimeModels[1].ID}}},
		{"bookings": []map[string]any{
			{"week_id": offer.Weeks[0].ID, "time_model_id": offer.Weeks[0].TimeModels[0].ID},
			{"week_id": offer.Weeks[1].ID, "time_model_id": offer.Weeks[1].TimeModels[0].ID},
		}},
	}
}

func (f reviewFixture) contribution(t *testing.T, registrationID int64) contributionBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayContributionPath(f.period.ID, registrationID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[contributionBody](t, rec.Body.Bytes())
}

func lineKinds(lines []contributionLineBody) []string {
	kinds := make([]string, len(lines))
	for i, line := range lines {
		kinds[i] = fmt.Sprintf("%s:%d", line.Kind, line.AmountCents)
	}
	return kinds
}

func TestContributionRulesAreKeptPerTraegerBehindTheBillingRight(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Regeln Beitrag")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Regeln Fremd", identityaccess.TraegerRoleBilling)

	sibling := createContributionRule(t, f, billing, siblingHalf())
	assert.Equal(t, "sibling", sibling.Kind)
	assert.Equal(t, 5000, *sibling.DiscountBasisPoints)
	assert.Nil(t, sibling.ValidUntil)
	exemption := createContributionRule(t, f, billing, benefitsExemption())
	assert.Equal(t, "Beitragsfrei wegen Leistungsbezug (BuT)", exemption.Note)

	overlap := siblingHalf()
	overlap["valid_from"] = "2027-06-01"
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, contributionRulesPath, overlap)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.contribution_rule_overlap", code, "two sibling rules may not be valid at the same time")

	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, contributionRulesPath,
		map[string]any{"kind": "sibling", "valid_from": "2027-01-01", "sibling_from_child": 4, "discount_basis_points": 5000, "discount_cents": 1000})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.contribution_rule_invalid", code)
	assert.ElementsMatch(t, []string{"sibling_from_child", "discount_cents"}, fields)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, contributionRulesPath, map[string]any{"kind": "benefits_exemption", "valid_from": "2027-01-01"})
	require.Equal(t, http.StatusBadRequest, rec.Code, "an exemption needs its note: %s", rec.Body.String())

	limited := siblingHalf()
	limited["valid_until"], limited["discount_basis_points"] = "2027-05-31", nil
	limited["discount_cents"] = 1500
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, fmt.Sprintf("%s/%d", contributionRulesPath, sibling.ID), limited)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	changed := decodeSchoolPortalData[contributionRuleBody](t, rec.Body.Bytes())
	assert.Nil(t, changed.DiscountBasisPoints)
	assert.Equal(t, int64(1500), *changed.DiscountCents)
	require.NotNil(t, changed.ValidUntil)
	createContributionRule(t, f, billing, overlap)

	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, contributionRulesPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Len(t, decodeSchoolPortalData[[]contributionRuleBody](t, rec.Body.Bytes()), 3)

	// Another Träger sees none of these rules and cannot change them.
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, contributionRulesPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[[]contributionRuleBody](t, rec.Body.Bytes()))
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		rec = traegerJSON(t, f.traeger, foreign.token, method, fmt.Sprintf("%s/%d", contributionRulesPath, exemption.ID), benefitsExemption())
		require.Equal(t, http.StatusNotFound, rec.Code, "%s of a foreign rule: %s", method, rec.Body.String())
	}
	// The administration confirms registrations but keeps no rules.
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec = traegerJSON(t, f.traeger, f.office.token, method, contributionRulesPath, benefitsExemption())
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s without the right: %s", method, rec.Body.String())
	}

	rec = traegerJSON(t, f.traeger, billing, http.MethodDelete, fmt.Sprintf("%s/%d", contributionRulesPath, exemption.ID), nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, billing, http.MethodDelete, fmt.Sprintf("%s/%d", contributionRulesPath, exemption.ID), nil)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestPublicHolidayFormShowsTheContributionWithItsLines(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Zusammenfassung Beitrag")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)

	quote := func(body map[string]any) *httptest.ResponseRecorder {
		return publicJSON(t, f.traeger, http.MethodPost, holidayQuotePath(f.period.PublicToken), body)
	}
	rec := quote(map[string]any{"children": quoteChildren(f.offer)})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	plain := decodeSchoolPortalData[contributionBody](t, rec.Body.Bytes())
	assert.Equal(t, int64(6750+9000), plain.TotalCents, "without rules the contribution is the sum of the time models")
	assert.False(t, plain.ExemptionAvailable)
	require.Len(t, plain.Children, 2)
	assert.Equal(t, []int64{f.offer.Weeks[0].TimeModels[1].PriceCents, f.offer.Weeks[0].TimeModels[0].PriceCents, f.offer.Weeks[1].TimeModels[0].PriceCents},
		[]int64{plain.Children[0].Lines[0].AmountCents, plain.Children[1].Lines[0].AmountCents, plain.Children[1].Lines[1].AmountCents},
		"the price the form shows at each time model is the base line of the summary (#5237)")

	createContributionRule(t, f, billing, siblingHalf())
	createContributionRule(t, f, billing, benefitsExemption())
	rec = quote(map[string]any{"children": quoteChildren(f.offer)})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Muster", "the quote carries no personal data")
	siblings := decodeSchoolPortalData[contributionBody](t, rec.Body.Bytes())
	require.Len(t, siblings.Children, 2)
	assert.Equal(t, []string{"base:6750", "sibling:-3375"}, lineKinds(siblings.Children[0].Lines),
		"Ben has the higher base price, so Lia is the second child and pays half")
	assert.Equal(t, "bis 16 Uhr", siblings.Children[0].Lines[0].TimeModelName)
	assert.Equal(t, "2027-01-04", *siblings.Children[0].Lines[0].WeekStartsOn)
	assert.Equal(t, 2, *siblings.Children[0].Lines[1].ChildRank)
	assert.Equal(t, []string{"base:4500", "base:4500"}, lineKinds(siblings.Children[1].Lines))
	assert.Equal(t, int64(3375+9000), siblings.TotalCents)
	assert.True(t, siblings.ExemptionAvailable)
	assert.False(t, siblings.BenefitsExemption)

	rec = quote(map[string]any{"children": quoteChildren(f.offer), "benefits_declared": true})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	freed := decodeSchoolPortalData[contributionBody](t, rec.Body.Bytes())
	assert.True(t, freed.BenefitsExemption)
	assert.Equal(t, int64(0), freed.TotalCents)
	assert.Equal(t, []string{"base:6750", "sibling:-3375", "benefits_exemption:-3375"}, lineKinds(freed.Children[0].Lines))
	assert.Equal(t, "Beitragsfrei wegen Leistungsbezug (BuT)", freed.Children[0].Lines[2].Note)

	foreignModel := []map[string]any{{"bookings": []map[string]any{{"week_id": f.offer.Weeks[0].ID, "time_model_id": f.offer.Weeks[1].TimeModels[0].ID}}}}
	rec = quote(map[string]any{"children": foreignModel})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.holiday_quote_invalid", code)
	rec = publicJSON(t, f.traeger, http.MethodPost, holidayQuotePath("unknown-link"), map[string]any{"children": quoteChildren(f.offer)})
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_not_found", code)
}

func TestConfirmationFreezesTheContributionAgainstLaterPriceAndRuleChanges(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Festhalten Beitrag")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	rule := createContributionRule(t, f, billing, siblingHalf())

	preview := f.contribution(t, f.siblings)
	assert.False(t, preview.Frozen)
	assert.Equal(t, int64(3375+9000), preview.TotalCents)

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayConfirmPath(f.period.ID, f.siblings), map[string]any{})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	confirmed := decodeSchoolPortalData[confirmationBody](t, rec.Body.Bytes())
	assert.Equal(t, "confirmed", confirmed.Status)
	assert.True(t, confirmed.Contribution.Frozen)
	require.NotNil(t, confirmed.Contribution.ConfirmedAt)
	assert.Equal(t, int64(3375+9000), confirmed.Contribution.TotalCents)

	// The Träger raises every price and lowers the sibling discount.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayPeriodPath(f.period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	period := decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
	for w := range period.Weeks {
		for m := range period.Weeks[w].TimeModels {
			period.Weeks[w].TimeModels[m].PriceCents *= 2
		}
	}
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayPeriodPath(f.period.ID), period)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	lower := siblingHalf()
	lower["discount_basis_points"] = 1000
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, fmt.Sprintf("%s/%d", contributionRulesPath, rule.ID), lower)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	frozen := f.contribution(t, f.siblings)
	assert.True(t, frozen.Frozen)
	assert.Equal(t, confirmed.Contribution.TotalCents, frozen.TotalCents, "the confirmed registration keeps its price")
	assert.Equal(t, []string{"base:6750", "sibling:-3375"}, lineKinds(frozen.Children[0].Lines))
	assert.Equal(t, confirmed.Contribution.Children[0].ChildID, frozen.Children[0].ChildID)
	assert.Equal(t, int64(9000), f.contribution(t, f.again).TotalCents, "an open registration shows the new price")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayConfirmPath(f.period.ID, f.siblings), nil)
	require.Equal(t, http.StatusConflict, rec.Code, "a confirmed registration is not confirmed twice: %s", rec.Body.String())

	var rows int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.holiday_contributions WHERE holiday_registration_id = ?`, f.siblings).
		Scan(context.Background(), &rows))
	assert.Equal(t, 2, rows, "one frozen contribution per child")
	assert.Equal(t, []string{
		fmt.Sprintf("read:%d", f.siblings), fmt.Sprintf("confirm:%d", f.siblings), fmt.Sprintf("read:%d", f.siblings), fmt.Sprintf("read:%d", f.again),
	}, f.auditActions(t), "every read of a contribution and the confirmation are in the registration's audit trail")
}

func TestBenefitsExemptionIsFrozenWithItsNoteOnlyWithARule(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Befreiung Beitrag")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)

	declared := siblingsRegistration(f.offer, "leistung@example.org")
	declared["benefits_declared"] = true
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), declared)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	registration := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID
	detail := f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(registration, ""), nil))
	assert.Equal(t, int64(6750), detail.Children[0].Bookings[0].PriceCents)

	// Without an exemption rule the statement changes nothing, and an
	// exemption cannot be confirmed: the registration stays received.
	assert.Equal(t, int64(6750+9000), f.contribution(t, registration).TotalCents)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayConfirmPath(f.period.ID, registration), map[string]any{"benefits_exemption": true})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.benefits_exemption_unavailable", code)
	assert.Equal(t, "received", f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(registration, ""), nil)).Status,
		"the refused confirmation was rolled back")

	createContributionRule(t, f, billing, benefitsExemption())
	preview := f.contribution(t, registration)
	assert.True(t, preview.BenefitsExemption, "the preview follows the parents' statement")
	assert.Equal(t, int64(0), preview.TotalCents)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayContributionPath(f.period.ID, registration)+"?benefits_exemption=false", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	withoutExemption := decodeSchoolPortalData[contributionBody](t, rec.Body.Bytes())
	assert.False(t, withoutExemption.BenefitsExemption, "the preview follows the choice of the Träger-Büro")
	assert.Equal(t, int64(6750+9000), withoutExemption.TotalCents)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayContributionPath(f.period.ID, registration)+"?benefits_exemption=perhaps", nil)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayConfirmPath(f.period.ID, registration), map[string]any{"benefits_exemption": true})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	frozen := decodeSchoolPortalData[confirmationBody](t, rec.Body.Bytes()).Contribution
	assert.True(t, frozen.BenefitsExemption)
	assert.Equal(t, int64(0), frozen.TotalCents)
	exemption := frozen.Children[1].Lines[len(frozen.Children[1].Lines)-1]
	assert.Equal(t, "benefits_exemption", exemption.Kind)
	assert.Equal(t, int64(-9000), exemption.AmountCents)
	assert.Equal(t, "Beitragsfrei wegen Leistungsbezug (BuT)", exemption.Note, "the note travels with the frozen contribution")

	// The Träger-Büro confirms the siblings' registration without the
	// exemption the parents did not state.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayConfirmPath(f.period.ID, f.siblings), map[string]any{"benefits_exemption": false})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int64(6750+9000), decodeSchoolPortalData[confirmationBody](t, rec.Body.Bytes()).Contribution.TotalCents)
}

func TestHolidayContributionNeedsTheRightAndStaysWithItsTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Grenze Beitrag")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Grenze Beitrag Fremd", identityaccess.TraegerRoleAdministration)

	for _, request := range []struct{ method, path string }{
		{http.MethodGet, holidayContributionPath(f.period.ID, f.siblings)},
		{http.MethodPost, holidayConfirmPath(f.period.ID, f.siblings)},
	} {
		rec := traegerJSON(t, f.traeger, billing, request.method, request.path, nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right: %s", request.method, request.path, rec.Body.String())
		rec = traegerJSON(t, f.traeger, foreign.token, request.method, request.path, nil)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s %s of another Träger: %s", request.method, request.path, rec.Body.String())
	}
	assert.Empty(t, f.auditActions(t), "refused requests read nothing")
	assert.Equal(t, "received", f.list(t, "").Registrations[0].Status, "the foreign confirmation changed nothing")
	var rows int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.holiday_contributions WHERE organization_id = ?`, f.office.organization).
		Scan(context.Background(), &rows))
	assert.Zero(t, rows)
}

func TestContributionRuleListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Budget Regeln")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	list := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, contributionRulesPath, nil)
		req.Header.Set("Authorization", "Bearer "+billing)
		rec := httptest.NewRecorder()
		f.traeger.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	createContributionRule(t, f, billing, siblingHalf())
	one := list()
	createContributionRule(t, f, billing, benefitsExemption())
	assert.Len(t, list(), len(one), "the list reads the same statements for any number of rules")
	querybudgets.AssertQueryBudget(t, "api.traeger.contribution_rules", one)
}
