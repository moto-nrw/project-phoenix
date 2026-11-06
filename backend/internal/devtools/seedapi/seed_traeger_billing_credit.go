package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
)

// seedTraegerBillingCreditStep credits part of the first invoice of the
// released holiday run (#4690): the Gutschrift is a document of its own
// with a number, a reason and its letter, so the run page shows a history
// and the overview for the Kämmerei a line with a minus. It runs after the
// letters step, so the Gutschrift gets its letter at once.
type seedTraegerBillingCreditStep struct{}

func (seedTraegerBillingCreditStep) Name() string { return "Seeding a credit of an invoice" }

// seedCreditCents is the credited amount; the reason is invented.
const (
	seedCreditCents  = int64(500)
	seedCreditReason = "Ausflug am Freitag fiel aus."
)

func (seedTraegerBillingCreditStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the credit seed prerequisites are not available")
	}
	if skipped, _ := rt.Values[seedHolidayRegistrationsSkippedKey].(bool); skipped {
		fmt.Println("  Abrechnung: no holiday run without registrations; no credit")
		return nil
	}
	runID, ok := rt.Values[seedBillingRunKey].(int64)
	if !ok || runID <= 0 {
		return fmt.Errorf("the credit needs the released holiday run")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	raw, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d", runID))
	if err != nil {
		return fmt.Errorf("read the released holiday run: %w", err)
	}
	documentID, childID, err := seedCreditTarget(raw)
	if err != nil {
		return err
	}
	if documentID == 0 {
		fmt.Println("  Abrechnung: no invoice of the holiday run to credit")
		return nil
	}
	if _, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d/documents/%d/credits", runID, documentID),
		map[string]any{"child_id": childID, "amount_cents": seedCreditCents, "reason": seedCreditReason}); err != nil {
		return fmt.Errorf("credit the invoice: %w", err)
	}
	fmt.Println("  Abrechnung: one invoice of the holiday run credited")
	return seedReadBillingFamilies(rt, auth)
}

// seedReadBillingFamilies reads the families of the Träger once the run has
// an invoice, a cancellation and a credit (#4691), so the seed proves the
// contract of the page "Familien".
func seedReadBillingFamilies(rt *Runtime, auth AuthRef) error {
	raw, err := rt.Client.GetWithAuth(auth, "/traeger/billing-families")
	if err != nil {
		return fmt.Errorf("read the families of the billing: %w", err)
	}
	var envelope struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode the families of the billing: %w", err)
	}
	if len(envelope.Data) == 0 {
		return fmt.Errorf("the families of the billing are empty after a released run")
	}
	fmt.Printf("  Abrechnung: %d families with invoices\n", len(envelope.Data))
	return nil
}

// seedCreditHistoryLink is what the credit step reads of the history of an
// invoice.
type seedCreditHistoryLink struct {
	Kind string `json:"kind"`
}

// seedCreditTarget finds the first child of an invoice of the run that is not
// cancelled with enough left to credit; zero when there is none.
func seedCreditTarget(raw []byte) (int64, int64, error) {
	var envelope struct {
		Data struct {
			Documents []struct {
				ID       int64                   `json:"id"`
				Kind     string                  `json:"kind"`
				History  []seedCreditHistoryLink `json:"history"`
				Children []struct {
					ChildID    int64 `json:"child_id"`
					TotalCents int64 `json:"total_cents"`
				} `json:"children"`
			} `json:"documents"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, 0, fmt.Errorf("decode the released holiday run: %w", err)
	}
	for _, document := range envelope.Data.Documents {
		if slices.Contains(document.History, seedCreditHistoryLink{Kind: "cancellation"}) {
			continue
		}
		for _, child := range document.Children {
			if document.Kind == "invoice" && child.TotalCents >= seedCreditCents {
				return document.ID, child.ChildID, nil
			}
		}
	}
	return 0, 0, nil
}
