package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The SJR price lists 2026/27 through the routes (#6196): Ispringen prices
// Modul 2 per weekday with a price for every further child, Birkenfeld
// prices Modul 4 by days per week with 25 % off from the second child and
// the meal on top. Siblings count per school; August is free at Ispringen.

const (
	sjrModule2 = int64(9201)
	sjrModule4 = int64(9204)
)

type priceFormsRunBody struct {
	ID           int64  `json:"id"`
	PreviewCount int    `json:"preview_count"`
	PreviewCents int64  `json:"preview_cents"`
	Fingerprint  string `json:"fingerprint"`
	FreeOfCharge int    `json:"free_of_charge"`
	Invoices     []struct {
		ContractID int64 `json:"contract_id"`
		Positions  []struct {
			DaysPerWeek      *int   `json:"days_per_week"`
			TierMonthlyCents *int64 `json:"tier_monthly_cents"`
			MealExtra        bool   `json:"meal_extra"`
			AmountCents      int64  `json:"amount_cents"`
		} `json:"positions"`
		Reductions []struct {
			TariffSiblingPrice bool  `json:"tariff_sibling_price"`
			AmountCents        int64 `json:"amount_cents"`
		} `json:"reductions"`
		TotalCents int64 `json:"total_cents"`
	} `json:"invoices"`
}

func ispringenModule2(school int64) map[string]any {
	prices := []map[string]any{}
	for _, weekday := range []string{"mon", "tue", "wed", "thu"} {
		prices = append(prices, map[string]any{"care_offering_id": sjrModule2, "module_name": "Regelschule Modul 2",
			"weekday": weekday, "monthly_cents": 1600, "sibling_cents": 1250})
	}
	prices = append(prices, map[string]any{"care_offering_id": sjrModule2, "module_name": "Regelschule Modul 2",
		"weekday": "fri", "monthly_cents": 1400, "sibling_cents": 1050})
	return map[string]any{"school_id": school, "valid_from": "2029-09-01", "free_months": []int{8}, "prices": prices}
}

func birkenfeldModule4(school int64, threeDays int64) map[string]any {
	prices := []map[string]any{}
	for days, cents := range []int64{2000, 4000, threeDays, 8000, 10000} {
		prices = append(prices, map[string]any{"care_offering_id": sjrModule4, "module_name": "Modul 4 mit Mittagessen",
			"days_per_week": days + 1, "monthly_cents": cents, "meal_extra": true})
	}
	return map[string]any{"school_id": school, "valid_from": "2029-09-01", "prices": prices}
}

func sjrModule2Services() []careService {
	services := []careService{}
	for _, weekday := range []string{"mon", "tue", "wed", "thu", "fri"} {
		services = append(services, careService{weekday, sjrModule2, "Regelschule Modul 2", "16:30"})
	}
	return services
}

func sjrModule4Services(weekdays ...string) []careService {
	services := []careService{}
	for _, weekday := range weekdays {
		services = append(services, careService{weekday, sjrModule4, "Modul 4 mit Mittagessen", "15:30"})
	}
	return services
}

func (f careBillingFixture) priceFormsRun(t *testing.T, id int64) priceFormsRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[priceFormsRunBody](t, rec.Body.Bytes())
}

