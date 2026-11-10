package api

import (
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedBookingAccounts gives the demo Träger revenue accounts and cost
// centers (#4821) before its monthly runs are released, so their positions
// carry them and the accounts page is filled: one account per Leistung, an
// own account and KOST2 for the first Angebot, a KOST1 per school and one
// for the holiday care from the first billed month. Invented values in the
// ranges DATEV accepts. A second seed finds the cost centers in place.
func seedBookingAccounts(client *Client, auth AuthRef, from calendar.Date) (int, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/finance-exports/booking-accounts")
	if err != nil {
		return 0, fmt.Errorf("read the booking accounts: %w", err)
	}
	var envelope struct {
		Data struct {
			Schools []struct {
				ID int64 `json:"id"`
			} `json:"schools"`
			Offerings []struct {
				CareOfferingID int64 `json:"care_offering_id"`
			} `json:"offerings"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("read the booking accounts: %w", err)
	}
	accounts := []map[string]any{
		{"scope": "meals", "scope_id": 0, "account": "4420"},
		{"scope": "holiday", "scope_id": 0, "account": "4430"},
	}
	centers := []map[string]any{{"scope": "holiday", "scope_id": 0, "code": "FER"}}
	for i, school := range envelope.Data.Schools {
		centers = append(centers, map[string]any{"scope": "school", "scope_id": school.ID, "code": fmt.Sprintf("KST%d00", i+1)})
	}
	if len(envelope.Data.Offerings) > 0 {
		offering := envelope.Data.Offerings[0].CareOfferingID
		accounts = append(accounts, map[string]any{"scope": "offering", "scope_id": offering, "account": "4410"})
		centers = append(centers, map[string]any{"scope": "offering", "scope_id": offering, "code": "FRUEH"})
	}
	for _, account := range accounts {
		if _, err := client.PutWithAuth(auth, "/traeger/finance-exports/booking-accounts/revenue-accounts", account); err != nil {
			return 0, fmt.Errorf("set the revenue account of %v: %w", account["scope"], err)
		}
	}
	for _, center := range centers {
		center["valid_from"] = from.String()
		_, err := client.PostWithAuth(auth, "/traeger/finance-exports/booking-accounts/cost-centers", center)
		if err != nil && !seedConflictCode(err, "parent_billing.cost_center_date_taken") {
			return 0, fmt.Errorf("add the cost center of %v: %w", center["scope"], err)
		}
	}
	return len(accounts) + len(centers), nil
}
