package seedapi

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Jahresbescheid of the demo Träger (#6219): the template of the fee
// notice gets its legal remedy, and a run of the current year assesses the
// months after the current one, which the monthly seed leaves open, and
// issues the notices. A month already billed by a monthly invoice stays out.

// seedTraegerNoticeTemplate is the template of the fee notice with the legal
// remedy the Träger writes itself (ADR 0170).
var seedTraegerNoticeTemplate = map[string]any{
	"subject":    "Gebührenbescheid {{bescheidnummer}}",
	"salutation": "Sehr geehrte Familie {{familienname}},",
	"body":       "für {{kinder}} setzen wir für {{zeitraum}} fest:\n{{positionen}}\n\n**{{zahlungshinweis}}**",
	"closing":    "Mit freundlichen Grüßen\n{{traeger}}",
	"legal_remedy": "Gegen diesen Bescheid können Sie innerhalb eines Monats nach Bekanntgabe Widerspruch einlegen. " +
		"Der Widerspruch ist schriftlich oder zur Niederschrift bei {{traeger}} einzulegen.",
}

// The registered codes of a notice run another seed of the same Träger
// started a moment earlier, and of a run with nothing to issue.
const (
	seedCareNoticeRunOpen = "parent_billing.care_notice_run_open"
	seedBillingRunEmpty   = "parent_billing.billing_run_empty"
)

type seedCareNoticeRun struct {
	ID           int64  `json:"id"`
	Year         int    `json:"year"`
	Status       string `json:"status"`
	PreviewCount int    `json:"preview_count"`
	PreviewCents int64  `json:"preview_cents"`
	Fingerprint  string `json:"fingerprint"`
}

// seedCareNotices issues the Jahresbescheide of the months after current up
// to December; in December there is no month left and nothing is issued.
func seedCareNotices(client *Client, auth AuthRef, current calendar.Date) (int, error) {
	from := monthAfter(current)
	if from.Year() != current.Year() {
		return 0, nil
	}
	if err := seedNoticeTemplateOnce(client, auth); err != nil {
		return 0, err
	}
	runID, err := seedStartNoticeRun(client, auth, from)
	if err != nil {
		return 0, err
	}
	raw, err := client.GetWithAuth(auth, fmt.Sprintf("/traeger/care-notice-runs/%d", runID))
	if err != nil {
		return 0, fmt.Errorf("check the notice run: %w", err)
	}
	var envelope struct {
		Data seedCareNoticeRun `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("check the notice run: %w", err)
	}
	run := envelope.Data
	if run.Status != "calculated" || run.PreviewCount == 0 {
		return 0, nil
	}
	_, err = client.PostWithAuth(auth, fmt.Sprintf("/traeger/care-notice-runs/%d/release", runID), map[string]any{
		"document_count": run.PreviewCount, "total_cents": run.PreviewCents, "fingerprint": run.Fingerprint,
	})
	if seedConflictCode(err, seedCareRunReleased) || seedConflictCode(err, seedBillingRunEmpty) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("release the notice run: %w", err)
	}
	return run.PreviewCount, nil
}

func monthAfter(month calendar.Date) calendar.Date {
	if month.Month() == 12 {
		return calendar.NewDate(month.Year()+1, 1, 1)
	}
	return calendar.NewDate(month.Year(), month.Month()+1, 1)
}

// seedNoticeTemplateOnce saves the template of the fee notice while the
// Träger still writes with the standard one.
func seedNoticeTemplateOnce(client *Client, auth AuthRef) error {
	raw, err := client.GetWithAuth(auth, "/traeger/master-data/document-templates/bescheid")
	if err != nil {
		return fmt.Errorf("read the Träger template of the fee notice: %w", err)
	}
	var envelope struct {
		Data struct {
			Current struct {
				Standard bool `json:"standard"`
			} `json:"current"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("read the Träger template of the fee notice: %w", err)
	}
	if !envelope.Data.Current.Standard {
		return nil
	}
	_, err = client.PostWithAuth(auth, "/traeger/master-data/document-templates/bescheid/versions", seedTraegerNoticeTemplate)
	var refused *APIError
	if errors.As(err, &refused) && refused.Code == seedTraegerTemplateUnchanged {
		return nil
	}
	if err != nil {
		return fmt.Errorf("save the Träger template of the fee notice: %w", err)
	}
	return nil
}

// seedStartNoticeRun takes the open run of the year or starts one.
func seedStartNoticeRun(client *Client, auth AuthRef, from calendar.Date) (int64, error) {
	raw, err := client.PostWithAuth(auth, "/traeger/care-notice-runs/", map[string]any{
		"year": from.Year(), "from_month": from.String(),
	})
	if seedConflictCode(err, seedCareNoticeRunOpen) {
		return seedOpenNoticeRun(client, auth, from.Year())
	}
	if err != nil {
		return 0, fmt.Errorf("start the notice run of %d: %w", from.Year(), err)
	}
	runID, err := decodeSeedEntityID(raw)
	if err != nil {
		return 0, fmt.Errorf("read the notice run: %w", err)
	}
	return runID, nil
}

func seedOpenNoticeRun(client *Client, auth AuthRef, year int) (int64, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/care-notice-runs/")
	if err != nil {
		return 0, fmt.Errorf("list the notice runs: %w", err)
	}
	var envelope struct {
		Data struct {
			Runs []seedCareNoticeRun `json:"runs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("list the notice runs: %w", err)
	}
	for _, run := range envelope.Data.Runs {
		if run.Year == year && run.Status == "calculated" {
			return run.ID, nil
		}
	}
	return 0, fmt.Errorf("the notice run of %d is neither open nor startable", year)
}
