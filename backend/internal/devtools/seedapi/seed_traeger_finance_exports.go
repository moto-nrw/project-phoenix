package seedapi

import (
	"fmt"
)

// seedSollstellung maps the columns of the Sollstellung as a Kämmerei asks
// for them (#4693) and downloads the Sollstellung of the released holiday
// run once, so the export page and the protocol show a file with its number
// and the invoice its history.
func seedSollstellung(client *Client, auth AuthRef, runID int64) error {
	if _, err := client.PutWithAuth(auth, "/traeger/finance-exports/settings/sollstellung", map[string]any{
		"columns": []map[string]any{
			{"field": "debtor_number", "header": "Debitor"},
			{"field": "invoice_number", "header": "Belegnummer"},
			{"field": "invoice_date", "header": "Belegdatum"},
			{"field": "due_on", "header": "Fällig"},
			{"field": "amount", "header": "Betrag"},
			{"field": "recipient_name", "header": "Name"},
			{"field": "document_kind", "header": "Belegart"},
			{"field": "constant", "header": "Buchungskreis", "value": "1000"},
		},
		"delimiter": ";", "decimal_separator": ",", "date_format": "dd.mm.yyyy", "encoding": "utf-8-bom",
		"header_row": true, "file_name_pattern": "csv-export-bewegungsdaten-{NR}.csv",
	}); err != nil {
		return fmt.Errorf("map the Sollstellung: %w", err)
	}
	file, err := client.GetWithAuth(auth, fmt.Sprintf("/traeger/finance-exports/runs/holiday-runs/%d/sollstellung", runID))
	if err != nil {
		return fmt.Errorf("download the Sollstellung: %w", err)
	}
	if len(file) == 0 {
		return fmt.Errorf("the Sollstellung is empty")
	}
	return nil
}
