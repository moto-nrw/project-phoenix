package api

import (
	"context"
	"fmt"
)

type seedImportAuditStep struct{}

func (seedImportAuditStep) Name() string { return "Seeding import audit" }

func (seedImportAuditStep) Run(_ context.Context, rt *Runtime) error {
	rt.Client.BindAuth(rt.TenantAuth)
	csv := []byte("Vorname,Nachname,Klasse\nImport,Demo,Klasse 3z\n")
	if _, err := rt.Client.PostFile("/api/import/students/import", "file", "demo-student.csv", csv); err != nil {
		return fmt.Errorf("seed student import audit: %w", err)
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
