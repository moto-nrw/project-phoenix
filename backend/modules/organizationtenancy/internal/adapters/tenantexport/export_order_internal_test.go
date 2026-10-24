package tenantexport

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// The physical row order is the insertion order. For the participation and
// the answers of an anonymous survey (#4781), written in one transaction, it
// would pair the n-th participation with the n-th answer, so their artifacts
// are ordered by columns instead.
func TestExportOrderReplacesPhysicalOrderOnlyWhenCatalogued(t *testing.T) {
	t.Parallel()
	model := organizationtenancy.ExportModelTable{Columns: []organizationtenancy.ExportColumn{{Name: "id"}, {Name: "survey_id"}, {Name: "account_id"}}}

	order, err := exportOrder(organizationtenancy.LifecycleTable{Name: "users.other"}, model)
	if err != nil || order != "t.ctid" {
		t.Fatalf("uncatalogued order = %q, %v; want t.ctid", order, err)
	}
	order, err = exportOrder(organizationtenancy.LifecycleTable{Name: "users.survey_participations", ExportOrder: []string{"survey_id", "account_id"}}, model)
	if err != nil || order != `t."survey_id", t."account_id"` {
		t.Fatalf("catalogued order = %q, %v", order, err)
	}
	if _, err := exportOrder(organizationtenancy.LifecycleTable{Name: "users.survey_responses", ExportOrder: []string{"created_at"}}, model); err == nil {
		t.Fatal("an order column the artifact does not export must fail the table")
	}
}

// Every catalogued survey table with a participant or an answer names an
// order: the shipped catalog cannot fall back to the insertion order there.
func TestSurveyTablesExportWithoutInsertionOrder(t *testing.T) {
	t.Parallel()
	raw, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog organizationtenancy.DataLifecycleCatalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"users.survey_participations": true, "users.survey_responses": true, "users.survey_answers": true}
	for _, table := range catalog.Tables {
		if want[table.Name] {
			if len(table.ExportOrder) == 0 {
				t.Errorf("%s is exported in insertion order", table.Name)
			}
			delete(want, table.Name)
		}
	}
	for name := range want {
		t.Errorf("%s is missing from the catalog", name)
	}
}
