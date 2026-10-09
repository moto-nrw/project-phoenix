package api

import (
	"context"
	"fmt"
)

// seedExternalCaregiversStep records the external AG leaders without a moto
// account (#3823), so the supervision pickers and the Personal list show the
// "Extern" entries on every dev machine.
type seedExternalCaregiversStep struct{}

func (seedExternalCaregiversStep) Name() string { return "Seeding external caregivers" }

func (seedExternalCaregiversStep) Run(_ context.Context, rt *Runtime) error {
	externals := []map[string]any{
		{"first_name": "Jonas", "last_name": "Becker", "organization": "Musikschule Bergstadt"},
		{"first_name": "Leonie", "last_name": "Wagner", "organization": "TuS Sportverein"},
		{"first_name": "Selin", "last_name": "Aydın"},
	}
	for _, external := range externals {
		if _, err := rt.Client.Post("/api/staff/externals", external); err != nil {
			return fmt.Errorf("create external caregiver %q: %w", external["last_name"], err)
		}
	}
	fmt.Printf("  %d external caregivers created\n", len(externals))
	return nil
}
