package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
// the numbered invoice of the confirmed siblings. It downloads the overview
// of the run for the Kämmerei (#4672) and returns the run.
func seedHolidayBillingRun(client *Client, auth AuthRef, periodID int64) (int64, error) {
	if _, err := client.PutWithAuth(auth, "/traeger/number-ranges/holiday", map[string]any{"pattern": seedBillingNumberPattern}); err != nil {
		return 0, fmt.Errorf("keep the number range: %w", err)
	}
	raw, err := client.PostWithAuth(auth, "/traeger/billing-runs/", map[string]any{"holiday_period_id": periodID})
	if err != nil {
		return 0, fmt.Errorf("start the billing run: %w", err)
	}
	runID, err := decodeSeedEntityID(raw)
	if err != nil {
		return 0, fmt.Errorf("read the billing run: %w", err)
	}
	run, err := seedReadBillingRun(client.GetWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d", runID)))
	if err != nil {
		return 0, fmt.Errorf("check the billing run: %w", err)
	}
	if len(run.Documents) == 0 || len(run.Documents[0].Children) == 0 {
		return 0, fmt.Errorf("the billing run has no invoice to check")
	}
	family, child := run.Documents[0], run.Documents[0].Children[0]
	run, err = seedReadBillingRun(client.PostWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d/corrections", runID), map[string]any{
		"registration_id": family.RegistrationID, "child_id": child.ChildID, "total_cents": max(child.TotalCents-1000, 0),
		"reason": "Am ersten Ferientag krank, Tag wird erstattet.",
	}))
	if err != nil {
		return 0, fmt.Errorf("correct the billing run: %w", err)
	}
	if _, err := client.PostWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d/release", runID), map[string]any{
		"document_count": run.PreviewCount, "total_cents": run.PreviewCents,
	}); err != nil {
		return 0, fmt.Errorf("release the billing run: %w", err)
	}
	// The overview for the Kämmerei (#4672) is downloaded once, so the trail
	// of the run shows a download.
	overview, err := client.GetWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d/export?format=csv", runID))
	if err != nil {
		return 0, fmt.Errorf("download the overview of the billing run: %w", err)
	}
	if len(overview) == 0 {
		return 0, fmt.Errorf("the overview of the billing run is empty")
	}
	fmt.Printf("  Abrechnung: holiday run released with %d invoice(s) and one correction, overview downloaded\n", run.PreviewCount)
	return runID, nil
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

// seedBillingRunKey keeps the run the holiday period step released, so the
// letters step can confirm its families once the Träger has its letterhead.
const seedBillingRunKey = "traeger.billing_run"

// seedTraegerBillingLettersStep creates the confirmation letters of the
// released holiday run (#4669): the payment date, the packing list and rule
// on illness, and the emergency numbers of the hosting OGS. It runs after
// the master data step, because a letter needs the bank account of the
// Träger.
type seedTraegerBillingLettersStep struct{}

func (seedTraegerBillingLettersStep) Name() string { return "Seeding the confirmation letters" }

// The text of the demo letters. Every value is invented.
var (
	seedLetterNotes = "Bitte denken Sie an: Frühstück, Getränk, Wechselkleidung, Hausschuhe und Kopfbedeckung. " +
		"Bitte schreiben Sie den Namen Ihres Kindes auf alle Sachen.\n" +
		"Kinder mit ansteckenden Krankheiten dürfen nicht kommen."
	seedLetterContacts = []map[string]any{{"name": "Betreuungsteam der Ferien-OGS", "phone": "+49 5402 1234-10"}}
)

func (seedTraegerBillingLettersStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the confirmation letter seed prerequisites are not available")
	}
	if skipped, _ := rt.Values[seedHolidayRegistrationsSkippedKey].(bool); skipped {
		fmt.Println("  Abrechnung: no holiday run without registrations; no confirmation letters")
		return nil
	}
	runID, ok := rt.Values[seedBillingRunKey].(int64)
	if !ok || runID <= 0 {
		return fmt.Errorf("the confirmation letters need the released holiday run")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	if _, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/traeger/billing-runs/%d/letters", runID), map[string]any{
		"payment_due_on": calendar.TodayDate().AddDays(14).String(), "notes": seedLetterNotes, "emergency_contacts": seedLetterContacts,
	}); err != nil {
		return fmt.Errorf("create the confirmation letters: %w", err)
	}
	fmt.Println("  Abrechnung: confirmation letters of the holiday run created")
	return nil
}
