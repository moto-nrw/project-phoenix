package api

import (
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedDatevValues are the DATEV values of the demo Träger's tax office
// (#4694): invented numbers in the ranges DATEV accepts.
var seedDatevValues = []struct {
	key   string
	value any
}{
	{"parent_billing.datev_consultant_number", "29098"},
	{"parent_billing.datev_client_number", "55003"},
	{"parent_billing.datev_account_length", "4"},
	{"parent_billing.datev_revenue_account", "4400"},
}

// seedDatevFile enters the DATEV values and downloads the DATEV file of the
// released monthly run of month once (#6433), so the export page shows a
// run whose bookings went to DATEV next to one with new bookings, and the
// protocol a DATEV file. Another seed of the same Träger may have taken the
// bookings a moment earlier.
func seedDatevFile(client *Client, auth AuthRef, month calendar.Date) error {
	for _, setting := range seedDatevValues {
		if _, err := client.PutWithAuth(auth, "/traeger/settings/values/"+setting.key, map[string]any{"value": setting.value}); err != nil {
			return fmt.Errorf("set the DATEV value %s: %w", setting.key, err)
		}
	}
	raw, err := client.GetWithAuth(auth, "/traeger/care-billing-runs/")
	if err != nil {
		return fmt.Errorf("read the monthly runs: %w", err)
	}
	var envelope struct {
		Data struct {
			Runs []struct {
				ID           int64         `json:"id"`
				BillingMonth calendar.Date `json:"billing_month"`
				Status       string        `json:"status"`
			} `json:"runs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("read the monthly runs: %w", err)
	}
	for _, run := range envelope.Data.Runs {
		if run.BillingMonth != month || run.Status != "released" {
			continue
		}
		file, err := client.GetWithAuth(auth, fmt.Sprintf("/traeger/finance-exports/runs/care-runs/%d/datev", run.ID))
		if seedConflictCode(err, seedDatevNoNewBookings) || seedConflictCode(err, seedDatevBookingsTaken) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("download the DATEV file: %w", err)
		}
		if len(file) == 0 {
			return fmt.Errorf("the DATEV file is empty")
		}
		fmt.Println("  Exporte: DATEV file of the first monthly run downloaded")
		return nil
	}
	return nil
}

// The registered codes of a DATEV download another seed of the Träger made.
const (
	seedDatevNoNewBookings = "parent_billing.datev_no_new_bookings"
	seedDatevBookingsTaken = "parent_billing.datev_bookings_taken"
)

// seedDebtorNumber is the invented debtor number of the i-th family of a
// monthly run: five digits in the debtor range DATEV accepts for accounts
// of four digits.
func seedDebtorNumber(i int) string {
	return fmt.Sprintf("1%04d", 101+i)
}
