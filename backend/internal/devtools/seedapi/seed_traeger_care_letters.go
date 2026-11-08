package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedTraegerCareLettersStep gives every released monthly run of the demo
// Träger its letters (#4684, ADR 0165) through the Träger routes, the way the
// Abrechnung works: it creates the letters, cancels the last invoice before
// it went out, so the run shows a Storno that reaches no family, and sends
// the rest. A run that has its letters stays as it is, so a second seed run
// sends nothing twice.
type seedTraegerCareLettersStep struct{}

func (seedTraegerCareLettersStep) Name() string { return "Seeding the letters of the monthly runs" }

type seedCareLetters struct {
	Letters   *struct{} `json:"letters"`
	Documents []struct {
		InvoiceID int64  `json:"invoice_id"`
		Kind      string `json:"kind"`
	} `json:"documents"`
}

func (seedTraegerCareLettersStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" {
		return fmt.Errorf("the monthly letters seed prerequisites are not available")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	raw, err := rt.Client.GetWithAuth(auth, "/traeger/care-billing-runs/")
	if err != nil {
		return fmt.Errorf("read the monthly runs: %w", err)
	}
	var list struct {
		Data struct {
			Runs []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"runs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return fmt.Errorf("read the monthly runs: %w", err)
	}
	sent := 0
	for _, run := range list.Data.Runs {
		if run.Status != "released" {
			continue
		}
		queued, err := seedCareRunLetters(rt.Client, auth, run.ID)
		if err != nil {
			return err
		}
		sent += queued
	}
	fmt.Printf("  Träger monthly letters: %d sent\n", sent)
	return nil
}

// seedCareRunLetters creates, cancels and sends the letters of one released
// run that has none yet and returns the number of mails it queued.
func seedCareRunLetters(client *Client, auth AuthRef, runID int64) (int, error) {
	raw, err := client.GetWithAuth(auth, fmt.Sprintf("/traeger/care-billing-runs/%d/letters", runID))
	if err != nil {
		return 0, fmt.Errorf("read the letters of the monthly run: %w", err)
	}
	var current struct {
		Data seedCareLetters `json:"data"`
	}
	if err := json.Unmarshal(raw, &current); err != nil {
		return 0, fmt.Errorf("read the letters of the monthly run: %w", err)
	}
	if current.Data.Letters != nil || len(current.Data.Documents) == 0 {
		return 0, nil
	}
	if _, err := client.PostWithAuth(auth, fmt.Sprintf("/traeger/care-billing-runs/%d/letters", runID), map[string]any{
		"payment_due_on": calendar.TodayDate().AddDays(14).String(),
		"notes":          "Bitte überweisen Sie den Beitrag pünktlich. Bei Fragen helfen wir Ihnen gern.",
	}); err != nil {
		return 0, fmt.Errorf("create the letters of the monthly run: %w", err)
	}
	if len(current.Data.Documents) > 2 {
		last := current.Data.Documents[len(current.Data.Documents)-1]
		if _, err := client.PostWithAuth(auth, fmt.Sprintf("/traeger/care-billing-runs/%d/invoices/%d/cancellation", runID, last.InvoiceID),
			map[string]any{"reason": "Der Vertrag wurde vor dem Versand geändert."}); err != nil {
			return 0, fmt.Errorf("cancel a monthly invoice: %w", err)
		}
	}
	raw, err = client.PostWithAuth(auth, fmt.Sprintf("/traeger/care-billing-runs/%d/letters/mail", runID), nil)
	if err != nil {
		return 0, fmt.Errorf("send the letters of the monthly run: %w", err)
	}
	var series struct {
		Data struct {
			Queued int `json:"queued"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &series); err != nil {
		return 0, fmt.Errorf("read the sent letters: %w", err)
	}
	return series.Data.Queued, nil
}
