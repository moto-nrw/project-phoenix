package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTraegerCatalogPreviewRefusesBilledPricesWithoutKeepingCopies(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	portal := traegerPortalWithDays(t, db, testpkg.TenantRuntime(t, db), owners.HolidayTestSchoolCalendar{})
	office := newTraegerOffice(t, db, portal, "Träger Vorschau", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCatalog(t, db, office.organization)
	testpkg.OwnTenantRows(t, db, office.schools...)
	router := traegerCatalogRouter(t, db, catalogFailedTariffs{err: parentbilling.ErrCareTariffMonthBilled})
	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/catalog/phases", map[string]any{"name": "Vorschau", "kind": "school_year", "service_start_date": "2099-08-01", "service_end_date": "2100-07-31", "all_schools": true})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	phase := decodeSchoolPortalData[catalogPhaseBody](t, rec.Body.Bytes())
	path := fmt.Sprintf("/traeger/catalog/phases/%d", phase.ID)
	rec = traegerJSON(t, router, office.token, http.MethodPost, path+"/offerings", catalogOffering(20, 1600))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	valid := catalogPreview(t, traegerCatalogRouter(t, db), office.token, path)
	for _, school := range valid.Schools {
		require.Equal(t, "ready", school.Outcome)
	}
	var financial int
	require.NoError(t, db.NewRaw(`SELECT (SELECT COUNT(*) FROM parent_billing.care_price_lists WHERE organization_id=?) + (SELECT COUNT(*) FROM care_contract.contract_templates WHERE organization_id=?)`, office.organization, office.organization).Scan(t.Context(), &financial))
	assert.Zero(t, financial, "successful financial validation is also rolled back")
	preview := catalogPreview(t, router, office.token, path)
	for _, school := range preview.Schools {
		assert.Equal(t, "rejected", school.Outcome)
		assert.Equal(t, "parent_billing.care_tariff_month_billed", school.ErrorCode)
	}
	var count int
	require.NoError(t, db.NewSelect().Table("enrollment.phases").ColumnExpr("COUNT(*)").Where("traeger_source_id = ?", phase.ID).Scan(t.Context(), &count))
	assert.Zero(t, count, "a rejected preview commits no school copy")
}

func TestTraegerCatalogCarriesActiveFlagsAndKeepsSchoolDeviations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	portal := traegerPortalWithDays(t, db, testpkg.TenantRuntime(t, db), owners.HolidayTestSchoolCalendar{})
	office := newTraegerOffice(t, db, portal, "Träger Aktiv", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCatalog(t, db, office.organization)
	testpkg.OwnTraegerCareContracts(t, db, office.organization)
	testpkg.OwnTenantRows(t, db, office.schools...)
	router := traegerCatalogRouter(t, db)
	fields := map[string]any{"name": "Aktiv", "kind": "school_year", "service_start_date": "2099-08-01", "service_end_date": "2100-07-31", "all_schools": true}
	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/catalog/phases", fields)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	phase := decodeSchoolPortalData[catalogPhaseBody](t, rec.Body.Bytes())
	path := fmt.Sprintf("/traeger/catalog/phases/%d", phase.ID)
	rec = traegerJSON(t, router, office.token, http.MethodPost, path+"/offerings", catalogOffering(20, 1600))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	offeringID := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID
	apply := func() {
		p := catalogPreview(t, router, office.token, path)
		r := catalogRollout(t, router, office.token, path, p.Fingerprint)
		for _, school := range r.Schools {
			require.Equal(t, "applied", school.Outcome, school.ErrorCode)
		}
	}
	apply()
	fields["is_active"] = false
	rec = traegerJSON(t, router, office.token, http.MethodPut, path, fields)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	draft := catalogOffering(20, 1600)
	draft["fields"].(map[string]any)["is_active"] = false
	rec = traegerJSON(t, router, office.token, http.MethodPut, fmt.Sprintf("/traeger/catalog/offerings/%d", offeringID), draft)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	apply()
	for _, table := range []string{"enrollment.phases", "enrollment.care_offerings"} {
		var active int
		require.NoError(t, db.NewSelect().Table(table).ColumnExpr("COUNT(*)").Where("tenant_id = ?", office.schools[0]).Where("is_active").Scan(t.Context(), &active))
		assert.Zero(t, active)
		_, err := db.NewUpdate().Table(table).Set("is_active = TRUE").Where("tenant_id = ?", office.schools[0]).Exec(t.Context())
		require.NoError(t, err)
	}
	fields["name"] = "Aktiv geändert"
	rec = traegerJSON(t, router, office.token, http.MethodPut, path, fields)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	draft["fields"].(map[string]any)["name"] = "Modul geändert"
	rec = traegerJSON(t, router, office.token, http.MethodPut, fmt.Sprintf("/traeger/catalog/offerings/%d", offeringID), draft)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	apply()
	for _, table := range []string{"enrollment.phases", "enrollment.care_offerings"} {
		var active int
		require.NoError(t, db.NewSelect().Table(table).ColumnExpr("COUNT(*)").Where("tenant_id = ?", office.schools[0]).Where("is_active").Scan(t.Context(), &active))
		assert.Equal(t, 1, active, "the school's activation stays")
		require.NoError(t, db.NewSelect().Table(table).ColumnExpr("COUNT(*)").Where("tenant_id = ?", office.schools[1]).Where("is_active").Scan(t.Context(), &active))
		assert.Zero(t, active)
	}
}
