package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// seedTraegerBillingReissueStep cancels the corrected invoice of the second
// family of the released holiday run and starts the next run of the period
// (#5663). That run stays a preview: it replaces the cancelled invoice and
// proposes its corrections. The Abrechnung removes one of them, so the page
// shows the proposal it keeps. It runs before the credit step, which credits
// the first family.
type seedTraegerBillingReissueStep struct{}

func (seedTraegerBillingReissueStep) Name() string { return "Seeding a reissued invoice" }

// seedReissueReason is the invented reason of the Storno.
const seedReissueReason = "Anschrift war falsch. Neue Rechnung folgt."

// seedReissueRun is what the step reads of a run.
type seedReissueRun struct {
	HolidayPeriodID int64 `json:"holiday_period_id"`
	Documents       []struct {
		ID              int64  `json:"id"`
		Kind            string `json:"kind"`
		ReferenceNumber string `json:"reference_number"`
		Children        []struct {
			Positions []struct {
				Kind string `json:"kind"`
			} `json:"positions"`
			CarriedCorrections []struct {
				PositionID int64 `json:"position_id"`
			} `json:"carried_corrections"`
		} `json:"children"`
	} `json:"documents"`
}

func (seedTraegerBillingReissueStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the reissue seed prerequisites are not available")
	}
	if skipped, _ := rt.Values[seedHolidayRegistrationsSkippedKey].(bool); skipped {
		fmt.Println("  Abrechnung: no holiday run without registrations; no reissued invoice")
		return nil
	}
	runID, ok := rt.Values[seedBillingRunKey].(int64)
	if !ok || runID <= 0 {
		return fmt.Errorf("the reissued invoice needs the released holiday run")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	run, err := seedReadReissueRun(rt.Client.GetWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d", runID)))
	if err != nil {
		return fmt.Errorf("read the released holiday run: %w", err)
	}
	invoiceID := seedReissueTarget(run)
	if invoiceID == 0 {
		fmt.Println("  Abrechnung: no second family with corrections; no reissued invoice")
		return nil
	}
	if _, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d/documents/%d/cancellation", runID, invoiceID),
		map[string]any{"reason": seedReissueReason}); err != nil {
		return fmt.Errorf("cancel the invoice: %w", err)
	}
	raw, err := rt.Client.PostWithAuth(auth, "/traeger/billing-runs/", map[string]any{"holiday_period_id": run.HolidayPeriodID})
	if err != nil {
		return fmt.Errorf("start the next billing run: %w", err)
	}
	nextID, err := decodeSeedEntityID(raw)
	if err != nil {
		return fmt.Errorf("read the next billing run: %w", err)
	}
	next, err := seedReadReissueRun(rt.Client.GetWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d", nextID)))
	if err != nil {
		return fmt.Errorf("read the next billing run: %w", err)
	}
	positionID := seedProposedCorrection(next)
	if positionID == 0 {
		return fmt.Errorf("the next billing run proposes no correction of the cancelled invoice")
	}
	if _, err := rt.Client.DeleteWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d/carried-corrections/%d", nextID, positionID)); err != nil {
		return fmt.Errorf("remove a proposed correction: %w", err)
	}
	fmt.Println("  Abrechnung: one invoice cancelled, the next run proposes its corrections")
	return nil
}

func seedReadReissueRun(raw []byte, err error) (seedReissueRun, error) {
	if err != nil {
		return seedReissueRun{}, err
	}
	var envelope struct {
		Data seedReissueRun `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return seedReissueRun{}, err
	}
	return envelope.Data, nil
}

// seedReissueTarget finds the invoice after the first with corrections; zero
// when there is none. The first family keeps its invoice for the credit step.
func seedReissueTarget(run seedReissueRun) int64 {
	invoices := 0
	for _, document := range run.Documents {
		if document.Kind != "invoice" {
			continue
		}
		invoices++
		if invoices == 1 {
			continue
		}
		for _, child := range document.Children {
			for _, position := range child.Positions {
				if position.Kind == "correction" {
					return document.ID
				}
			}
		}
	}
	return 0
}

// seedProposedCorrection finds the first proposal of the draft that replaces
// the cancelled invoice; zero when there is none.
func seedProposedCorrection(run seedReissueRun) int64 {
	for _, document := range run.Documents {
		for _, child := range document.Children {
			for _, carried := range child.CarriedCorrections {
				return carried.PositionID
			}
		}
	}
	return 0
}
