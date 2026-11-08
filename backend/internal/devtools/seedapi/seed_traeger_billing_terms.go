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
				ContractID  int64  `json:"contract_id"`
				FamilyEmail string `json:"family_email"`
				Recipient   struct {
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
	return seedSEPAMandates(client, auth, invoices)
}

// seedSEPAMandates gives the demo Träger its creditor identifier (the
// Bundesbank's test identifier) and the SEPA mandates of the families that
// pay by direct debit (#4695): one family's first mandate was revoked and
// replaced, one family has none, so the review shows its hint and the
// invoice goes as a transfer. The first family pays by transfer anyway.
func seedSEPAMandates(client *Client, auth AuthRef, invoices []struct {
	ContractID  int64  `json:"contract_id"`
	FamilyEmail string `json:"family_email"`
	Recipient   struct {
		LastName string `json:"last_name"`
	} `json:"recipient"`
}) error {
	if _, err := client.PutWithAuth(auth, "/traeger/sepa-creditor/", map[string]any{
		"creditor_id": "DE98ZZZ09999999999", "prenotification_days": 5, "missing_mandate": "transfer",
	}); err != nil {
		return fmt.Errorf("keep the SEPA creditor: %w", err)
	}
	// Every monthly run seeds its families again: a family that has a
	// mandate already, or one of its siblings, keeps it.
	raw, err := client.GetWithAuth(auth, "/traeger/sepa-mandates/")
	if err != nil {
		return fmt.Errorf("read the SEPA mandates: %w", err)
	}
	var existing struct {
		Data []struct {
			FamilyEmail string `json:"family_email"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &existing); err != nil {
		return fmt.Errorf("read the SEPA mandates: %w", err)
	}
	seeded := map[string]bool{}
	for _, mandate := range existing.Data {
		seeded[mandate.FamilyEmail] = true
	}
	for i, invoice := range invoices {
		if i == 0 || i == 2 || seeded[invoice.FamilyEmail] {
			continue
		}
		seeded[invoice.FamilyEmail] = true
		mandate := map[string]any{
			"contract_id": invoice.ContractID, "reference": fmt.Sprintf("SJR-2026-%04d", invoice.ContractID), "signed_on": "2026-09-01",
			"kind": "recurring", "sequence": "recurring", "account_holder": "Familie " + invoice.Recipient.LastName,
			"iban": demoIBAN(9000 + i),
		}
		raw, err := client.PostWithAuth(auth, "/traeger/sepa-mandates/", mandate)
		if err != nil {
			return fmt.Errorf("keep the SEPA mandate of a family: %w", err)
		}
		if i != 1 {
			continue
		}
		var created struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &created); err != nil {
			return fmt.Errorf("read the SEPA mandate: %w", err)
		}
		if _, err := client.PostWithAuth(auth, fmt.Sprintf("/traeger/sepa-mandates/%d/revoke", created.Data.ID),
			map[string]any{"revoked_on": "2026-10-01"}); err != nil {
			return fmt.Errorf("revoke the SEPA mandate of a family: %w", err)
		}
		mandate["reference"], mandate["signed_on"], mandate["sequence"] = fmt.Sprintf("SJR-2026-%04d-2", invoice.ContractID), "2026-10-02", "first"
		mandate["iban"] = demoIBAN(9100 + i)
		if _, err := client.PostWithAuth(auth, "/traeger/sepa-mandates/", mandate); err != nil {
			return fmt.Errorf("keep the new SEPA mandate of a family: %w", err)
		}
	}
	return nil
}
