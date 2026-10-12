package api

import (
	"context"
	"fmt"
)

type seedImportAuditStep struct{}

func (seedImportAuditStep) Name() string { return "Seeding import audit" }

// importAuditStudentIndex is the demo child the seeded import updates. The
// import exists for its audit row; creating a child for it left a record
// without parents, group or plan in every child picker (#3894). An upsert of
// a child the school already has is the everyday case: the class list comes
// again from the school office.
const importAuditStudentIndex = 7

func (seedImportAuditStep) Run(_ context.Context, rt *Runtime) error {
	rt.Client.BindAuth(rt.TenantAuth)
	student := DemoStudents[importAuditStudentIndex]
	csv := fmt.Appendf(nil, "Vorname,Nachname,Klasse\n%s,%s,%s\n", student.FirstName, student.LastName, student.Class)
	raw, err := rt.Client.PostFileWithFields("/api/import/students/import", "file", "klassenliste.csv", csv,
		map[string]string{"mode": "upsert"})
	if err != nil {
		return fmt.Errorf("seed student import audit: %w", err)
	}
	var resp struct {
		Data struct {
			CreatedCount int
			UpdatedCount int
			ErrorCount   int
		} `json:"data"`
	}
	if err := parseJSON(raw, &resp); err != nil {
		return fmt.Errorf("parse student import audit: %w", err)
	}
	if resp.Data.CreatedCount != 0 || resp.Data.UpdatedCount != 1 || resp.Data.ErrorCount != 0 {
		return fmt.Errorf("seed student import audit: want 1 updated child, got %d created, %d updated, %d errors",
			resp.Data.CreatedCount, resp.Data.UpdatedCount, resp.Data.ErrorCount)
	}
	return nil
}

type seedDataAccessAuditStep struct{}

func (seedDataAccessAuditStep) Name() string { return "Seeding data access audit" }

func (seedDataAccessAuditStep) Run(_ context.Context, rt *Runtime) error {
	rt.Client.BindAuth(rt.TenantAuth)
	today := todaySeedDate()
	path := fmt.Sprintf("/api/staff/time-tracking/export?year=%d&month=%d&format=csv", today.Year(), today.Month())
	if _, err := rt.Client.Get(path); err != nil {
		return fmt.Errorf("seed time-tracking export audit: %w", err)
	}
	return nil
}
