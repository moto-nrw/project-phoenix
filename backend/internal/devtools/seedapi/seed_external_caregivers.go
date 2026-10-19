package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// seedExternalCaregiversStep records the external AG leaders without a moto
// account (#3823), so the supervision pickers and the Personal list show the
// "Extern" entries on every dev machine.
type seedExternalCaregiversStep struct{}

func (seedExternalCaregiversStep) Name() string { return "Seeding external caregivers" }

func (seedExternalCaregiversStep) Run(_ context.Context, rt *Runtime) error {
	existing, err := seededExternalCaregivers(rt)
	if err != nil {
		return err
	}
	externals := []seedExternalCaregiver{
		{FirstName: "Jonas", LastName: "Becker", Organization: "Musikschule Bergstadt"},
		{FirstName: "Leonie", LastName: "Wagner", Organization: "TuS Sportverein"},
		{FirstName: "Selin", LastName: "Aydın"},
	}
	created := 0
	for _, external := range externals {
		if _, found := existing[external]; found {
			continue
		}
		if _, err := rt.Client.Post("/api/staff/externals", external); err != nil {
			return fmt.Errorf("create external caregiver %q: %w", external.LastName, err)
		}
		created++
	}
	fmt.Printf("  %d external caregivers created\n", created)
	return nil
}

type seedExternalCaregiver struct {
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Organization string `json:"organization,omitempty"`
}

// seededExternalCaregivers reads the directory once before creating the demo
// entries. Internal colleagues with the same name deliberately do not match:
// only the explicit external marker identifies a prior seed run.
func seededExternalCaregivers(rt *Runtime) (map[seedExternalCaregiver]struct{}, error) {
	raw, err := rt.Client.Get("/api/staff/")
	if err != nil {
		return nil, fmt.Errorf("list existing external caregivers: %w", err)
	}
	var envelope struct {
		Data []struct {
			IsExternal           bool   `json:"is_external"`
			ExternalOrganization string `json:"external_organization"`
			Person               *struct {
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
			} `json:"person"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode existing external caregivers: %w", err)
	}
	existing := make(map[seedExternalCaregiver]struct{}, len(envelope.Data))
	for _, staff := range envelope.Data {
		if staff.IsExternal && staff.Person != nil {
			existing[seedExternalCaregiver{
				FirstName:    staff.Person.FirstName,
				LastName:     staff.Person.LastName,
				Organization: staff.ExternalOrganization,
			}] = struct{}{}
		}
	}
	return existing, nil
}
