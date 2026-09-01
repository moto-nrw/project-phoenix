package api

import (
	"context"
	"fmt"
)

type seedStaffMasterDataStep struct{}

func (seedStaffMasterDataStep) Name() string { return "Seeding staff master data" }

func (seedStaffMasterDataStep) Run(_ context.Context, rt *Runtime) error {
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if len(staffIDs) == 0 {
		return fmt.Errorf("staff master-data prerequisites not available")
	}
	rt.Client.BindAuth(rt.TenantAuth)
	path := fmt.Sprintf("/api/staff/%d/stammdaten/kontakt", staffIDs[0])
	if _, err := rt.Client.Put(path, map[string]any{
		"address_street":          "Musterweg 12",
		"address_postal_code":     "49074",
		"address_city":            "Osnabrück",
		"phone":                   "+49 541 1234567",
		"email":                   "anna.mueller@example.test",
		"emergency_contact_name":  "Thomas Müller",
		"emergency_contact_phone": "+49 541 7654321",
		"note":                    "Kontaktdaten für die Demo ergänzt",
	}); err != nil {
		return fmt.Errorf("seed staff master data: %w", err)
	}
	fmt.Println("  1 staff master-data record created")
	return nil
}
