package api

import (
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedBillingTerms gives the demo Träger who gets an invoice, how it is paid
// and by when (#4689) through the Träger routes, before the monthly run is
// released: the SJR pattern of the monthly care by direct debit and the
// holiday care by transfer, the run's due date per payment method (2nd and
// 4th of the month after), and two families that deviate: one invoice goes
// to another payer, one to both parents with its own due day.
func seedBillingTerms(client *Client, auth AuthRef, runID int64, month calendar.Date) error {
	for service, terms := range map[string]map[string]any{
		"care":    {"payment_method": "direct_debit", "due_days": 10},
		"meal":    {"payment_method": "direct_debit", "due_days": 10, "bundle_with": "care"},
		"holiday": {"payment_method": "transfer", "due_days": 14},
	} {
		if _, err := client.PutWithAuth(auth, "/traeger/billing-terms/"+service, terms); err != nil {
			return fmt.Errorf("keep the payment terms of %s: %w", service, err)
		}
	}
	next := calendar.NewDate(month.Year(), month.Month(), 1).AddDays(32)
	if _, err := client.PutWithAuth(auth, fmt.Sprintf("/traeger/billing-run-due-dates/care/%d", runID), map[string]any{
		"due_dates": []map[string]any{
			{"payment_method": "transfer", "due_on": calendar.NewDate(next.Year(), next.Month(), 2).String()},
			{"payment_method": "direct_debit", "due_on": calendar.NewDate(next.Year(), next.Month(), 4).String()},
		},
	}); err != nil {
		return fmt.Errorf("keep the due dates of the monthly run: %w", err)
	}
	raw, err := client.GetWithAuth(auth, fmt.Sprintf("/traeger/care-billing-runs/%d", runID))
	if err != nil {
		return fmt.Errorf("read the monthly run: %w", err)
	}
	var envelope struct {
		Data struct {
			Invoices []struct {
				ContractID int64 `json:"contract_id"`
				Recipient  struct {
					LastName string `json:"last_name"`
				} `json:"recipient"`
			} `json:"invoices"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("read the monthly run: %w", err)
	}
	invoices := envelope.Data.Invoices
	deviations := []map[string]any{
		{"recipient_kind": "payer", "payer_first_name": "Helga", "payer_last_name": "Großmann", "payer_email": "helga.grossmann@example.org",
			"payer_address":  map[string]any{"street": "Lindenweg 4", "postal_code": "75175", "city": "Pforzheim"},
			"payment_method": "transfer"},
		{"recipient_kind": "both", "second_first_name": "Jonas", "due_day": 15},
	}
	for i, deviation := range deviations {
		if i >= len(invoices) {
			break
		}
		deviation["contract_id"] = invoices[i].ContractID
		if deviation["recipient_kind"] == "both" {
			deviation["second_last_name"] = invoices[i].Recipient.LastName
		}
		if _, err := client.PutWithAuth(auth, "/traeger/family-billing/", deviation); err != nil {
			return fmt.Errorf("keep the deviation of a family: %w", err)
		}
	}
	return nil
}
