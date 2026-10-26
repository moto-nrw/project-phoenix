package api

import (
	"encoding/json"
	"fmt"
)

// seedBillingNumberPattern is the number range of the holiday invoices the
// Abrechnung keeps (#4668): the year and a running number of four digits.
const seedBillingNumberPattern = "FE-{JJJJ}-{NNNN}"

// seedBillingRunDetail is what the seed reads of a run: the sums of the
// preview and the first child of the first invoice.
type seedBillingRunDetail struct {
	PreviewCount int   `json:"preview_count"`
	PreviewCents int64 `json:"preview_cents"`
	Documents    []struct {
		RegistrationID int64 `json:"registration_id"`
		Children       []struct {
			ChildID    int64 `json:"child_id"`
			TotalCents int64 `json:"total_cents"`
		} `json:"children"`
	} `json:"documents"`
}

// seedHolidayBillingRun bills the holiday period in advance (#4668): the
// Abrechnung keeps its number range, starts a run for the period, lowers
// the amount of one child with a reason and releases the run, which issues
// the numbered invoice of the confirmed siblings.
func seedHolidayBillingRun(client *Client, auth AuthRef, periodID int64) error {
	if _, err := client.PutWithAuth(auth, "/traeger/number-ranges/holiday", map[string]any{"pattern": seedBillingNumberPattern}); err != nil {
		return fmt.Errorf("keep the number range: %w", err)
	}
	raw, err := client.PostWithAuth(auth, "/traeger/billing-runs/", map[string]any{"holiday_period_id": periodID})
	if err != nil {
		return fmt.Errorf("start the billing run: %w", err)
	}
	runID, err := decodeSeedEntityID(raw)
	if err != nil {
		return fmt.Errorf("read the billing run: %w", err)
	}
	run, err := seedReadBillingRun(client.GetWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d", runID)))
	if err != nil {
		return fmt.Errorf("check the billing run: %w", err)
	}
	if len(run.Documents) == 0 || len(run.Documents[0].Children) == 0 {
		return fmt.Errorf("the billing run has no invoice to check")
	}
	family, child := run.Documents[0], run.Documents[0].Children[0]
	run, err = seedReadBillingRun(client.PostWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d/corrections", runID), map[string]any{
		"registration_id": family.RegistrationID, "child_id": child.ChildID, "total_cents": max(child.TotalCents-1000, 0),
		"reason": "Am ersten Ferientag krank, Tag wird erstattet.",
	}))
	if err != nil {
		return fmt.Errorf("correct the billing run: %w", err)
	}
	if _, err := client.PostWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d/release", runID), map[string]any{
		"document_count": run.PreviewCount, "total_cents": run.PreviewCents,
	}); err != nil {
		return fmt.Errorf("release the billing run: %w", err)
	}
	fmt.Printf("  Abrechnung: holiday run released with %d invoice(s) and one correction\n", run.PreviewCount)
	return nil
}

func seedReadBillingRun(raw []byte, err error) (seedBillingRunDetail, error) {
	if err != nil {
		return seedBillingRunDetail{}, err
	}
	var envelope struct {
		Data seedBillingRunDetail `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return seedBillingRunDetail{}, err
	}
	return envelope.Data, nil
}