func TestSJRPriceFormsBillThroughTheRoutes(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger SJR Preisformen")
	ispringen, birkenfeld := f.office.schools[0], f.office.schools[1]
	anna := careContract(t, f.db, f.office.organization, ispringen, "Anna", careReductionStart, sjrModule2Services()...)
	ben := careContract(t, f.db, f.office.organization, ispringen, "Ben", careReductionStart, sjrModule2Services()...)
	jonas := careContract(t, f.db, f.office.organization, birkenfeld, "Jonas", careReductionStart, sjrModule4Services("mon", "tue", "wed")...)
	klara := careContract(t, f.db, f.office.organization, birkenfeld, "Klara", careReductionStart, sjrModule4Services("mon", "wed", "fri")...)
	// Rosa belongs to Anna's family but goes to Birkenfeld: she counts there.
	rosa := careContract(t, f.db, f.office.organization, birkenfeld, "Rosa", careReductionStart, sjrModule4Services("tue", "wed", "thu")...)
	sameFamily(t, f, ben, anna)
	sameFamily(t, f, rosa, anna)
	sameFamily(t, f, klara, jonas)
	for _, list := range []map[string]any{ispringenModule2(ispringen), birkenfeldModule4(birkenfeld, 6000)} {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, list)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	f.saveRule(t, careRule("sibling", "Geschwisterpreis Ispringen", []int64{ispringen},
		map[string]any{"sibling_from_child": 2, "tariff_sibling_price": true}))
	f.saveRule(t, careRule("sibling", "Geschwister Birkenfeld", []int64{birkenfeld},
		map[string]any{"sibling_from_child": 2, "discount_basis_points": 2500}))

	run := f.startRun(t, careMonth)
	preview := f.priceFormsRun(t, run.ID)
	totals := map[int64]int64{}
	for _, invoice := range preview.Invoices {
		totals[invoice.ContractID] = invoice.TotalCents
	}
	// Ispringen: 4 × 16,00 + 14,00 = 78,00 €; the second child 4 × 12,50 +
	// 10,50 = 60,50 €. Birkenfeld Modul 4 with 3 days: 60,00 €, the second
	// child 45,00 €. Rosa is the only child of her family in Birkenfeld.
	assert.Equal(t, map[int64]int64{anna: 7800, ben: 6050, jonas: 6000, klara: 4500, rosa: 6000}, totals)

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"),
		map[string]any{"document_count": preview.PreviewCount, "total_cents": preview.PreviewCents, "fingerprint": preview.Fingerprint})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[priceFormsRunBody](t, rec.Body.Bytes())
	require.Len(t, released.Invoices, 5)
	for _, invoice := range released.Invoices {
		assert.Equal(t, totals[invoice.ContractID], invoice.TotalCents, "the issued invoice keeps its total")
		require.Len(t, invoice.Positions, 1)
		position := invoice.Positions[0]
		switch invoice.ContractID {
		case klara:
			require.NotNil(t, position.DaysPerWeek, "the issued position keeps its scale step")
			assert.Equal(t, 3, *position.DaysPerWeek)
			assert.Equal(t, int64(6000), *position.TierMonthlyCents)
			assert.True(t, position.MealExtra, "the meal is billed apart, never as a silent zero")
		case ben:
			assert.Nil(t, position.DaysPerWeek)
			require.Len(t, invoice.Reductions, 1)
			assert.True(t, invoice.Reductions[0].TariffSiblingPrice, "the reduction keeps its mode")
			assert.Equal(t, int64(-1750), invoice.Reductions[0].AmountCents)
		}
	}

	// A billed scale step keeps its price for the billed month.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, birkenfeldModule4(birkenfeld, 6500))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_tariff_month_billed")

	// August is free at Ispringen only.
	august := f.priceFormsRun(t, f.startRun(t, "2030-08-01").ID)
	assert.Equal(t, 2, august.FreeOfCharge)
	billed := []int64{}
	for _, invoice := range august.Invoices {
		billed = append(billed, invoice.ContractID)
	}
	assert.ElementsMatch(t, []int64{jonas, klara, rosa}, billed)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(august.ID, "/release"),
		map[string]any{"document_count": august.PreviewCount, "total_cents": august.PreviewCents, "fingerprint": august.Fingerprint})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 2, decodeSchoolPortalData[priceFormsRunBody](t, rec.Body.Bytes()).FreeOfCharge)
	changedList := ispringenModule2(ispringen)
	changedList["free_months"] = []int{}
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, changedList)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 2, f.priceFormsRun(t, august.ID).FreeOfCharge)
}

// The routes refuse a price list that mixes forms within a module or puts
// a sibling price above the first child's, and a rule with both a share and
// the sibling prices of the price list.
func TestPriceFormsAreCheckedBeforeTheyAreSaved(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Preisformen Prüfung")
	school := f.office.schools[0]
	for name, prices := range map[string][]map[string]any{
		"two forms in one module": {
			{"care_offering_id": sjrModule4, "module_name": "Modul 4", "days_per_week": 1, "monthly_cents": 2000},
			{"care_offering_id": sjrModule4, "module_name": "Modul 4", "weekday": "mon", "monthly_cents": 2000},
		},
		"sibling above first": {
			{"care_offering_id": sjrModule4, "module_name": "Modul 4", "days_per_week": 1, "monthly_cents": 2000, "sibling_cents": 2100},
		},
	} {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath,
			map[string]any{"school_id": school, "valid_from": "2029-09-01", "prices": prices})
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", name, rec.Body.String())
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRulesPath, careRule("sibling", "Beides", []int64{school},
		map[string]any{"sibling_from_child": 2, "tariff_sibling_price": true, "discount_basis_points": 2500}))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

// A run of a free month bills nothing and still closes (#6196): otherwise
// it would hold the school's August for good.
func TestFreeMonthRunClosesWithoutInvoices(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Preisformen August")
	ispringen := f.office.schools[0]
	careContract(t, f.db, f.office.organization, ispringen, "Anna", careReductionStart, sjrModule2Services()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, ispringenModule2(ispringen))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunsPath,
		map[string]any{"billing_month": "2030-08-01", "school_ids": []int64{ispringen}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	august := f.priceFormsRun(t, decodeSchoolPortalData[priceFormsRunBody](t, rec.Body.Bytes()).ID)
	assert.Zero(t, august.PreviewCount)
	assert.Equal(t, 1, august.FreeOfCharge)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(august.ID, "/release"),
		map[string]any{"document_count": 0, "total_cents": 0, "fingerprint": august.Fingerprint})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "released", f.run(t, august.ID).Status)
}
