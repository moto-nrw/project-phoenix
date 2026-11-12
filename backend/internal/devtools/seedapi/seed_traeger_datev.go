package api

import (
	"fmt"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
// released monthly run of the school in month once (#6433), so the export
// page shows a run whose bookings went to DATEV next to one with new
// bookings, and the protocol a DATEV file. A released run of other schools
// of the Träger is not the school's: on the public demo it may hold no
// invoice any more once its demo school is gone. Another seed of the same Träger may have taken the
// bookings a moment earlier. Legacy runs may lack debtor numbers in their
// immutable invoice snapshots; skip this demonstration download in that case.
func seedDatevFile(client *Client, auth AuthRef, school int64, month calendar.Date) error {
	for _, setting := range seedDatevValues {
		if _, err := client.PutWithAuth(auth, "/traeger/settings/values/"+setting.key, map[string]any{"value": setting.value}); err != nil {
			return fmt.Errorf("set the DATEV value %s: %w", setting.key, err)
		}
	}
	runID, err := seedCareRunOf(client, auth, school, month, "released")
	if err != nil || runID == 0 {
		return err
	}
	file, err := client.GetWithAuth(auth, fmt.Sprintf("/traeger/finance-exports/runs/care-runs/%d/datev", runID))
	if seedConflictCode(err, seedDatevNoNewBookings) || seedConflictCode(err, seedDatevBookingsTaken) {
		return nil
	}
	if seedConflictCode(err, seedDatevBookingsInvalid) {
		fmt.Println("  Exporte: DATEV demo download skipped: existing invoice snapshots are not DATEV-ready")
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

// The registered conflicts that make the demonstration download unnecessary
// or unavailable for existing invoice snapshots.
const (
	seedDatevNoNewBookings   = "parent_billing.datev_no_new_bookings"
	seedDatevBookingsTaken   = "parent_billing.datev_bookings_taken"
	seedDatevBookingsInvalid = "parent_billing.datev_bookings_invalid"
)

// seedDebtorNumber is the invented debtor number of the i-th family of a
// monthly run: five digits in the debtor range DATEV accepts for accounts
// of four digits.
func seedDebtorNumber(i int) string {
	return fmt.Sprintf("1%04d", 101+i)
}
